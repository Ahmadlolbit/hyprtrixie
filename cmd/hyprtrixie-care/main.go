package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/care"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/snapshots"
)

const usage = `Usage: hyprtrixie-care [firmware|disk]

Looks after the system quietly. firmware tells you about new firmware from
fwupd, once per version. disk warns when / is 90% full, once a day, and
offers to clean up old snapshots. Without an argument it does both, the
firmware once a day. A user timer runs it every hour.
`

const (
	firmwareGap = 20 * time.Hour
	askFirmware = "--ask-firmware"
	runFirmware = "--update-firmware"
	askDisk     = "--ask-disk"
	diskLimit   = 90
)

func stateFile(name string) string {
	return filepath.Join(env.StateHome(), "hyprtrixie", name)
}

func save(path, text string) {
	err := os.MkdirAll(filepath.Dir(path), 0o777)
	if err == nil {
		err = atomic.Write(path, []byte(text), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func self() string {
	return run.Sibling("hyprtrixie-care")
}

func askApart(unit string, args ...string) bool {
	return run.Within(run.Quick, "systemd-run", append([]string{"--user", "--collect", "--quiet", "--unit=" + unit, self()}, args...)...) == 0
}

func firmware(limit time.Duration) bool {
	output, err := run.CaptureQuiet(limit, "fwupdmgr", "get-updates", "--json")
	if run.TimedOut(err) {
		return false
	}
	waiting := care.Updates([]byte(output))
	seen := stateFile("firmware-seen")
	fresh := care.Unseen(waiting, fsutil.Optional(seen))
	if len(fresh) > 0 && askApart("hyprtrixie-care-firmware", askFirmware, strings.Join(fresh, "\n")) {
		save(seen, strings.Join(waiting, "\n")+"\n")
	}
	return true
}

func dailyFirmware(limit time.Duration) {
	stamp := stateFile("firmware-checked")
	if info, err := os.Stat(stamp); err == nil && time.Since(info.ModTime()) < firmwareGap {
		return
	}
	if firmware(limit) {
		save(stamp, "")
	}
}

func offerFirmware(body string) int {
	action := notify.Ask("--app-name=hyprtrixie", "--expire-time=0", "--action=default=Update", "Firmware update", body)
	if action != "default" {
		return 0
	}
	return run.Report(run.Exec("hyprtrixie-terminal", "--wait", self(), runFirmware))
}

func updateFirmware() int {
	run.Status("fwupdmgr", "refresh", "--force")
	return run.Status("fwupdmgr", "update")
}

func warned(use int, limit time.Duration) bool {
	title := fmt.Sprintf("The disk is %d%% full", use)
	all, err := snapshots.All(limit)
	if run.TimedOut(err) {
		return false
	}
	if len(all) == 0 {
		return notify.Post("--app-name=hyprtrixie", "--urgency=critical", title, "There are no snapshots to clean up.") == 0
	}
	return askApart("hyprtrixie-care-disk", askDisk, title, fmt.Sprintf("%d snapshots are on it. Clean up deletes the oldest automatic ones.", len(all)))
}

func disk(limit time.Duration) {
	full, err := strconv.Atoi(os.Getenv("HYPRTRIXIE_DISK_LIMIT"))
	if err != nil {
		full = diskLimit
	}
	use, ok := snapshots.DiskUse()
	if !ok || use < full {
		return
	}
	stamp := stateFile("disk-warned")
	today := time.Now().Format(time.DateOnly)
	if fsutil.Optional(stamp) != today && warned(use, limit) {
		save(stamp, today+"\n")
	}
}

func offerCleanup(title, body string) int {
	action := notify.Ask("--app-name=hyprtrixie", "--urgency=critical", "--expire-time=0", "--action=default=Clean up", title, body)
	if action != "default" {
		return 0
	}
	return run.Report(run.Exec("hyprtrixie-terminal", "--wait", "sudo", "hyprtrixie", "cleanup"))
}

func main() {
	switch env.Arg(1) {
	case "":
		disk(run.Minute)
		dailyFirmware(run.Minute)
	case "firmware":
		firmware(run.Minute)
	case "disk":
		disk(run.Minute)
	case askFirmware:
		os.Exit(offerFirmware(env.Arg(2)))
	case runFirmware:
		os.Exit(updateFirmware())
	case askDisk:
		os.Exit(offerCleanup(env.Arg(2), env.Arg(3)))
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
