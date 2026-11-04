package osinstall

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/disks"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

const hosts = "127.0.0.1\tlocalhost\n127.0.1.1\t%s\n::1\t\tlocalhost ip6-localhost ip6-loopback\nff02::1\t\tip6-allnodes\nff02::2\t\tip6-allrouters\n"

var (
	bound       = []string{"dev", "dev/pts", "proc", "sys", "sys/firmware/efi/efivars", "run"}
	wanted      = []string{"sudo", "adm", "audio", "video", "plugdev", "netdev", "users", "lpadmin", "bluetooth"}
	livePieces  = []string{"live-boot", "live-boot-initramfs-tools", "live-config", "live-config-systemd", "live-tools"}
	windowsMenu = "GRUB_DISABLE_OS_PROBER=false\nGRUB_TIMEOUT_STYLE=menu\nGRUB_TIMEOUT=5\n"
)

type install struct {
	answers Answers
	plan    disks.Plan
}

func squashfs() string {
	return env.Or("HYPRTRIXIE_SQUASHFS", "/run/live/medium/live/filesystem.squashfs")
}

func target() string {
	return env.Or("HYPRTRIXIE_TARGET", disks.Target)
}

func inTarget(path string) string {
	return filepath.Join(target(), path)
}

func chroot(log *runlog.Log, args ...string) error {
	return run.Logged(log, "chroot", append([]string{target()}, args...)...)
}

func Grouped(steps []disks.Step) [][]disks.Step {
	var groups [][]disks.Step
	for _, each := range steps {
		if each.Title != "" || len(groups) == 0 {
			groups = append(groups, nil)
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], each)
	}
	return groups
}

func (work *install) commands(group []disks.Step) step.Work {
	return func(log *runlog.Log) error {
		for _, each := range group {
			var err error
			if each.Secret {
				err = run.Supply(run.Minute, log, work.answers.Password, each.Args[0], each.Args[1:]...)
			} else {
				err = run.Logged(log, each.Args[0], each.Args[1:]...)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func (work *install) clean(log *runlog.Log) error {
	run.Quiet(run.Minute, "umount", "-R", target())
	run.Quiet(run.Minute, "umount", "-R", "/run/hyprtrixie-top")
	if fsutil.Exists("/dev/mapper/" + disks.Mapper) {
		run.Quiet(run.Minute, "cryptsetup", "close", disks.Mapper)
	}
	for _, point := range work.plan.Disk.Mounted() {
		if err := run.Logged(log, "umount", point); err != nil && mounted(point) {
			return err
		}
	}
	for _, each := range work.plan.Disk.Release() {
		if err := run.Logged(log, each.Args[0], each.Args[1:]...); err != nil && fsutil.Exists(each.Args[len(each.Args)-1]) {
			return err
		}
	}
	return nil
}

func mounted(point string) bool {
	return run.Quiet(run.Quick, "findmnt", "--mountpoint", point) == 0
}

func (work *install) copySystem(log *runlog.Log) error {
	if !fsutil.IsFile(squashfs()) {
		return errors.New(squashfs() + " is missing: the installer runs from the live system")
	}
	return run.Logged(log, "unsquashfs", "-f", "-no-progress", "-d", target(), squashfs())
}

func uuid(device string) (string, error) {
	value, err := run.CaptureQuiet(run.Quick, "blkid", "-s", "UUID", "-o", "value", device)
	if err == nil && value == "" {
		err = errors.New(device + " has no UUID")
	}
	return value, err
}

func (work *install) tables(*runlog.Log) error {
	var ids disks.IDs
	for _, pair := range []struct {
		into   *string
		device string
	}{{&ids.Root, "/dev/mapper/" + disks.Mapper}, {&ids.Luks, work.plan.RootDevice()}, {&ids.Boot, work.plan.BootDevice()}, {&ids.ESP, work.plan.ESPDevice()}} {
		value, err := uuid(pair.device)
		if err != nil {
			return err
		}
		*pair.into = value
	}
	if err := atomic.Install(inTarget("etc/fstab"), []byte(ids.Fstab()), 0o644); err != nil {
		return err
	}
	return atomic.Install(inTarget("etc/crypttab"), []byte(ids.Crypttab()), 0o644)
}

func (work *install) bind(log *runlog.Log) error {
	for _, dir := range bound {
		if !fsutil.IsDir("/" + dir) {
			continue
		}
		if err := os.MkdirAll(inTarget(dir), 0o755); err != nil {
			return err
		}
		if err := run.Logged(log, "mount", "--bind", "/"+dir, inTarget(dir)); err != nil {
			return err
		}
	}
	return nil
}

func (work *install) purgeLive(log *runlog.Log) error {
	output, _ := run.CaptureQuiet(run.Minute, "chroot", target(), "dpkg-query", "-W", "-f=${Package} ${Status}\n")
	var present []string
	for _, line := range run.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) == 4 && slices.Contains(livePieces, fields[0]) && fields[3] == "installed" {
			present = append(present, fields[0])
		}
	}
	if len(present) == 0 {
		return nil
	}
	return chroot(log, append([]string{"dpkg", "--purge"}, present...)...)
}
