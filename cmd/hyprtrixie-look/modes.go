package main

import (
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/look"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	cursorTheme = "Bibata-Modern-Ice"
	cursorSize  = "24"
	bigCursor   = "48"
)

func reloadAll() int {
	run.Quiet(run.Quick, "hyprctl", "reload")
	look.ReloadShell()
	return 0
}

func zen(args []string) int {
	return onOff{"zen", putThen("zen", "yes", reloadAll), putThen("zen", "no", reloadAll)}.run(args)
}

func doNotDisturbOn() bool {
	modes, err := run.CaptureQuiet(run.Quick, "makoctl", "mode")
	return err == nil && slices.Contains(strings.Fields(modes), doNotDisturb)
}

func presentationOn() int {
	if !doNotDisturbOn() {
		run.Quiet(run.Quick, "makoctl", "mode", "-a", doNotDisturb)
		save(lookState("presentation-quiet"), "yes")
	}
	look.CallShell("awake", "on")
	run.Quiet(run.Quick, "hyprctl", "setcursor", cursorTheme, bigCursor)
	return reloadAll()
}

func presentationOff() int {
	if stored(lookState("presentation-quiet")) == "yes" {
		if !quietWanted() {
			run.Quiet(run.Quick, "makoctl", "mode", "-r", doNotDisturb)
		}
		remove(lookState("presentation-quiet"))
	}
	look.CallShell("awake", "off")
	run.Quiet(run.Quick, "hyprctl", "setcursor", cursorTheme, cursorSize)
	return reloadAll()
}

func presentation(args []string) int {
	on, off := putThen("presentation", "yes", presentationOn), putThen("presentation", "no", presentationOff)
	ended := func() int {
		if settings.Get("presentation") != "yes" {
			return 0
		}
		return off()
	}
	return onOff{"presentation", on, ended}.run(args)
}
