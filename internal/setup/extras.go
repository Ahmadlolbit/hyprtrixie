package setup

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

const (
	flathubURL    = "https://dl.flathub.org/repo/flathub.flatpakrepo"
	bazaarApp     = "io.github.kolunmi.Bazaar"
	steamApp      = "com.valvesoftware.Steam"
	mangohudLayer = "org.freedesktop.Platform.VulkanLayer.MangoHud"
)

var (
	printUnits  = []string{"cups.service", "cups-browsed.service", "avahi-daemon.service"}
	printGroups = []string{"lpadmin", "scanner"}
)

func (context *Context) extra(name string) bool {
	return context.Config.Enabled("EXTRAS", name)
}

func (context *Context) flathubWanted() bool {
	return context.extra("flatpak") || context.extra("gaming")
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

func steamMangoHud(log *runlog.Log) error {
	runtime, err := run.CaptureQuiet(run.Minute, "flatpak", "info", "--system", "--show-runtime", steamApp)
	if err != nil {
		return err
	}
	runtime = strings.TrimSpace(runtime)
	branch := runtime[strings.LastIndex(runtime, "/")+1:]
	if branch == "" {
		return fmt.Errorf("no runtime for Steam: %q", runtime)
	}
	return flatpakInstall(mangohudLayer + "//" + branch)(log)
}

func printServices(log *runlog.Log) error {
	return service.Start(log, printUnits...)
}

func (context *Context) printAccess(log *runlog.Log) error {
	return context.Person.AddGroups(log, printGroups...)
}

func printReady(*runlog.Log) error {
	for _, unit := range printUnits {
		if !service.Enabled(unit) {
			return fmt.Errorf("%s is off", unit)
		}
	}
	return nil
}

func (context *Context) printAccessReady(*runlog.Log) error {
	for _, group := range printGroups {
		if !context.Person.InGroup(group) {
			return fmt.Errorf("you are not in the %s group", group)
		}
	}
	return nil
}

func extrasSteps(context *Context) {
	if context.flathubWanted() {
		context.Run.Step("Install Flatpak", packages("extras-flatpak"))
		context.Run.Step("Add Flathub", flathubAdd)
	}
	if context.extra("flatpak") {
		context.Run.Step("Install the Bazaar store", flatpakInstall(bazaarApp))
	}
	if context.extra("gaming") {
		context.Run.Step("Install GameMode and MangoHud", packages("extras-gaming"))
		context.Run.Step("Install Steam", flatpakInstall(steamApp))
		context.Run.Step("Add MangoHud to Steam", steamMangoHud)
	}
	if context.extra("printing") {
		context.Run.Step("Install printing and scanning", packages("extras-printing"))
		context.Run.Step("Turn on printing and finding printers on the network", printServices)
		context.Run.Step("Let you manage printers and scanners", context.printAccess)
	}
	if context.extra("distrobox") {
		context.Run.Step("Install distrobox", packages("extras-distrobox"))
		context.Run.Step("Let Podman run boxes as you", context.podmanInstall)
	}
}

func extrasChecks(context *Context) {
	if context.flathubWanted() {
		context.Run.Check("Flathub is set up", flathubReady)
	}
	if context.extra("flatpak") {
		context.Run.Check("Bazaar is installed", flatpakApps(bazaarApp))
	}
	if context.extra("gaming") {
		context.Run.Check("GameMode and MangoHud are installed", commands("gamemoded", "mangohud"))
		context.Run.Check("Steam is installed", flatpakApps(steamApp))
	}
	if context.extra("printing") {
		context.Run.Check("Printing and scanning tools are installed", commands("lpstat", "scanimage", "system-config-printer", "simple-scan"))
		context.Run.Check("Printing and finding printers are on", printReady)
		context.Run.Check("You can manage printers and scanners", context.printAccessReady)
	}
	if context.extra("distrobox") {
		context.Run.Check("distrobox and Podman are installed", commands("distrobox", "podman"))
	}
}
