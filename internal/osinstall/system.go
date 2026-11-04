package osinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/disks"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/keyboard"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const firstBootUnit = `[Unit]
Description=Finish setting up hyprtrixie
Wants=network-online.target
After=network-online.target
Before=display-manager.service
ConditionPathExists=/etc/hyprtrixie/first-boot

[Service]
Type=oneshot
Environment=HYPRTRIXIE_USER=%s
ExecStartPre=-/usr/bin/plymouth display-message --text="Finishing the setup, which takes a few minutes"
ExecStart=%s first-start
TimeoutStartSec=2h
StandardOutput=journal+console

[Install]
WantedBy=multi-user.target
`

func Groups(table string) []string {
	var present []string
	for _, line := range strings.Split(table, "\n") {
		name, _, _ := strings.Cut(line, ":")
		if slices.Contains(wanted, name) {
			present = append(present, name)
		}
	}
	slices.Sort(present)
	return present
}

func (work *install) account(log *runlog.Log) error {
	groups := Groups(fsutil.Optional(inTarget("etc/group")))
	args := []string{"useradd", "-m", "-s", "/bin/bash", "-c", work.answers.Name}
	if len(groups) > 0 {
		args = append(args, "-G", strings.Join(groups, ","))
	}
	if err := chroot(log, append(args, work.answers.User)...); err != nil {
		return err
	}
	return run.Supply(run.Minute, log, work.answers.User+":"+work.answers.Secret+"\n", "chroot", target(), "chpasswd")
}

func (work *install) name(*runlog.Log) error {
	if err := atomic.Install(inTarget("etc/hostname"), []byte(work.answers.Host+"\n"), 0o644); err != nil {
		return err
	}
	return atomic.Install(inTarget("etc/hosts"), []byte(fmt.Sprintf(hosts, work.answers.Host)), 0o644)
}

func LocaleGen(current, locale string) (string, bool) {
	line := locale + " UTF-8"
	lines := strings.Split(current, "\n")
	for index, each := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(each), "#"))
		if trimmed == line {
			changed := strings.TrimSpace(each) != line
			lines[index] = line
			return strings.Join(lines, "\n"), changed
		}
	}
	return strings.TrimRight(current, "\n") + "\n" + line + "\n", true
}

func (work *install) localize(log *runlog.Log) error {
	zone := filepath.Join("/usr/share/zoneinfo", work.answers.Zone)
	if !fsutil.Exists(inTarget(zone)) {
		return fmt.Errorf("there's no time zone %s", work.answers.Zone)
	}
	if err := atomic.Symlink(zone, inTarget("etc/localtime")); err != nil {
		return err
	}
	if err := atomic.Install(inTarget("etc/timezone"), []byte(work.answers.Zone+"\n"), 0o644); err != nil {
		return err
	}
	if keyboard.Known(work.answers.Keyboard) {
		text := keyboard.With(fsutil.Optional(inTarget("etc/default/keyboard")), work.answers.Keyboard)
		if err := atomic.Install(inTarget("etc/default/keyboard"), []byte(text), 0o644); err != nil {
			return err
		}
	}
	if err := work.languages(); err != nil {
		return err
	}
	text, changed := LocaleGen(fsutil.Optional(inTarget("etc/locale.gen")), work.answers.Locale)
	if !changed {
		return nil
	}
	if err := atomic.Install(inTarget("etc/locale.gen"), []byte(text), 0o644); err != nil {
		return err
	}
	return chroot(log, "locale-gen")
}

func (work *install) languages() error {
	line := []byte("LANG=" + work.answers.Locale + "\n")
	if err := atomic.Install(inTarget("etc/locale.conf"), line, 0o644); err != nil {
		return err
	}
	if fsutil.IsLink(inTarget("etc/default/locale")) {
		return nil
	}
	return atomic.Install(inTarget("etc/default/locale"), line, 0o644)
}

func (work *install) identity(log *runlog.Log) error {
	if err := atomic.Install(inTarget("etc/machine-id"), nil, 0o444); err != nil {
		return err
	}
	if err := run.Logged(log, "systemd-machine-id-setup", "--root", target()); err != nil {
		return err
	}
	keys, _ := filepath.Glob(inTarget("etc/ssh/ssh_host_*"))
	for _, key := range keys {
		if err := os.Remove(key); err != nil {
			return err
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return chroot(log, "ssh-keygen", "-A")
}

func (work *install) bootImage(log *runlog.Log) error {
	return chroot(log, "update-initramfs", "-u", "-k", "all")
}

func (work *install) bootLoader(log *runlog.Log) error {
	args := []string{"grub-install", "--target=x86_64-efi", "--efi-directory=/boot/efi", "--bootloader-id=debian", "--recheck"}
	if !work.plan.Beside {
		args = append(args, "--force-extra-removable")
	}
	if err := chroot(log, args...); err != nil {
		return err
	}
	if work.plan.Beside {
		if err := os.MkdirAll(inTarget("etc/default/grub.d"), 0o755); err != nil {
			return err
		}
		if err := atomic.Install(inTarget("etc/default/grub.d/zz-windows.cfg"), []byte(windowsMenu), 0o644); err != nil {
			return err
		}
	}
	return chroot(log, "update-grub")
}

func clone() string {
	return env.Or("HYPRTRIXIE_ROOT", "/opt/hyprtrixie")
}

func (work *install) firstBoot(log *runlog.Log) error {
	home := filepath.Join("/home", work.answers.User, "hyprtrixie")
	if err := run.Logged(log, "mv", inTarget(clone()), inTarget(home)); err != nil {
		return err
	}
	if err := chroot(log, "chown", "-R", work.answers.User+":"+work.answers.User, home); err != nil {
		return err
	}
	if err := os.MkdirAll(inTarget("etc/hyprtrixie"), 0o755); err != nil {
		return err
	}
	config, err := os.ReadFile(work.answers.Config)
	if err != nil {
		return err
	}
	unit := fmt.Sprintf(firstBootUnit, work.answers.User, filepath.Join(home, "hyprtrixie"))
	for path, data := range map[string][]byte{"etc/hyprtrixie/hyprtrixie.conf": config, "etc/hyprtrixie/first-boot": nil, "etc/systemd/system/hyprtrixie-first-boot.service": []byte(unit)} {
		if err := atomic.Install(inTarget(path), data, 0o644); err != nil {
			return err
		}
	}
	return chroot(log, "systemctl", "enable", "hyprtrixie-first-boot.service")
}

func (work *install) close(log *runlog.Log) error {
	if run.Logged(log, "umount", "-R", target()) != nil {
		if err := run.Logged(log, "umount", "-R", "-l", target()); err != nil {
			return err
		}
	}
	if run.Logged(log, "cryptsetup", "close", disks.Mapper) != nil {
		log.Line("warn", "the encrypted disk stays open until the restart")
	}
	return nil
}
