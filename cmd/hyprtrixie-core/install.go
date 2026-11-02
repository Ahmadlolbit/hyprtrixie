package main

import (
	"errors"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/engine"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
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

func testingLogs() {
	person, ok := account.Target()
	if !ok {
		ui.Warn("no testing logs: cannot tell whose desktop this is")
		return
	}
	ui.Busy("Pack the testing logs")
	saved, err := gather(person, env.Root(), "hyprtrixie-logs-", true)
	ui.Done()
	if err != nil {
		ui.Warn(err.Error())
		return
	}
	ui.Print(saved)
}

func installAndPack(args []string) int {
	code := engine.Install(args)
	if options, err := engine.Parse(args); err == nil && options.Testing {
		testingLogs()
	}
	return code
}

func installCommand() int {
	return handedOver("install", installAndPack)
}

func verifyCommand() int {
	return handedOver("verify", engine.Verify)
}
