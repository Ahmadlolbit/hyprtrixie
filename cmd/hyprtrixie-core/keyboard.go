package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/keyboard"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

func listLayouts() int {
	var listing strings.Builder
	for _, layout := range keyboard.Layouts() {
		fmt.Fprintf(&listing, "%s\t%s\n", layout.ID, layout.Label)
	}
	return say(listing.String())
}

func setLayout(id string) int {
	if !keyboard.Known(id) {
		return fail("no keyboard layout named %s; see: hyprtrixie keyboard list", id)
	}
	text := keyboard.With(fsutil.Optional(keyboard.File()), id)
	if err := atomic.Write(keyboard.File(), []byte(text), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	run.Quiet(run.Slow, "setupcon", "--save-only")
	return say("The keyboard layout is " + id + ". The console and the login screen use it after a restart.\n")
}

func keyboardCommand() int {
	switch id := env.Arg(2); id {
	case "":
		return say(keyboard.Current() + "\n")
	case "list":
		return listLayouts()
	default:
		return asRoot("keyboard "+id, func() int { return setLayout(id) })
	}
}
