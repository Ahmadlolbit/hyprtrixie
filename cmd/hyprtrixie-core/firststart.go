package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const firstStartNote = `The first start didn't finish the whole setup: some of it needs the internet, or it ran out of time.
Connect, then run: sudo hyprtrixie install --yes
What happened is in ~/runbook-report.md and %s.
`

func firstStartMarker() string {
	return env.Or("HYPRTRIXIE_FIRST_BOOT", "/etc/hyprtrixie/first-boot")
}

func firstStartNotePath(person account.Account) string {
	return filepath.Join(person.Home, ".local", "state", "hyprtrixie", "first-start.txt")
}

func leaveFirstStartNote(person account.Account) error {
	uid, err := strconv.Atoi(person.UID)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(person.GID)
	if err != nil {
		return err
	}
	note := firstStartNotePath(person)
	if run.Quiet(run.Quick, "runuser", "-u", person.Name, "--", "mkdir", "-p", filepath.Dir(note)) != 0 {
		return fmt.Errorf("cannot make %s", filepath.Dir(note))
	}
	text := fmt.Sprintf(firstStartNote, filepath.Join(env.LogDir(), "latest.log"))
	return atomic.InstallAs(note, []byte(text), 0o644, uid, gid)
}

func firstStartCommand() int {
	if os.Geteuid() != 0 {
		return fail("the first start runs as root")
	}
	remove(firstStartMarker())
	person, found := account.Target()
	if found {
		if err := leaveFirstStartNote(person); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	code := run.Status(script(), "install", "--config", config.Path(), "--on-failure", "continue", "--yes")
	if code == 0 && found {
		remove(firstStartNotePath(person))
	}
	return code
}
