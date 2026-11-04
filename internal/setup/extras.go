package setup

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

const (
	flathubURL = "https://dl.flathub.org/repo/flathub.flatpakrepo"
	bazaarApp  = "io.github.kolunmi.Bazaar"
)

func (context *Context) extra(name string) bool {
	return context.Config.Enabled("EXTRAS", name)
}

func flathubAdd(log *runlog.Log) error {
	return run.Logged(log, "flatpak", "remote-add", "--system", "--if-not-exists", "flathub", flathubURL)
}

func flatpakInstall(apps ...string) step.Work {
	return func(log *runlog.Log) error {
		return run.Logged(log, "flatpak", append([]string{"install", "--system", "--noninteractive", "-y", "flathub"}, apps...)...)
	}
}

func flathubReady(*runlog.Log) error {
	remotes, err := run.CaptureQuiet(run.Minute, "flatpak", "remotes", "--system", "--columns=name")
	if err != nil {
		return err
	}
	return expect(slices.Contains(strings.Fields(remotes), "flathub"), "Flathub is not set up")
}

func flatpakApps(apps ...string) step.Work {
	return func(*runlog.Log) error {
		for _, app := range apps {
			if run.Quiet(run.Minute, "flatpak", "info", "--system", app) != 0 {
				return fmt.Errorf("%s is not installed", app)
			}
		}
		return nil
	}
}

func extrasSteps(context *Context) {
	if context.extra("flatpak") {
		context.Run.Step("Install Flatpak", packages("extras-flatpak"))
		context.Run.Step("Add Flathub", flathubAdd)
		context.Run.Step("Install the Bazaar store", flatpakInstall(bazaarApp))
	}
}

func extrasChecks(context *Context) {
	if context.extra("flatpak") {
		context.Run.Check("Flathub is set up", flathubReady)
		context.Run.Check("Bazaar is installed", flatpakApps(bazaarApp))
	}
}
