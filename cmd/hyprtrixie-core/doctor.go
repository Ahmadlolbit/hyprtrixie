package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/btrfs"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/deployed"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
)

const (
	secondsADay = 86400
	warnedFull  = 85
	failedFull  = 95
	wornBattery = 70
	maxShown    = 3
	noMatch     = 1
	inactive    = 3
	unknownUnit = 4
)

var desktopPrograms = [][2]string{
	{"bar", "qs -p /usr/share/hyprtrixie/shell"},
	{"hyprpaper", "^hyprpaper"},
	{"mako", "^mako"},
	{"elephant", "^elephant"},
	{"walker", "walker --gapplication-service"},
	{"hypridle", "^hypridle"},
}

var sleepUnits = []string{
	"systemd-suspend.service",
	"systemd-hibernate.service",
	"systemd-hybrid-sleep.service",
	"systemd-suspend-then-hibernate.service",
	"systemd-poweroff.service",
	"systemd-reboot.service",
	"systemd-halt.service",
	"systemd-kexec.service",
	"systemd-soft-reboot.service",
}

type fix struct {
	question string
	apply    func() bool
}

type checkup struct {
	person account.Account
	limit  time.Duration
	fixes  []fix
}

func tooLong(check string) {
	ui.Skip(check + " took longer than a minute")
}

func couldNot(what, program string, err error) {
	reason := program + " failed"
	if errors.Is(err, exec.ErrNotFound) {
		reason = program + " isn't installed"
	}
	ui.Warn("Couldn't check " + what + ": " + reason)
}

func unchecked(what, doing, program string, err error) bool {
	switch {
	case err == nil:
		return false
	case run.TimedOut(err):
		tooLong(doing)
	default:
		couldNot(what, program, err)
	}
	return true
}

func (visit *checkup) offer(question string, apply func() bool) {
	visit.fixes = append(visit.fixes, fix{question, apply})
}

func (visit *checkup) running(args ...string) (bool, error) {
	_, err := run.Capture(visit.limit, "pgrep", append([]string{"-u", visit.person.UID}, args...)...)
	if err != nil && !run.TimedOut(err) && run.Code(err) == noMatch {
		return false, nil
	}
	return err == nil, err
}

func succeeded(name string, args ...string) bool {
	return run.Within(run.Slow, name, args...) == 0
}

func (visit *checkup) restart(name string) bool {
	instance := hyprInstance(visit.person)
	if instance == "" {
		ui.Print("Hyprland is not running\n")
		return false
	}
	return succeeded("runuser", hyprCommand(visit.person, instance, "hyprctl", "dispatch", "hl.dsp.exec_cmd([[hyprtrixie-start "+name+"]])")...)
}

func (visit *checkup) missingPrograms() ([]string, error) {
	var missing []string
	for _, program := range desktopPrograms {
		found, err := visit.running("-f", program[1])
		if err != nil {
			return nil, err
		}
		if !found {
			missing = append(missing, program[0])
		}
	}
	return missing, nil
}

func (visit *checkup) desktop() {
	up, err := visit.running("-x", "Hyprland")
	if err == nil && !up {
		ui.Skip("Desktop programs: Hyprland isn't running for " + visit.person.Name)
		return
	}
	var missing []string
	if err == nil {
		missing, err = visit.missingPrograms()
	}
	if unchecked("the desktop programs", "Checking the desktop programs", "pgrep", err) {
		return
	}
	if len(missing) == 0 {
		ui.OK("The bar, wallpaper, notifications, launcher and idle timer are running")
		return
	}
	ui.Fail("Not running: " + strings.Join(missing, " "))
	ui.Note("Fix: log out and back in, or let the doctor start them again")
	for _, name := range missing {
		visit.offer("Start "+name+" again", func() bool { return visit.restart(name) })
	}
}

func firstWords(output string) []string {
	var words []string
	for _, line := range run.Lines(output) {
		if fields := strings.Fields(line); len(fields) > 0 {
			words = append(words, fields[0])
		}
	}
	return words
}

func (visit *checkup) userSystemctl(args ...string) []string {
	return visit.person.RunuserArgs(append([]string{"env", "XDG_RUNTIME_DIR=" + visit.person.Runtime(), "systemctl", "--user"}, args...)...)
}

