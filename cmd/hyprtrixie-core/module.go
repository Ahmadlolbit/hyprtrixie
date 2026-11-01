package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/setup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

const (
	resultMode  = 0o600
	moduleBroke = 2
)

func handBack(runner *step.Runner) error {
	var result strings.Builder
	fmt.Fprintf(&result, "steps|%d\nchecks|%d\n", runner.Steps, runner.Checks)
	for _, mark := range runner.Failures {
		fmt.Fprintf(&result, "failed|%s|%s\n", mark.Kind, mark.Title)
	}
	for _, mark := range runner.Skips {
		fmt.Fprintf(&result, "skipped|%s|%s\n", mark.Kind, mark.Title)
	}
	if runner.Skipped {
		result.WriteString("module-skipped\n")
	}
	if err := atomic.Write(os.Getenv("HYPRTRIXIE_MODULE_RESULT"), []byte(result.String()), resultMode); err != nil {
		return fmt.Errorf("cannot hand the module's results back: %w", err)
	}
	return nil
}

func moduleContext(id, part string) (*setup.Context, error) {
	log := runlog.Joined()
	person, _ := account.Lookup(os.Getenv("HYPRTRIXIE_USER"))
	settings := config.Read(config.Path(), os.Getenv("HYPRTRIXIE_RUN_CONFIG"))
	context := &setup.Context{Run: step.New(log, id, os.Getenv("HYPRTRIXIE_FAILURE_MODE")), Person: person, Config: settings}
	if part != "steps" {
		return context, nil
	}
	deployer, err := files.New(log, person)
	context.Deploy = deployer
	return context, err
}

func moduleBroken(format string, args ...any) int {
	fail(format, args...)
	return moduleBroke
}

func runModule() int {
	kind, id := env.Arg(2), env.Arg(3)
	body, known := setup.Modules[id]
	if !known {
		return moduleBroken("no module %s in Go", id)
	}
	part := map[string]func(*setup.Context){"steps": body.Steps, "checks": body.Checks}[kind]
	if part == nil {
		return moduleBroken("usage: hyprtrixie-core module steps|checks ID")
	}
	context, err := moduleContext(id, kind)
	if err != nil {
		return moduleBroken("cannot read what's installed: %v", err)
	}
	part(context)
	if err := handBack(context.Run); err != nil {
		return moduleBroken("%v", err)
	}
	return context.Run.ExitCode()
}

func module() int {
	if os.Getenv("HYPRTRIXIE_MODULE_RESULT") == "" {
		return backToBash()
	}
	return asRoot("install", runModule)
}
