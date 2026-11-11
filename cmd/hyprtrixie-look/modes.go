package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

type awakeState struct {
	Awake bool    `json:"awake"`
	Until float64 `json:"until"`
}

func awakeNow() awakeState {
	var state awakeState
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		return state
	}
	data, err := os.ReadFile(filepath.Join(runtime, "hyprtrixie-awake"))
	if err != nil || json.Unmarshal(data, &state) != nil {
		return awakeState{}
	}
	return state
}

func awakeAgain() {
	kept := stored(lookState("presentation-awake"))
	remove(lookState("presentation-awake"))
	until, err := strconv.ParseFloat(kept, 64)
	if kept == "" || err != nil || !awakeNow().Awake {
		look.CallShell("awake", "off")
		return
	}
	if until == 0 {
		return
	}
	left := 0
	if at, ok := moment(); ok {
		left = int(math.Ceil((until - float64(at.UnixMilli())) / 60000))
	}
	if left <= 0 {
		look.CallShell("awake", "off")
		return
	}
	look.CallShell("awake", "minutes", strconv.Itoa(left))
}

func presentationOn(again bool) int {
	if !doNotDisturbOn() {
		run.Quiet(run.Quick, "makoctl", "mode", "-a", doNotDisturb)
		save(lookState("presentation-quiet"), "yes")
	}
	if state := awakeNow(); state.Awake && !again {
		save(lookState("presentation-awake"), strconv.FormatFloat(state.Until, 'f', 0, 64))
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
	awakeAgain()
	run.Quiet(run.Quick, "hyprctl", "setcursor", cursorTheme, cursorSize)
	return reloadAll()
}

func presentation(args []string) int {
	again := settings.Get("presentation") == "yes"
	on := putThen("presentation", "yes", func() int { return presentationOn(again) })
	off := putThen("presentation", "no", presentationOff)
	ended := func() int {
		if settings.Get("presentation") != "yes" {
			return 0
		}
		return off()
	}
	return onOff{"presentation", on, ended}.run(args)
}
