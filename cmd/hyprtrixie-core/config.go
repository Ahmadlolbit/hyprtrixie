package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/presets"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const importUsage = "usage: sudo hyprtrixie config import FILE|https://ADDRESS"

func configEdit() int {
	path := config.Path()
	if !fsutil.IsFile(path) {
		if err := config.Defaults().Save(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	return run.Status(env.Or("EDITOR", "nano"), path)
}

func configReset() int {
	remove(config.Path())
	return say("Configuration reset; defaults apply on the next run.\n")
}

func curlReady() error {
	if installed("curl") {
		return nil
	}
	log := runlog.Joined()
	log.Mirror(func(data []byte) { os.Stderr.Write(data) })
	return apt.Install(log, "curl")
}

func presetFile(source string) (string, func(), error) {
	if !presets.Remote(source) {
		if strings.Contains(source, "://") || !fsutil.IsFile(source) {
			return "", nil, fmt.Errorf("no preset at %s: give a file, or an https address", source)
		}
		return source, func() {}, nil
	}
	if err := curlReady(); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "hyprtrixie-preset-")
	if err != nil {
		return "", nil, err
	}
	done := func() { os.RemoveAll(dir) }
	path := filepath.Join(dir, "preset.conf")
	if err := presets.Fetch(source, path); err != nil {
		done()
		return "", nil, err
	}
	return path, done, nil
}

func configImport() int {
	source := env.Arg(3)
	if source == "" {
		return fail(importUsage)
	}
	path, done, err := presetFile(source)
	if err != nil {
		return fail("%v", err)
	}
	defer done()
	current := config.Read(config.Path())
	seen := len(current.Notes)
	changes := presets.Import(current, path)
	for _, note := range current.Notes[seen:] {
		fmt.Fprintln(os.Stderr, note.Text)
	}
	if err := current.Save(config.Path()); err != nil {
		return fail("%v", err)
	}
	var report strings.Builder
	for _, change := range changes {
		fmt.Fprintf(&report, "%s: %q -> %q\n", change.Key, change.Before, change.After)
	}
	if len(changes) == 0 {
		report.WriteString("The preset changes nothing.\n")
	}
	report.WriteString("Run sudo hyprtrixie install, or hyprtrixie for the setup, to set the computer up with it.\n")
	return say(report.String())
}

func configCommand() int {
	switch cmp.Or(env.Arg(2), "show") {
	case "show":
		return say(config.Read(config.Path()).Show())
	case "path":
		return say(config.Path() + "\n")
	case "edit":
		return asRoot("config edit", configEdit)
	case "reset":
		return asRoot("config reset", configReset)
	case "export":
		return say(presets.Export(config.Read(config.Path())))
	case "import":
		return asRoot("config import", configImport)
	}
	return fail("usage: hyprtrixie config [show|path|edit|reset|export|import FILE|URL]")
}
