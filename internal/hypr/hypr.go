package hypr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const monitorCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-"

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

func FocusedOutput() (string, bool) {
	monitors, ok := Monitors()
	if !ok {
		return "", false
	}
	for _, monitor := range monitors {
		if monitor.Focused {
			return monitor.Name, true
		}
	}
	fmt.Fprintln(os.Stderr, "no focused monitor")
	return "", false
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

func ValidMonitor(name string) bool {
	return name != "" && strings.Trim(name, monitorCharacters) == ""
}

func Session(runtime string) ([]string, bool) {
	entries, _ := os.ReadDir(filepath.Join(runtime, "hypr"))
	for _, entry := range entries {
		hyprctl := []string{"env", "XDG_RUNTIME_DIR=" + runtime, "HYPRLAND_INSTANCE_SIGNATURE=" + entry.Name(), "hyprctl"}
		if entry.IsDir() && run.Quiet(run.Quick, hyprctl[0], append(hyprctl[1:], "version")...) == 0 {
			return hyprctl, true
		}
	}
	return nil, false
}

func Running(runtime string) []string {
	entries, _ := os.ReadDir(filepath.Join(runtime, "hypr"))
	for _, entry := range entries {
		lock := strings.Fields(fsutil.Optional(filepath.Join(runtime, "hypr", entry.Name(), "hyprland.lock")))
		instance := "HYPRLAND_INSTANCE_SIGNATURE=" + entry.Name()
		if entry.IsDir() && len(lock) > 1 && run.Quiet(run.Quick, "env", "XDG_RUNTIME_DIR="+runtime, instance, "hyprctl", "version") == 0 {
			return []string{instance, "WAYLAND_DISPLAY=" + lock[1]}
		}
	}
	return nil
}

func Wallpaper(monitor, file string) bool {
	return run.Quiet(run.Quick, "hyprctl", "hyprpaper", "wallpaper", monitor+","+file+",cover") == 0
}
