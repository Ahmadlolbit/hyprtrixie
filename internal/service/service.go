package service

import (
	"io"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

func Enabled(unit string) bool {
	return run.Quiet(run.Minute, "systemctl", "is-enabled", "--quiet", unit) == 0
}

func Active(unit string) bool {
	return run.Quiet(run.Minute, "systemctl", "is-active", "--quiet", unit) == 0
}

func Enable(out io.Writer, units ...string) error {
	for _, unit := range units {
		if Enabled(unit) {
			continue
		}
		if err := run.Logged(out, "systemctl", "enable", unit); err != nil {
			return err
		}
	}
	return nil
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
		if !Enabled(unit) && !Active(unit) {
			continue
		}
		if err := run.Logged(out, "systemctl", "disable", "--now", unit); err != nil {
			return err
		}
	}
	return nil
}

func Restart(out io.Writer, unit string) error {
	return run.Logged(out, "systemctl", "restart", unit)
}

func Reload(out io.Writer) error {
	return run.Logged(out, "systemctl", "daemon-reload")
}
