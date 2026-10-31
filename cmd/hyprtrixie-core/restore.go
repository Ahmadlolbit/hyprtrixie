package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/snapshots"
)

func restoreList(points []snapshots.Snapshot) string {
	var list strings.Builder
	for _, snapshot := range slices.Backward(points) {
		fmt.Fprintf(&list, "%s\t%s\n", snapshot.Number, snapshot.Label())
	}
	return list.String()
}

func restoreTable(points []snapshots.Snapshot) string {
	var table strings.Builder
	for _, snapshot := range slices.Backward(points) {
		fmt.Fprintf(&table, "%6s  %s\n", snapshot.Number, snapshot.Label())
	}
	return table.String() + "\nGo back to one: hyprtrixie restore NUMBER\n"
}

func mounted(path string) bool {
	return run.Quiet(run.Quick, "mountpoint", "-q", path) == 0
}

func kept() string {
	home, logs := mounted("/home"), mounted("/var/log")
	switch {
	case home && logs:
		return "Going back keeps your files in /home and the logs in /var/log as they are.\n"
	case home:
		return "Going back keeps your files in /home as they are.\n"
	case logs:
		return "/home is part of this snapshot, so your files go back too. The logs in /var/log stay as they are.\n"
	}
	return "/home is part of this snapshot, so your files go back too.\n"
}

func newestKernel(boot string) string {
	kernels, _ := filepath.Glob(boot + "/vmlinuz-*")
	if len(kernels) == 0 {
		return ""
	}
	sorted, _ := run.CaptureQuiet(run.Quick, "ls", append([]string{"-v"}, kernels...)...)
	last := sorted[strings.LastIndexByte(sorted, '\n')+1:]
	return strings.TrimPrefix(filepath.Base(last), "vmlinuz-")
}

func kernelWarning(status string) string {
	boot := env.Or("HYPRTRIXIE_BOOT", "/boot")
	kernel := newestKernel(boot)
	if kernel == "" || !snapshots.KernelAdded(status, kernel) || !mounted(boot) {
		return ""
	}
	return "This snapshot doesn't have the newest kernel, " + kernel +
		". When the machine restarts, open Advanced options in the GRUB menu and pick the older kernel.\n"
}

func goBack(number string) int {
	defer cleanup.IgnoreQuit()()
	if code := say("Comparing snapshot " + number + " with today, this can take a minute...\n"); code != 0 {
		return code
	}
	status, err := run.CaptureQuiet(run.Slow, "snapper", "-c", "root", "status", number+"..0")
	if err != nil {
		return fail("snapshot %s can't be read: see hyprtrixie restore", number)
	}
	if code := say(snapshots.Changes(status) + "\n" + kept() + kernelWarning(status) + "\n"); code != 0 {
		return code
	}
	return run.Report(run.Exec("sudo", "snapper-rollback", number))
}

func showPoints(target string) int {
	points, err := snapshots.All(run.Minute)
	if err != nil {
		return fail("the snapshots can't be read")
	}
	if target == "list" {
		return say(restoreList(points))
	}
	return say(restoreTable(points))
}

func restore() int {
	target := env.Arg(2)
	if target != "" && target != "list" && !snapshots.IsNumber(target) {
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
