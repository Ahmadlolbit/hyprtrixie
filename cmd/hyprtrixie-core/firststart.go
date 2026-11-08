package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const firstStartTries = 3

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
	note := firstStartNotePath(person)
	if run.Quiet(run.Quick, "runuser", "-u", person.Name, "--", "mkdir", "-p", filepath.Dir(note)) != 0 {
		return fmt.Errorf("cannot make %s", filepath.Dir(note))
	}
	text := fmt.Sprintf(firstStartNote, filepath.Join(env.LogDir(), "latest.log"))
	return person.Write(io.Discard, note, text)
}

func countFirstStart() error {
	marker := firstStartMarker()
	if !fsutil.Exists(marker) {
		return nil
	}
	tries, _ := strconv.Atoi(strings.TrimSpace(fsutil.Optional(marker)))
	if tries+1 >= firstStartTries {
		return os.Remove(marker)
	}
	return atomic.Install(marker, []byte(strconv.Itoa(tries+1)+"\n"), 0o644)
}

func firstStartCommand() int {
	if os.Geteuid() != 0 {
		return fail("the first start runs as root")
	}
	if err := countFirstStart(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	person, found := account.Target()
	if found {
		if err := leaveFirstStartNote(person); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	os.Setenv("HYPRTRIXIE_FIRST_START", "1")
	code := run.Status(script(), "install", "--config", config.Path(), "--on-failure", "continue", "--yes")
	remove(firstStartMarker())
	if code == 0 && found {
		person.Run(os.Stderr, "rm", "-f", firstStartNotePath(person))
	}
	return code
}
