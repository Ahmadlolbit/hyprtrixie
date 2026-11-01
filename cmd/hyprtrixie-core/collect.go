package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/deployed"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/modules"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const kernelLines = 100

var hyprRequests = [][]string{{"version"}, {"monitors"}, {"workspaces"}, {"clients"}, {"layers"}, {"configerrors"}, {"hyprpaper", "listactive"}}

type report struct {
	file    *os.File
	person  account.Account
	limit   time.Duration
	problem error
}

type filedReport struct {
	name string
	fill func(*report)
}

func (out *report) write(format string, args ...any) {
	if _, err := fmt.Fprintf(out.file, format, args...); err != nil && out.problem == nil {
		out.problem = err
	}
}

func (out *report) section(title string) {
	out.write("\n==== %s\n\n", title)
}

func (out *report) command(name string, args ...string) {
	run.Record(out.limit, out.file, name, args...)
}

func (out *report) noteDelay(name string, err error) {
	if run.TimedOut(err) {
		out.write("%s took longer than %s\n", name, out.limit)
	}
}

func (out *report) quiet(name string, args ...string) (string, error) {
	output, err := run.CaptureQuiet(out.limit, name, args...)
	out.noteDelay(name, err)
	return output, err
}

func (out *report) answer(fallback, name string, args ...string) string {
	output, err := out.quiet(name, args...)
	if err != nil {
		return strings.TrimPrefix(output+"\n"+fallback, "\n")
	}
	return output
}

func (out *report) asUser(name string, args ...string) {
	out.command("runuser", out.person.RunuserArgs(append([]string{name}, args...)...)...)
}

func (out *report) asHypr(args ...string) {
	instance := hyprInstance(out.person)
	if instance == "" {
		out.write("Hyprland is not running\n")
		return
	}
	out.command("runuser", hyprCommand(out.person, instance, args...)...)
}

func (out *report) tail(lines, path string, show func(string, ...string)) {
	if fsutil.IsFile(path) {
		show("tail", "-n", lines, path)
	} else {
		out.write("(no %s)\n", path)
	}
}

func newest(pattern string, wanted func(string) bool) string {
	found, _ := filepath.Glob(pattern)
	latest, when := "", time.Time{}
	for _, path := range found {
		info, err := os.Stat(path)
		if err == nil && wanted(path) && info.ModTime().After(when) {
			latest, when = path, info.ModTime()
		}
	}
	return latest
}

func hyprInstance(person account.Account) string {
	if folder := newest(person.Runtime()+"/hypr/*", fsutil.IsDir); folder != "" {
		return filepath.Base(folder)
	}
	return ""
}

func hyprCommand(person account.Account, instance string, args ...string) []string {
	return person.RunuserArgs(append([]string{"env", "HYPRLAND_INSTANCE_SIGNATURE=" + instance}, args...)...)
}

func installerState(commit, last string) string {
	switch {
	case last == "":
		return "The installer has not run from this folder yet"
	case commit == "":
		return "The installer last ran from " + last + "; this folder is not a git checkout"
	case deployed.Matches(commit, last):
		return "The installer last ran from the current commit (" + commit + ")"
	}
	return "The repo is at " + commit + " but the installer last ran from " + last + ": run sudo ./hyprtrixie install"
}

func (out *report) themeState() string {
	return filepath.Join(out.person.Home, ".local/state/hyprtrixie/current")
}

func (out *report) system() {
	out.write("%s\n", detect.Summary())
	out.command("uname", "-a")
	out.command("cat", "/etc/os-release")
	out.write("\n")
	out.command("lspci", "-nnk")
	out.command("lsusb")
	out.write("\n")
	out.command("ip", "-brief", "address")
	out.command("nmcli", "general", "status")
	out.command("findmnt", "-t", "btrfs")
	out.write("\n")
	out.command("dpkg-query", "-W", "-f", "${Package} ${Version}\n", "hyprland*", "quickshell", "linux-image-*", "firmware-*", "nvidia-*", "mesa-vulkan-drivers")
	out.write("\n")
	out.command("journalctl", "-b", "-p", "warning", "--no-pager", "-n", "300")
}

