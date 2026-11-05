package main

import (
	"os"
	"os/user"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

var fingerprintTools = map[string]string{"add": "fprintd-enroll", "list": "fprintd-list", "remove": "fprintd-delete"}

func fingerprintCommand() int {
	tool, known := fingerprintTools[env.Arg(2)]
	if !known || env.Arg(3) != "" {
		return fail("usage: hyprtrixie fingerprint add|list|remove")
	}
	if os.Geteuid() == 0 {
		return fail("Run it without sudo: the fingers are your own.")
	}
	if !installed(tool) {
		return fail("Fingerprints aren't set up. Turn on Fingerprint in the setup's Security, then run: sudo hyprtrixie install --only 12,15")
	}
	if env.Arg(2) == "add" {
		return run.Status(tool)
	}
	me, err := user.Current()
	if err != nil {
		return fail("%v", err)
	}
	return run.Status(tool, me.Username)
}
