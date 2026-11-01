package setup

import (
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

func install(name string) step.Work {
	return func(log *runlog.Log) error { return apt.Install(log, name) }
}

func installed(name string) step.Work {
	return func(*runlog.Log) error { return apt.Present(name) }
}

func commit() string {
	output, err := run.CaptureQuiet(run.Quick, "git", "-C", env.Root(), "rev-parse", "--short", "HEAD")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(output)
}

func (context *Context) admin() string {
	if context.Person.Name == "" {
		return "not chosen yet"
	}
	return context.Person.Name
}

func (context *Context) recordRun(log *runlog.Log) error {
	log.Line("info", "hyprtrixie "+env.Version+" ("+commit()+") from "+env.Root())
	log.Line("info", "admin user: "+context.admin())
	for _, line := range strings.Split(strings.TrimSuffix(context.Config.Show(), "\n"), "\n") {
		log.Line("info", "config "+line)
	}
	return nil
}

func initSteps(context *Context) {
	context.Run.Step("Install gum for the setup screens", install("gum"))
	context.Run.Step("Install curl for downloads", install("curl"))
	context.Run.Step("Record what this run uses", context.recordRun)
}

func logsWritable(log *runlog.Log) error {
	return holds(log.Writable())
}

func initChecks(context *Context) {
	context.Run.Check("curl is installed", installed("curl"))
	context.Run.Check("gum is installed", installed("gum"))
	context.Run.Check("Logs are being written", logsWritable)
}
