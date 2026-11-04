package main

import (
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

func presentationOn() int {
	run.Quiet(run.Quick, "makoctl", "mode", "-a", doNotDisturb)
	look.CallShell("awake", "on")
	run.Quiet(run.Quick, "hyprctl", "setcursor", cursorTheme, bigCursor)
	return reloadAll()
}

func presentationOff() int {
	run.Quiet(run.Quick, "makoctl", "mode", "-r", doNotDisturb)
	look.CallShell("awake", "off")
	run.Quiet(run.Quick, "hyprctl", "setcursor", cursorTheme, cursorSize)
	return reloadAll()
}

func presentation(args []string) int {
	on, off := putThen("presentation", "yes", presentationOn), putThen("presentation", "no", presentationOff)
	return onOff{"presentation", on, off}.run(args)
}
