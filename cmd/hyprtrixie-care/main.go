package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/care"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `Usage: hyprtrixie-care [firmware]

Looks after the system quietly. firmware tells you about new firmware from
fwupd, once per version. Without an argument it checks the firmware once a
day. A user timer runs it every hour.
`

const (
	firmwareGap = 20 * time.Hour
	askFirmware = "--ask-firmware"
	runFirmware = "--update-firmware"
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

func main() {
	switch env.Arg(1) {
	case "":
		dailyFirmware(run.Minute)
	case "firmware":
		firmware(run.Minute)
	case askFirmware:
		os.Exit(offerFirmware(env.Arg(2)))
	case runFirmware:
		os.Exit(updateFirmware())
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
