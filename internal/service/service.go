package service

import (
	"io"

	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ledger"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

var inChroot = detect.Chroot

func Enabled(unit string) bool {
	return run.Quiet(run.Minute, "systemctl", "is-enabled", "--quiet", unit) == 0
}

func Active(unit string) bool {
	return run.Quiet(run.Minute, "systemctl", "is-active", "--quiet", unit) == 0
}

func Enable(out io.Writer, units ...string) error {
	for _, unit := range units {
		ledger.Note(ledger.Service, unit)
		if Enabled(unit) {
			continue
		}
		if err := run.Logged(out, "systemctl", "enable", unit); err != nil {
			return err
		}
	}
	return nil
}

func Replace(out io.Writer, unit string) error {
	ledger.Note(ledger.Service, unit)
	return run.Logged(out, "systemctl", "enable", "--force", unit)
}

func Start(out io.Writer, units ...string) error {
	for _, unit := range units {
		if err := Enable(out, unit); err != nil {
			return err
		}
		if Active(unit) {
			continue
		}
		if err := run.Logged(out, "systemctl", "start", unit); err != nil {
			return err
		}
	}
	return nil
}

func Disable(out io.Writer, units ...string) error {
	for _, unit := range units {
		if !Enabled(unit) && (inChroot() || !Active(unit)) {
			continue
		}
		if err := run.Logged(out, "systemctl", disabling(unit)...); err != nil {
			return err
		}
	}
	return nil
}

func disabling(unit string) []string {
	if inChroot() {
		return []string{"disable", unit}
	}
	return []string{"disable", "--now", unit}
}

func Restart(out io.Writer, unit string) error {
	return run.Logged(out, "systemctl", "restart", unit)
}

func Reload(out io.Writer) error {
	return run.Logged(out, "systemctl", "daemon-reload")
}
