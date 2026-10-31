package hypr

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

type Monitor struct {
	Name            string `json:"name"`
	Focused         bool   `json:"focused"`
	ActiveWorkspace struct {
		ID *int `json:"id"`
	} `json:"activeWorkspace"`
}

type Window struct {
	Class string `json:"class"`
	At    [2]int `json:"at"`
	Size  [2]int `json:"size"`
}

func Monitors() ([]Monitor, bool) {
	output, err := run.Capture(run.Quick, "hyprctl", "monitors", "-j")
	return parsed[[]Monitor]("monitors", output, err)
}

func QuietMonitors() ([]Monitor, bool) {
	output, err := run.CaptureQuiet(run.Quick, "hyprctl", "monitors", "-j")
	var found []Monitor
	if err != nil || json.Unmarshal([]byte(output), &found) != nil {
		return nil, false
	}
	return found, true
}

func ActiveWindow() (Window, bool) {
	output, err := run.Capture(run.Quick, "hyprctl", "activewindow", "-j")
	return parsed[Window]("activewindow", output, err)
}

func parsed[T any](what, output string, failed error) (T, bool) {
	var value, nothing T
	if failed != nil {
		return nothing, false
	}
	if err := json.Unmarshal([]byte(output), &value); err != nil {
		fmt.Fprintf(os.Stderr, "hyprctl %s: %v\n", what, err)
		return nothing, false
	}
	return value, true
}

func Wallpaper(monitor, file string) bool {
	return run.Quiet(run.Quick, "hyprctl", "hyprpaper", "wallpaper", monitor+","+file+",cover") == 0
}
