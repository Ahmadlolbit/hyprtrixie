package main

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

func installed(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func git(args ...string) string {
	output, _ := run.Capture(run.Slow, "git", append([]string{"-C", root()}, args...)...)
	return output
}

func updateSystem() int {
	if installed("snapper") && fsutil.IsFile(env.Or("HYPRTRIXIE_SNAPPER_CONFIG", "/etc/snapper/configs/root")) {
		fmt.Println("Taking a snapshot first")
		if run.Status("sudo", "snapper", "--no-dbus", "-c", "root", "create", "--description", "before hyprtrixie update", "--cleanup-algorithm", "number") != 0 {
			return fail("the snapshot failed, so nothing was updated")
		}
	}
	fmt.Println("Updating Debian")
	if run.Status("sudo", "apt", "update") != 0 || run.Status("sudo", "apt", "full-upgrade") != 0 {
		return fail("updating Debian failed")
	}
	return 0
}

func updateApps() int {
	for _, tool := range []struct {
		name, what, action string
	}{
		{"flatpak", "Flatpak apps", "update"},
		{"mise", "mise's tools", "upgrade"},
		{"rustup", "Rust", "update"},
	} {
		if !installed(tool.name) {
			continue
		}
		fmt.Printf("Updating %s\n", tool.what)
		if run.Status(tool.name, tool.action) != 0 {
			return fail("updating %s failed", tool.what)
		}
	}
	return 0
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			return line
		}
	}
	return ""
}

func mergeHeading(commit string) string {
	subject := git("log", "-1", "--format=%s", commit)
	heading := firstLine(git("log", "-1", "--format=%b", commit))
	if heading == "" {
		heading = subject
	}
	rest, merged := strings.CutPrefix(subject, "Merge pull request #")
	if number, _, spaced := strings.Cut(rest, " "); merged && spaced {
		return "#" + number + " " + heading
	}
	return heading
}

func changesBetween(from, to string) string {
	var text strings.Builder
	for _, line := range run.Lines(git("log", "--first-parent", "--merges", "--format=%H %P", from+".."+to)) {
		fields := strings.Fields(line)
		text.WriteString(mergeHeading(fields[0]) + "\n")
		if merged := git("log", "--no-merges", "--format=  - %s", fields[1]+".."+fields[2]); merged != "" {
			text.WriteString(merged + "\n")
		}
	}
	if others := git("log", "--first-parent", "--no-merges", "--format=  - %s", from+".."+to); others != "" {
		text.WriteString("Other changes\n" + others + "\n")
	}
	return text.String()
}

func writeChanges(dir, text string) error {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	if err := atomic.Write(filepath.Join(dir, "changes.txt"), []byte(text), 0o666); err != nil {
		return err
	}
	return atomic.Write(filepath.Join(dir, "changes-unseen"), nil, 0o666)
}

func updateHyprtrixie() int {
	from := git("rev-parse", "HEAD")
	fmt.Printf("Updating %s\n", root())
	if run.Status("git", "-C", root(), "pull", "--ff-only") != 0 {
		return fail("git pull failed: the folder has local changes, or its branch has no upstream")
	}
	to := git("rev-parse", "HEAD")
	if from == to {
		fmt.Println("hyprtrixie is already up to date.")
		return 0
	}
	dir := changesDir()
	text := changesBetween(from, to)
	if err := writeChanges(dir, text); err != nil {
		fmt.Fprintln(os.Stderr, "cannot save the update notes:", err)
	}
	fmt.Printf("\nWhat changed:\n%s\n", text)
	return run.Report(run.Exec("sudo", script(), "install", "--changes", filepath.Join(dir, "changes.txt")))
}

func update() int {
	part := cmp.Or(env.Arg(2), "all")
	if !slices.Contains([]string{"all", "system", "apps", "hyprtrixie"}, part) {
		return fail("usage: hyprtrixie update [system | apps | hyprtrixie]")
	}
	if os.Geteuid() == 0 {
		return fail("run it as yourself, without sudo: hyprtrixie update")
	}
	wanted := func(name string) bool { return part == "all" || part == name }
	if wanted("hyprtrixie") && run.Quiet(run.Quick, "git", "-C", root(), "rev-parse", "--git-dir") != 0 {
		return fail("%s is not a git clone, so it can't update itself", root())
	}
	for _, step := range []struct {
		name string
		work func() int
	}{{"system", updateSystem}, {"apps", updateApps}, {"hyprtrixie", updateHyprtrixie}} {
		if wanted(step.name) {
			if status := step.work(); status != 0 {
				return status
			}
		}
	}
	return 0
}