func (out *report) desktop() {
	state := out.themeState()
	out.asUser("cat", state+"/theme.name")
	out.asUser("ls", "-l", state, state+"/theme")
	out.write("\n")
	out.command("pgrep", "-a", "-u", out.person.Name, "-x", "Hyprland|hyprpaper|hypridle|mako|elephant|walker|qs|quickshell")
	out.write("\n")
	out.asUser("qs", "log", "-p", env.Or("HYPRTRIXIE_SHARE_DIR", "/usr/share/hyprtrixie")+"/shell", "-t", "300")
}

func (out *report) packages() {
	output, _ := out.quiet("dpkg-query", "-W", "-f", "${Package} ${Version}\n",
		"hyprland", "quickshell", "hyprpaper", "hypridle", "hyprlock", "mako-notifier", "sddm", "linux-image-[0-9]*")
	if output != "" {
		out.write("%s\n", output)
	}
	bin := env.Or("HYPRTRIXIE_BIN_DIR", "/usr/local/bin")
	out.write("walker %s\n", out.answer("not found", bin+"/walker", "--version"))
	out.write("elephant %s\n", out.answer("not found", bin+"/elephant", "version"))
}

func (out *report) summary() {
	state := "off"
	if fsutil.IsFile(filepath.Join(out.person.Home, ".config/hyprtrixie/verbose")) {
		state = "on"
	}
	out.section("Summary")
	out.write("Date          %s\n", time.Now().Format("2006-01-02 15:04:05 -0700"))
	out.write("hyprtrixie    %s from %s\n", env.Version, root())
	out.write("Repo          %s\n", out.answer("not a git checkout", "git", "-C", root(), "log", "-1", "--no-show-signature", "--format=%h %s (%D)"))
	out.write("Last install  from %s\n", deployed.LastCommit())
	out.write("Verbose       %s\n\n", state)
	out.write("%s\n", detect.Summary())
	out.packages()
}

func (out *report) files() {
	out.section("Installed files")
	out.write("%s\n", installerState(deployed.Commit(root()), deployed.LastCommit()))
	changes, _ := deployed.Changes(out.limit, out.file)
	for _, change := range changes {
		if change.Missing {
			out.write("missing  %s\n", change.Path)
		} else {
			out.write("edited   %s\n", change.Path)
		}
	}
	waiting, err := deployed.Waiting(out.limit, out.person.Home)
	for _, path := range waiting {
		out.write("%s\n", path)
	}
	out.noteDelay("find", err)
	out.section("Modules")
	out.write("%s", moduleTable())
}

func (out *report) programs() {
	state := out.themeState()
	out.section("Theme")
	out.asUser("cat", state+"/theme.name")
	out.asUser("ls", "-l", state)
	out.section("Desktop programs")
	out.command("pgrep", "-a", "-u", out.person.Name, "-x", "Hyprland|hyprpaper|hypridle|hyprlock|mako|elephant|walker|qs|quickshell")
	for _, request := range hyprRequests {
		out.section("hyprctl " + strings.Join(request, " "))
		out.asHypr(append([]string{"hyprctl"}, request...)...)
	}
}

func (out *report) barLog() {
	out.section("Bar log")
	log := newest(out.person.Runtime()+"/quickshell/by-id/*/log.qslog", fsutil.Exists)
	if log == "" {
		out.write("(no bar log)\n")
		return
	}
	out.write("%s\n", log)
	out.asUser("env", "NO_COLOR=1", "qs", "log", "-t", "3000", log)
}

func (out *report) programLogs() {
	logs, _ := filepath.Glob(filepath.Join(out.person.Home, ".local/state/hyprtrixie/logs/*.log"))
	for _, file := range logs {
		if fsutil.IsFile(file) {
			out.section("Program log " + filepath.Base(file))
			out.asUser("tail", "-n", "400", file)
		}
	}
}

func (out *report) hyprLog() {
	out.section("Hyprland log")
	if instance := hyprInstance(out.person); instance != "" {
		out.tail("600", filepath.Join(out.person.Runtime(), "hypr", instance, "hyprland.log"), out.asUser)
	} else {
		out.write("Hyprland is not running\n")
	}
}

func (out *report) kernelLog() {
	out.section("Kernel errors and warnings")
	output, _ := out.quiet("dmesg", "--level=err,warn")
	lines := run.Lines(output)
	for _, line := range lines[max(len(lines)-kernelLines, 0):] {
		out.write("%s\n", line)
	}
}