func (visit *checkup) failedUnits() ([]string, []string, bool, bool) {
	output, err := run.CaptureQuiet(visit.limit, "systemctl", "--failed", "--no-legend", "--plain")
	system := firstWords(output)
	output, userErr := run.CaptureQuiet(visit.limit, "runuser", visit.userSystemctl("--failed", "--no-legend", "--plain")...)
	user := firstWords(output)
	if run.TimedOut(err) || run.TimedOut(userErr) {
		tooLong("Checking the services")
		return nil, nil, false, false
	}
	if unchecked("the services", "Checking the services", "systemctl", err) {
		return nil, nil, false, false
	}
	return system, user, !unchecked("your own services", "Checking the services", "systemctl --user", userErr), true
}

func (visit *checkup) services() {
	system, user, ownChecked, ok := visit.failedUnits()
	if !ok {
		return
	}
	if len(system)+len(user) == 0 && ownChecked {
		ui.OK("No service has failed")
		return
	}
	if len(system)+len(user) == 0 {
		ui.OK("No system service has failed")
		return
	}
	names := slices.Clone(system)
	for _, unit := range user {
		names = append(names, "user:"+unit)
	}
	ui.Fail("Failed services: " + strings.Join(names, " "))
	ui.Note("Fix: systemctl status NAME says why; restarting often helps")
	for _, unit := range system {
		if slices.Contains(sleepUnits, unit) {
			ui.Note("Not restarting " + unit + ": it would put the machine to sleep or switch it off at once. journalctl -b -u " + unit + " says why it failed")
			continue
		}
		visit.offer("Restart "+unit, func() bool { return succeeded("systemctl", "restart", unit) })
	}
	for _, unit := range user {
		visit.offer("Restart "+unit, func() bool { return succeeded("runuser", visit.userSystemctl("restart", unit)...) })
	}
}

