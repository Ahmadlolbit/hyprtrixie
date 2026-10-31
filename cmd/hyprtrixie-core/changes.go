package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

func changesDir() string {
	return filepath.Join(env.StateHome(), "hyprtrixie")
}

func notifyChanges(dir string) int {
	unseen := filepath.Join(dir, "changes-unseen")
	if !fsutil.IsFile(unseen) {
		return 0
	}
	text := fsutil.Optional(filepath.Join(dir, "changes.txt"))
	if text == "" {
		return 0
	}
	remove(unseen)
	count := strings.Count("\n"+text, "\n  - ")
	action := notify.Ask("--app-name=hyprtrixie", "--action=default=Read", "hyprtrixie was updated",
		fmt.Sprintf("%d changes since the last update. Click to read them, or open Menu › Update › What changed.", count))
	if action == "default" {
		return run.Report(run.Exec("hyprtrixie-terminal", "--wait", script(), "changes"))
	}
	return 0
}

func changes() int {
	dir := changesDir()
	if env.Arg(2) == "--notify" {
		return notifyChanges(dir)
	}
	if text := fsutil.Optional(filepath.Join(dir, "changes.txt")); text != "" {
		fmt.Println(text)
	} else {
		fmt.Println("No update notes yet. hyprtrixie update writes them.")
	}
	return remove(filepath.Join(dir, "changes-unseen"))
}