func (out *report) logs() {
	out.barLog()
	out.programLogs()
	out.section("Session log")
	out.tail("400", filepath.Join(out.person.Home, ".local/share/sddm/wayland-session.log"), out.asUser)
	out.hyprLog()
	out.section("Your journal (this boot)")
	out.command("journalctl", "-b", "_UID="+out.person.UID, "--no-pager", "-n", "400")
	out.section("System warnings (this boot)")
	out.command("journalctl", "-b", "-p", "warning", "--no-pager", "-n", "200")
	out.kernelLog()
	out.section("Last install run")
	out.tail("80", filepath.Join(env.LogDir(), "latest.log"), out.command)
}

func (out *report) everything() {
	out.summary()
	out.files()
	out.programs()
	out.logs()
}

func writeReport(path string, person account.Account, fill func(*report)) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	out := report{file: file, person: person, limit: run.Minute}
	fill(&out)
	return errors.Join(out.problem, file.Close())
}

func copyAsUser(person account.Account, source, folder string) {
	target, err := os.OpenFile(filepath.Join(folder, filepath.Base(source)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer target.Close()
	if run.Into(run.Slow, target, "runuser", person.RunuserArgs("cat", source)...) != 0 {
		os.Remove(target.Name())
	}
}

func copyLogs(person account.Account, folder string) {
	run.Quiet(run.Slow, "cp", "-r", env.LogDir(), filepath.Join(folder, "log"))
	run.Quiet(run.Slow, "cp", "-r", filepath.Join(person.Home, "runbook-logs"), folder+"/")
	for _, file := range []string{config.Path(), modules.StatusFile()} {
		if fsutil.IsFile(file) {
			run.Within(run.Slow, "cp", file, folder+"/")
		}
	}
	hyprLogs, _ := filepath.Glob(person.Runtime() + "/hypr/*/hyprland.log")
	for _, file := range append([]string{filepath.Join(person.Home, "runbook-report.md")}, hyprLogs...) {
		if fsutil.IsFile(file) {
			copyAsUser(person, file, folder)
		}
	}
}

func saveFor(person account.Account, path, name string, args ...string) bool {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o666)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	defer file.Close()
	defer cleanup.OnSignal(func() { os.Remove(path) })()
	if err := person.Own(file); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Remove(path)
		return false
	}
	if run.Into(run.Slow, file, name, args...) != 0 {
		os.Remove(path)
		return false
	}
	return true
}

func pack(person account.Account, work, stamp string, verbose bool) (string, bool) {
	name := "hyprtrixie-collect-" + stamp
	folder := filepath.Join(work, name)
	if err := os.MkdirAll(folder, 0o777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return folder, false
	}
	logName := "hyprtrixie-log-" + stamp + ".txt"
	reports := []filedReport{{"system.txt", (*report).system}, {"desktop.txt", (*report).desktop}}
	if verbose {
		reports = append(reports, filedReport{logName, (*report).everything})
	}
	for _, one := range reports {
		path := filepath.Join(folder, one.name)
		if err := writeReport(path, person, one.fill); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return path, false
		}
	}
	copyLogs(person, folder)
	saved := ""
	if verbose {
		logfile := filepath.Join(person.Home, logName)
		if !saveFor(person, logfile, "cat", filepath.Join(folder, logName)) {
			return logfile, false
		}
		saved = "Log: " + logfile + "\n"
	}
	archive := filepath.Join(person.Home, name+".tar.gz")
	if !saveFor(person, archive, "tar", "-czf", "-", "-C", work, name) {
		return archive, false
	}
	return "Saved " + archive + "\n" + saved, true
}

func collectAll(verbose bool) int {
	person, ok := account.Target()
	if !ok {
		return fail("cannot tell whose desktop this is; run it with sudo from your own account")
	}
	work, err := os.MkdirTemp("", "hyprtrixie-collect-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fail("cannot create a temporary folder")
	}
	defer cleanup.OnSignal(func() { os.RemoveAll(work) })()
	saved, ok := pack(person, work, time.Now().Format("20060102-150405"), verbose)
	if !ok {
		return fail("cannot write %s; what was collected is in %s", saved, work)
	}
	os.RemoveAll(work)
	return say(saved)
}

func collect() int {
	verbose := env.Arg(2) == "--verbose"
	if env.Arg(2) != "" && !verbose {
		return fail("usage: hyprtrixie collect [--verbose]")
	}
	return asRoot("collect", func() int { return collectAll(verbose) })
}
