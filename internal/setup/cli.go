package setup

import (
	"os"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fetch"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

var cliCommands = []string{
	"batcat", "fdfind", "rg", "fzf", "eza", "btop", "lazygit", "fastfetch", "grim", "slurp", "wl-copy", "swappy",
	"brightnessctl", "upower", "ufw", "restic", "bat", "fd",
}

func shortName(target, name string) error {
	path := filepath.Join(env.BinDir(), name)
	if !fsutil.Runnable(target) || fsutil.Exists(path) {
		return nil
	}
	return atomic.Symlink(target, path)
}

func cliNames(*runlog.Log) error {
	if err := os.MkdirAll(env.BinDir(), programMode); err != nil {
		return err
	}
	if err := shortName("/usr/bin/batcat", "bat"); err != nil {
		return err
	}
	return shortName("/usr/bin/fdfind", "fd")
}

func cliSteps(context *Context) {
	context.Run.Step("Install command-line tools", packages("cli"))
	context.Run.Step("Add short names for bat and fd", cliNames)
	context.Run.Step("Install lazydocker", func(log *runlog.Log) error {
		return fetch.ReleaseExtract(log, "lazydocker", "lazydocker", filepath.Join(env.BinDir(), "lazydocker"), programMode)
	})
}

func cliChecks(context *Context) {
	context.Run.Check("Command-line tools are available", commands(cliCommands...))
	context.Run.Check("lazydocker is installed", commands("lazydocker"))
}
