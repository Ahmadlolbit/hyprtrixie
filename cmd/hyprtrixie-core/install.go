package main

import (
	"errors"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/engine"
)

func handedOver(mode string, body func([]string) int) int {
	if os.Getenv("HYPRTRIXIE_ENGINE") == "" {
		return backToBash()
	}
	args := os.Args[2:]
	_, err := engine.Parse(args)
	switch {
	case errors.Is(err, engine.ErrHelp):
		return showHelp()
	case err != nil:
		return fail("%v", err)
	}
	return asRoot(mode, func() int { return body(args) })
}

func installCommand() int {
	return handedOver("install", engine.Install)
}

func verifyCommand() int {
	return handedOver("verify", engine.Verify)
}
