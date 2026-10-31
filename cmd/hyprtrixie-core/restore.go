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

const kernelFile = "vmlinuz-"

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

func keptWords(root string) string {
	text := "/home is part of this snapshot, so your files go back too."
	if separate(filepath.Join(root, "home"), root) {
		text = "Going back keeps your files in /home as they are."
	}
	if separate(filepath.Join(root, "var/log"), root) {
		text += " The logs in /var/log stay as they are."
	}
	return text + "\n"
}

func kernelsIn(boot, modules string) ([]string, error) {
	entries, err := os.ReadDir(boot)
	if err != nil {
		return nil, err
	}
	var kernels []string
	for _, entry := range entries {
		version, found := strings.CutPrefix(entry.Name(), kernelFile)
		if found && fsutil.IsDir(filepath.Join(modules, version)) {
			kernels = append(kernels, version)
		}
	}
	return kernels, nil
}

func newestKernel(boot string, kernels []string) string {
	args := []string{"-v"}
	for _, kernel := range kernels {
		args = append(args, filepath.Join(boot, kernelFile+kernel))
	}
	sorted, _ := run.CaptureQuiet(run.Quick, "ls", args...)
	last := sorted[strings.LastIndexByte(sorted, '\n')+1:]
	return strings.TrimPrefix(filepath.Base(last), kernelFile)
}

func kernelAdvice(status, boot, modules, root string) string {
	kernels, err := kernelsIn(boot, modules)
	if err != nil || len(kernels) == 0 {
		return "Couldn't read the kernels in " + boot + ", so this can't tell if the snapshot has one that will start.\n"
	}
	if !separate(boot, root) {
		return ""
	}
	newest := newestKernel(boot, kernels)
	if !snapshots.KernelAdded(status, newest) {
		return ""
	}
	left := slices.DeleteFunc(kernels, func(kernel string) bool { return snapshots.KernelAdded(status, kernel) })
	if len(left) == 0 {
		return "This snapshot has no kernel that will start: every kernel in " + boot + " came after it.\n"
	}
	return "This snapshot doesn't have the newest kernel, " + newest + ". When the machine restarts, press Esc right after the laptop's logo, " +
		"open Advanced options and pick " + newestKernel(boot, left) + ".\n"
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
	advice := kernelAdvice(status, env.Or("HYPRTRIXIE_BOOT", "/boot"), env.Or("HYPRTRIXIE_KERNEL_MODULES", "/usr/lib/modules"), "/")
	if code := say(snapshots.Changes(status) + "\n" + keptWords("/") + advice + "\n"); code != 0 {
		return code
	}
	code := run.Status("sudo", "snapper-rollback", number)
	if closing := say("\nIf it says the rollback is done, restart to use the snapshot.\n"); closing != 0 {
		return closing
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
