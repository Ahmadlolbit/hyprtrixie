package main

import (
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/events"
	"github.com/Ahmadlolbit/hyprtrixie/internal/osinstall"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
	"github.com/Ahmadlolbit/hyprtrixie/internal/words"
)

const installOSUsage = "usage: hyprtrixie install-os [--events] --answers FILE"

func installOSArgs() (string, bool, bool) {
	answers, streamed := "", false
	for index := 2; env.Arg(index) != ""; index++ {
		switch env.Arg(index) {
		case "--events":
			streamed = true
		case "--answers":
			index++
			answers = env.Arg(index)
		default:
			return "", false, false
		}
	}
	return answers, streamed, answers != ""
}

func installOSCommand() int {
	answers, streamed, ok := installOSArgs()
	if !ok {
		return fail(installOSUsage)
	}
	if os.Geteuid() != 0 {
		return fail("the installer runs as root")
	}
	if streamed {
		cleanup.OwnGroup()
		ui.Attach(events.Open(os.Stdin, os.Stdout, cleanup.StopGroup))
	}
	log, err := runlog.Start("install-os")
	if err != nil {
		return run.Report(err)
	}
	if streamed {
		log.Mirror(ui.Mirror)
	}
	chosen, err := osinstall.Read(answers)
	os.Remove(answers)
	if err == nil {
		err = osinstall.Ready()
	}
	if err != nil {
		return fail("%v", err)
	}
	plan, err := osinstall.PlanFor(chosen)
	if err != nil {
		return fail("%v", err)
	}
	runner := step.New(log, "install", "halt")
	osinstall.Steps(runner, chosen, plan)
	ui.Rule(events.SummaryTitle)
	if code := runner.ExitCode(); code != 0 {
		ui.Print(words.T("The install stopped at the step that failed, and nothing after it ran.") + "\n")
		ui.Print(words.T("Its log") + ": " + log.Path() + "\n")
		return code
	}
	ui.Print(words.T("hyprtrixie is installed. Restart, take the USB stick out, and type your disk password.") + "\n")
	ui.Print(words.T("The first start finishes the setup before the login screen, which takes a few minutes.") + "\n")
	return 0
}