func (visit *checkup) timers() (bool, error) {
	for _, unit := range []string{"snapper-timeline.timer", "snapper-cleanup.timer"} {
		_, err := run.CaptureQuiet(visit.limit, "systemctl", "is-active", "--quiet", unit)
		if err == nil {
			continue
		}
		if code := run.Code(err); !run.TimedOut(err) && (code == inactive || code == unknownUnit) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func newestSnapshot(limit time.Duration) (int64, bool, error) {
	output, err := run.CaptureQuiet(limit, "snapper", "--csvout", "--no-headers", "-c", "root", "list", "--columns", "date")
	last := output[strings.LastIndexByte(output, '\n')+1:]
	if last == "" || err != nil {
		return 0, false, err
	}
	stamp, err := run.CaptureQuiet(limit, "date", "-d", last, "+%s")
	seconds, problem := strconv.ParseInt(stamp, 10, 64)
	if !run.TimedOut(err) {
		err = nil
	}
	return seconds, problem == nil, err
}

func (visit *checkup) timersOff() {
	if !installed("snapper") {
		ui.Fail("Snapshots aren't set up: snapper isn't installed")
		ui.Note("Fix: sudo hyprtrixie install --only 05 sets them up")
		return
	}
	ui.Fail("The snapshot timers aren't running")
	ui.Note("Fix: systemctl enable --now snapper-timeline.timer snapper-cleanup.timer")
	visit.offer("Turn the snapshot timers back on", func() bool {
		return succeeded("systemctl", "enable", "--now", "snapper-timeline.timer", "snapper-cleanup.timer")
	})
}

func (visit *checkup) snapshots(setting string) {
	if setting != "yes" {
		ui.Skip("Snapshots are turned off")
		return
	}
	if kind := btrfs.RootType(); kind != "" && kind != "btrfs" {
		ui.Skip("Snapshots need a btrfs root; this system uses " + kind)
		return
	}
	active, err := visit.timers()
	if unchecked("the snapshot timers", "Checking the snapshots", "systemctl", err) {
		return
	}
	if !active {
		visit.timersOff()
		return
	}
	last, ok, err := newestSnapshot(visit.limit)
	if unchecked("the snapshots", "Checking the snapshots", "snapper", err) {
		return
	}
	if !ok {
		ui.Warn("No snapshot of the system yet")
		return
	}
	if age := (time.Now().Unix() - last) / secondsADay; age > 2 {
		ui.Warn(fmt.Sprintf("The newest snapshot is %d days old", age))
		ui.Note("Fix: journalctl -u snapper-timeline says why snapshots stopped")
		return
	}
	ui.OK("Snapshots are being taken")
}

func (visit *checkup) freeSpace() bool {
	before, _ := filesystems(visit.limit)
	run.Within(run.Slow, "apt-get", "clean")
	if installed("snapper") {
		run.Within(run.Slow, "snapper", "-c", "root", "cleanup", "number")
	}
	run.Within(run.Slow, "journalctl", "--vacuum-time=2weeks")
	after, err := filesystems(visit.limit)
	if err != nil || len(after) == 0 {
		ui.Note("Couldn't check the disks again; df -h shows how full they are")
		return false
	}
	return roomNow(before, after)
}

func roomNow(before, after []fsutil.Filesystem) bool {
	room := true
	for _, now := range after {
		if !now.Known {
			continue
		}
		was := now.Use
		for _, old := range before {
			if old.Device == now.Device && old.Known {
				was = old.Use
			}
		}
		ui.Note(fmt.Sprintf("%s was %d%% full, now %d%%", now.Mount, was, now.Use))
		room = room && now.Use < warnedFull
	}
	if !room {
		ui.Note("Still full: the space is in your files or the snapshots. du -sh ~/* lists your biggest folders, and sudo hyprtrixie cleanup removes older snapshots")
	}
	return room
}

func filesystems(limit time.Duration) ([]fsutil.Filesystem, error) {
	output, err := run.CaptureQuiet(limit, "df", "-P", "/", "/home")
	return fsutil.Filesystems(output), err
}

func (visit *checkup) disk() {
	found, err := filesystems(visit.limit)
	if run.TimedOut(err) {
		tooLong("Checking the disks")
		return
	}
	if len(found) == 0 {
		couldNot("the disks", "df", err)
		return
	}
	offered := false
	for _, one := range found {
		text := fmt.Sprintf("%s is %d%% full", one.Mount, one.Use)
		switch {
		case one.Use < warnedFull:
			ui.OK(text)
			continue
		case one.Use < failedFull:
			ui.Warn(text)
		default:
			ui.Fail(text)
		}
		ui.Note("Fix: clear the package cache, trim old snapshots and old logs")
		if !offered {
			offered = true
			visit.offer("Free space: clear the package cache, trim old snapshots and old logs", visit.freeSpace)
		}
	}
}

func batteryValue(battery, energy, charge string) string {
	if text, err := fsutil.Text(filepath.Join(battery, energy)); err == nil {
		return text
	}
	text, _ := fsutil.Text(filepath.Join(battery, charge))
	return text
}

func batteryHealth(battery string) (uint64, bool) {
	full, err := strconv.ParseUint(batteryValue(battery, "energy_full", "charge_full"), 10, 64)
	if err != nil {
		return 0, false
	}
	text := batteryValue(battery, "energy_full_design", "charge_full_design")
	design, err := strconv.ParseUint(text, 10, 64)
	if err != nil || text[0] == '0' {
		return 0, false
	}
	return full * 100 / design, true
}

func checkBatteries() {
	found := false
	batteries, _ := filepath.Glob(env.Or("HYPRTRIXIE_POWER_SUPPLY", "/sys/class/power_supply") + "/BAT*")
	for _, battery := range batteries {
		health, ok := batteryHealth(battery)
		if !ok {
			continue
		}
		found = true
		text := fmt.Sprintf("Battery %s holds %d%% of what it did when new", filepath.Base(battery), health)
		if health < wornBattery {
			ui.Warn(text)
			ui.Note("Fix: none needed yet; a replacement brings the runtime back")
		} else {
			ui.OK(text)
		}
	}
	if !found {
		ui.Skip("No battery found")
	}
}

func (visit *checkup) firmware() {
	output, err := run.CaptureQuiet(visit.limit, "journalctl", "-k", "-b", "-p", "3", "--no-pager", "-q", "-o", "cat")
	if unchecked("the kernel log", "Reading the kernel log", "journalctl", err) {
		return
	}
	lines := slices.Compact(run.Sorted(run.Lines(output)))
	var firmware []string
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "firmware") {
			firmware = append(firmware, line)
		}
	}
	if len(firmware) == 0 {
		ui.OK(fmt.Sprintf("No firmware errors since boot (%d other kernel errors)", len(lines)))
		return
	}
	ui.Warn(fmt.Sprintf("Firmware or driver errors since boot: %d", len(firmware)))
	for _, line := range firmware[:min(maxShown, len(firmware))] {
		ui.Note(line)
	}
	ui.Note("Fix: sudo hyprtrixie collect --verbose and send the file it prints")
}

