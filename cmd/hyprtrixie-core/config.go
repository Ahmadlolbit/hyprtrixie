package main

import (
	"cmp"
	"fmt"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

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
	}
	return fail("usage: hyprtrixie config [show|path|edit|reset]")
}
