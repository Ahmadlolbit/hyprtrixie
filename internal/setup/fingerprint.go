package setup

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	FingerprintLine = "auth       sufficient   pam_fprintd.so max-tries=2 timeout=10"
	pamCommonAuth   = "@include common-auth"
)

func sudoPAM() string {
	return env.Or("HYPRTRIXIE_PAM_SUDO", "/etc/pam.d/sudo")
}

func commonAuthPAM() string {
	return env.Or("HYPRTRIXIE_PAM_COMMON_AUTH", "/etc/pam.d/common-auth")
}

func fprintdProfile() string {
	return env.Or("HYPRTRIXIE_PAM_FPRINTD", "/usr/share/pam-configs/fprintd")
}

func fingerprintOnlyWhereAsked(log *runlog.Log) error {
	if !fsutil.IsFile(fprintdProfile()) {
		return nil
	}
	return run.LoggedWithin(run.Minute, log, "pam-auth-update", "--disable", "fprintd")
}

func fingerprintEverywhere() bool {
	current, _ := fsutil.Text(commonAuthPAM())
	return strings.Contains(current, "pam_fprintd")
}

func sameFields(line, wanted string) bool {
	return slices.Equal(strings.Fields(line), strings.Fields(wanted))
}

func WithFingerprint(text string, wanted bool) (string, bool) {
	lines := slices.DeleteFunc(strings.Split(text, "\n"), func(line string) bool { return sameFields(line, FingerprintLine) })
	if !wanted {
		return strings.Join(lines, "\n"), true
	}
	at := slices.IndexFunc(lines, func(line string) bool { return sameFields(line, pamCommonAuth) })
	if at < 0 {
		return text, false
	}
	return strings.Join(slices.Insert(lines, at, FingerprintLine), "\n"), true
}

func fingerprintFile(path string, wanted bool) error {
	current, err := fsutil.Text(path)
	if err != nil {
		return err
	}
	changed, ok := WithFingerprint(current, wanted)
	if !ok {
		return fmt.Errorf("%s has no %q line for the finger to go before", path, pamCommonAuth)
	}
	if changed == current {
		return nil
	}
	return atomic.Install(path, []byte(changed+"\n"), plainMode)
}

func fingerprintSudo(wanted bool) error {
	for index, path := range []string{sudoPAM(), sudoPAM() + "-i"} {
		if !fsutil.IsFile(path) && (index > 0 || !wanted) {
			continue
		}
		if err := fingerprintFile(path, wanted); err != nil {
			return err
		}
	}
	return nil
}

func fingerprintSetup(log *runlog.Log) error {
	if err := apt.InstallLists(log, "fingerprint"); err != nil {
		return err
	}
	if fingerprintEverywhere() {
		if err := fingerprintOnlyWhereAsked(log); err != nil {
			return err
		}
	}
	return fingerprintSudo(true)
}

func fingerprintInSudo() bool {
	current, _ := fsutil.Text(sudoPAM())
	return slices.ContainsFunc(strings.Split(current, "\n"), func(line string) bool { return sameFields(line, FingerprintLine) })
}

func fingerprintOff(log *runlog.Log) error {
	if err := fingerprintOnlyWhereAsked(log); err != nil {
		return err
	}
	return fingerprintSudo(false)
}

func fingerprintReady(log *runlog.Log) error {
	if err := apt.ListsPresent(log, "fingerprint"); err != nil {
		return err
	}
	if fingerprintEverywhere() {
		return fmt.Errorf("%s lets a finger into every login, not only sudo and the lock screen", commonAuthPAM())
	}
	return expect(fingerprintInSudo(), "sudo doesn't ask for a finger")
}
