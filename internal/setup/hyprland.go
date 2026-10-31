package setup

import (
	"cmp"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const oldestHyprland = "0.55"

func hyprlandRecent(*runlog.Log) error {
	version := apt.Version("hyprland")
	return quietly("found "+cmp.Or(version, "no Hyprland"), "dpkg", "--compare-versions", version, "ge", oldestHyprland)
}

func quickshellWorks(*runlog.Log) error {
	return quietly("qs --version failed", "qs", "--version")
}

func hyprlandSteps(context *Context) {
	context.Run.Step("Install desktop services", packages("desktop"))
	context.Run.Step("Install Hyprland from backports", packages("hyprland"))
}

func hyprlandChecks(context *Context) {
	context.Run.Check("Desktop services are installed", func(log *runlog.Log) error { return apt.ListsPresent(log, "desktop") })
	context.Run.Check("Hyprland 0.55 or newer is installed", hyprlandRecent)
	context.Run.Check("Quickshell runs", quickshellWorks)
}