func (visit *checkup) updates() {
	output, err := run.CaptureInEnglish(visit.limit, "apt", "list", "--upgradable")
	if unchecked("for updates", "Checking for updates", "apt", err) {
		return
	}
	count := 0
	for _, line := range run.Lines(output) {
		if strings.Contains(line, "upgradable") {
			count++
		}
	}
	if count == 0 {
		ui.OK("Packages are up to date")
		return
	}
	ui.Warn(fmt.Sprintf("%d package updates are waiting", count))
	ui.Note("Fix: Menu › Update › Debian packages, or sudo apt full-upgrade")
}

func firstFew(list []string) string {
	return strings.Join(list[:min(maxShown, len(list))], " ")
}

func (visit *checkup) files() {
	changes, err := deployed.Changes(visit.limit, os.Stderr)
	var missing []string
	edited := 0
	for _, change := range changes {
		if change.Missing {
			missing = append(missing, change.Path)
		} else {
			edited++
		}
	}
	switch {
	case len(missing) > 0:
		ui.Fail("Installed files are missing: " + firstFew(missing))
		ui.Note("Fix: sudo hyprtrixie install puts them back")
	case run.TimedOut(err):
		tooLong("Comparing the installed files")
	case err != nil:
		couldNot("the installed files", "sha256sum", err)
	default:
		ui.OK(fmt.Sprintf("Every installed file is in place (%d edited by you)", edited))
	}
	waiting, err := deployed.Waiting(visit.limit, visit.person.Home)
	if run.TimedOut(err) {
		tooLong("Looking for newer versions")
	}
	if len(waiting) > 0 {
		ui.Warn("Newer versions wait next to files you edited: " + firstFew(waiting))
		ui.Note("Fix: compare each with its .hyprtrixie-new copy and keep what you want")
	}
}

func checkInstaller() {
	commit, last := deployed.Commit(root()), deployed.LastCommit()
	switch {
	case last == "" || commit == "":
		ui.Skip("Can't tell which version the installer last ran from")
	case deployed.Matches(commit, last):
		ui.OK("The installer last ran from this folder's version (" + commit + ")")
	default:
		ui.Warn("This folder is at " + commit + " but the installer last ran from " + last)
		ui.Note("Fix: sudo hyprtrixie install")
	}
}

func (visit *checkup) offerFixes() {
	if len(visit.fixes) == 0 {
		ui.Print("\nNothing for the doctor to fix.\n")
		return
	}
	if !ui.CanAsk() {
		ui.Print("\nRun it in a terminal to apply the fixes it offers.\n")
		return
	}
	ui.Print("\n")
	for _, offered := range visit.fixes {
		if !ui.Confirm(offered.question + "?") {
			continue
		}
		if offered.apply() {
			ui.OK(offered.question)
		} else {
			ui.Fail(offered.question)
		}
	}
}

func examine() int {
	person, ok := account.Target()
	if !ok {
		return fail("cannot tell whose desktop this is; run it with sudo from your own account")
	}
	setting := config.Read(config.Path()).Get("SNAPSHOTS")
	visit := checkup{person: person, limit: run.Minute}
	ui.Header("hyprtrixie doctor", "Checks the system and offers the safe fixes")
	ui.Rule("Desktop")
	visit.desktop()
	visit.services()
	ui.Rule("Storage")
	visit.snapshots(setting)
	visit.disk()
	ui.Rule("Hardware and packages")
	checkBatteries()
	visit.firmware()
	visit.updates()
	ui.Rule("Hyprtrixie")
	visit.files()
	checkInstaller()
	visit.offerFixes()
	return ui.ExitCode()
}

func doctor() int {
	return asRoot("doctor", examine)
}
