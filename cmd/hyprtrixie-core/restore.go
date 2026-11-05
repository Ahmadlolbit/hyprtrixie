package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/snapshots"
)

const (
	kernelFile = "vmlinuz-"
	initrdFile = "initrd.img-"
	mountField = 4
)

func restoreRows(points []snapshots.Snapshot, format string) string {
	var rows strings.Builder
	for _, snapshot := range slices.Backward(points) {
		fmt.Fprintf(&rows, format, snapshot.Number, snapshot.Label())
	}
	return rows.String()
}

func separate(path, root string) bool {
	var one, other syscall.Stat_t
	if syscall.Stat(path, &one) != nil || syscall.Stat(root, &other) != nil {
		return false
	}
	return one.Dev != other.Dev
}

func mountedAt(table, path string) bool {
	for _, line := range strings.Split(table, "\n") {
		if fields := strings.Fields(line); len(fields) > mountField && fields[mountField] == path {
			return true
		}
	}
	return false
}

func keptWord(root, table, rel, inside, apart, nested string) string {
	switch {
	case !separate(filepath.Join(root, rel), root):
		return inside
	case mountedAt(table, "/"+rel):
		return apart
	}
	return nested
}

func keptWords(root, table string) string {
	return keptWord(root, table, "home", "/home is part of this snapshot, so your files go back too.",
		"Going back keeps your files in /home as they are.",
		"/home is a subvolume inside the root, so it comes up empty after going back, and your files stay in the old root.") +
		keptWord(root, table, "var/log", "", " The logs in /var/log stay as they are.",
			" /var/log comes up empty, and the old logs stay in the old root.") + "\n"
}

func kernelsIn(boot string) ([]string, error) {
	entries, err := os.ReadDir(boot)
	if err != nil {
		return nil, err
	}
	var kernels []string
	for _, entry := range entries {
		version, found := strings.CutPrefix(entry.Name(), kernelFile)
		if found && fsutil.IsFile(filepath.Join(boot, initrdFile+version)) {
			kernels = append(kernels, version)
		}
	}
	return kernels, nil
}

func newestKernel(boot string, kernels []string) (string, error) {
	args := []string{"-v", "--quoting-style=literal"}
	for _, kernel := range kernels {
		args = append(args, filepath.Join(boot, kernelFile+kernel))
	}
	sorted, err := run.CaptureQuiet(run.Quick, "ls", args...)
	if err == nil && sorted == "" {
		err = os.ErrNotExist
	}
	last := sorted[strings.LastIndexByte(sorted, '\n')+1:]
	return strings.TrimPrefix(filepath.Base(last), kernelFile), err
}

func kernelAdvice(snapshotModules, boot, root string) string {
	couldNot := "Couldn't read the kernels in " + boot + ", so this can't tell if the snapshot has one that will start.\n"
	if !fsutil.IsDir(boot) {
		return couldNot
	}
	if !separate(boot, root) {
		return ""
	}
	kernels, err := kernelsIn(boot)
	if err != nil || len(kernels) == 0 {
		return couldNot
	}
	if _, err := os.ReadDir(snapshotModules); err != nil {
		return "Couldn't read the kernels in this snapshot, so this can't tell if it has one that will start.\n"
	}
	newest, err := newestKernel(boot, kernels)
	if err != nil {
		return couldNot
	}
	inSnapshot := func(kernel string) bool { return fsutil.IsDir(filepath.Join(snapshotModules, kernel)) }
	if inSnapshot(newest) {
		return ""
	}
	left := slices.DeleteFunc(kernels, func(kernel string) bool { return !inSnapshot(kernel) })
	if len(left) == 0 {
		return "This snapshot has no kernel that will start: every kernel in " + boot + " came after it.\n"
	}
	pick, err := newestKernel(boot, left)
	if err != nil {
		return couldNot
	}
	return "This snapshot doesn't have the newest kernel, " + newest + ". When the machine restarts, press Esc right after the laptop's logo, " +
		"open Advanced options and pick " + pick + ". Do that at every restart until the next update brings " + newest + " back.\n"
}

func goBack(number string) int {
	defer cleanup.IgnoreQuit()()
	if code := say("Comparing snapshot " + number + " with today, this can take a minute...\n"); code != 0 {
		return code
	}
	status, err := run.Capture(run.Slow, "snapper", "-c", "root", "status", number+"..0")
	if err != nil {
		return fail("snapshot %s can't be read: see hyprtrixie restore", number)
	}
	modules := filepath.Join(env.Or("HYPRTRIXIE_SNAPSHOTS", "/.snapshots"), number, "snapshot/usr/lib/modules")
	advice := kernelAdvice(modules, env.Or("HYPRTRIXIE_BOOT", "/boot"), "/")
	kept := keptWords("/", fsutil.Optional("/proc/self/mountinfo"))
	if code := say(snapshots.Changes(status) + "\n" + kept + advice + "\n"); code != 0 {
		return code
	}
	code := run.Status("sudo", "snapper-rollback", number)
	if code == 0 && advice != "" {
		return say("\n" + advice)
	}
	return code
}

func showPoints(target string) int {
	points, err := snapshots.All()
	switch {
	case err != nil:
		return fail("the snapshots can't be read")
	case target == "list":
		return say(restoreRows(points, "%s\t%s\n"))
	case len(points) == 0:
		return say("There are no snapshots yet.\n")
	}
	return say(restoreRows(points, "%6s  %s\n") + "\nGo back to one: hyprtrixie restore NUMBER\n")
}

func restore() int {
	target := env.Arg(2)
	if env.Arg(3) != "" || (target != "" && target != "list" && !snapshots.IsNumber(target)) {
		return fail("usage: hyprtrixie restore [list | NUMBER]")
	}
	if os.Geteuid() == 0 {
		return fail("run it as yourself, without sudo: hyprtrixie restore")
	}
	if !installed("snapper") {
		return fail("snapper isn't installed, so there are no restore points")
	}
	if target == "" || target == "list" {
		return showPoints(target)
	}
	return goBack(target)
}
