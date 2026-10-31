package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/restic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	usbUnit      = "restic-backup-usb.service"
	unreadable   = 3
	interrupted  = 128
	serviceLimit = time.Minute
)

func unitFile() string {
	return filepath.Join(env.ConfigHome(), "systemd", "user", usbUnit)
}

func self() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func mountUnit(mount string) (string, bool) {
	unit, err := run.Capture(run.Quick, "systemd-escape", "-p", "--suffix=mount", mount)
	return unit, err == nil
}

func writeUnit(unit string) error {
	program, err := self()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitFile()), 0o777); err != nil {
		return err
	}
	return atomic.Write(unitFile(), []byte(restic.Unit(unit, program)), 0o666)
}

func enableUnit() int {
	if code := run.Within(serviceLimit, "systemctl", "--user", "daemon-reload"); code != 0 {
		return code
	}
	return run.Within(serviceLimit, "systemctl", "--user", "enable", usbUnit)
}

func usbOn() int {
	if !useRepository() {
		return 1
	}
	mount, ok := restic.USBMount(os.Getenv("RESTIC_REPOSITORY"))
	if !ok {
		fmt.Fprintln(os.Stderr, "The backup location is not on a USB drive (under /media or /run/media).")
		return 1
	}
	unit, ok := mountUnit(mount)
	if !ok {
		return 1
	}
	if err := writeUnit(unit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if code := enableUnit(); code != 0 {
		return code
	}
	fmt.Printf("Backups now start by themselves when %s is plugged in.\n", mount)
	return 0
}

func usbOff() int {
	run.Quiet(serviceLimit, "systemctl", "--user", "disable", usbUnit)
	if err := os.Remove(unitFile()); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
	}
	run.Within(serviceLimit, "systemctl", "--user", "daemon-reload")
	fmt.Println("Backups no longer start by themselves.")
	return 0
}

func usbRun() int {
	notify.Post("--app-name=hyprtrixie", "Backup started", "Leave the drive plugged in.")
	cleanup.Notify(make(chan os.Signal, 1))
	code := backup()
	finished := time.Now().Format("15:04")
	switch {
	case code == 0:
		notify.Post("--app-name=hyprtrixie", "Backup done", finished+": your home folder is on the drive.")
		return 0
	case code == unreadable:
		notify.Post("--app-name=hyprtrixie", "Backup done with a warning", finished+": some files could not be read. Run restic-backup in a terminal to see which.")
		return 0
	case code > interrupted:
		notify.Post("--app-name=hyprtrixie", "--urgency=critical", "Backup stopped", "It did not finish.")
		return 1
	}
	notify.Post("--app-name=hyprtrixie", "--urgency=critical", "Backup failed", "Run restic-backup in a terminal to see why.")
	return 1
}

func usb([]string) int {
	switch env.Arg(2) {
	case "on":
		return usbOn()
	case "off":
		return usbOff()
	case "run":
		return usbRun()
	}
	fmt.Fprint(os.Stderr, usage)
	return 2
}
