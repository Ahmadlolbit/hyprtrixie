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

const diskLimit = 90

func save(path, text string) {
	err := os.MkdirAll(filepath.Dir(path), 0o777)
	if err == nil {
		err = atomic.Write(path, []byte(text), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func stateFolder() string {
	return filepath.Join(env.StateHome(), "hyprtrixie")
}

func stamp(name string) string {
	return filepath.Join(env.Or("XDG_RUNTIME_DIR", stateFolder()), "hyprtrixie-"+name)
}

func today() string {
	return time.Now().Format(time.DateOnly)
}

func offer(app, urgency, title, body string) bool {
	return notify.Post("--app-name="+app, "--urgency="+urgency, "--expire-time=0", "--hint=string:x-dunst-stack-tag:"+app, "--", title, body) == 0
}

func firmware() bool {
	output, err := run.Capture(run.Minute, "fwupdmgr", "get-updates", "--json")
	waiting := care.Updates([]byte(output))
	seen := filepath.Join(stateFolder(), "firmware-seen")
	fresh := care.Unseen(waiting, fsutil.Optional(seen))
	if len(fresh) > 0 {
		if !offer("hyprtrixie-firmware", "normal", "Firmware update", care.Markup(strings.Join(fresh, "\n"))+"\nClick to update.") {
			return false
		}
		save(seen, strings.Join(waiting, "\n")+"\n")
	}
	return err == nil
}

func dailyFirmware() {
	checked := stamp("firmware-checked")
	if fsutil.Optional(checked) != today() && firmware() {
		save(checked, today())
	}
}

func warned(use int) bool {
	title := fmt.Sprintf("The disk is %d%% full", use)
	all, _ := snapshots.All()
	if len(snapshots.Disposable(all)) == 0 {
		return notify.Post("--app-name=hyprtrixie", "--urgency=critical", "--", title) == 0
	}
	return offer("hyprtrixie-disk", "critical", title, fmt.Sprintf("%d snapshots are on it. Click to delete the oldest automatic ones.", len(all)))
}

func disk() {
	full, err := strconv.Atoi(os.Getenv("HYPRTRIXIE_DISK_LIMIT"))
	if err != nil {
		full = diskLimit
	}
	use, ok := snapshots.DiskUse()
	warnedToday := stamp("disk-warned")
	if ok && use >= full && fsutil.Optional(warnedToday) != today() && warned(use) {
		save(warnedToday, today())
	}
}

func main() {
	switch env.Arg(1) {
	case "":
		disk()
		dailyFirmware()
	case "firmware":
		firmware()
	case "disk":
		disk()
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
