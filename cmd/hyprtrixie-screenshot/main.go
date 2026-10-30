package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `Usage: hyprtrixie-screenshot [region|window|screen]

Saves a screenshot to ~/Pictures/Screenshots, copies it to the clipboard and
shows a notification. Click the notification to annotate it.
`

func hyprctl(target any, query string) bool {
	output, err := run.Capture(run.Quick, "hyprctl", "-j", query)
	if err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(output), target); err != nil {
		fmt.Fprintf(os.Stderr, "hyprctl %s: %v\n", query, err)
		return false
	}
	return true
}

func windowArea() (string, bool) {
	var window struct {
		At   [2]int `json:"at"`
		Size [2]int `json:"size"`
	}
	if !hyprctl(&window, "activewindow") {
		return "", false
	}
	if window.Size[0] == 0 || window.Size[1] == 0 {
		fmt.Fprintln(os.Stderr, "no active window")
		return "", false
	}
	return fmt.Sprintf("%d,%d %dx%d", window.At[0], window.At[1], window.Size[0], window.Size[1]), true
}

func focusedOutput() (string, bool) {
	var monitors []struct {
		Name    string `json:"name"`
		Focused bool   `json:"focused"`
	}
	if !hyprctl(&monitors, "monitors") {
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

func capture(mode, file string) int {
	switch mode {
	case "region":
		area, err := run.Output("slurp", "-d")
		if err != nil {
			return 2
		}
		return run.Within(run.Quick, "grim", "-g", area, file)
	case "window":
		area, ok := windowArea()
		if !ok {
			return 1
		}
		return run.Within(run.Quick, "grim", "-g", area, file)
	default:
		output, ok := focusedOutput()
		if !ok {
			return 1
		}
		return run.Within(run.Quick, "grim", "-o", output, file)
	}
}

func picturesDir() string {
	if dir, err := run.CaptureQuiet(run.Quick, "xdg-user-dir", "PICTURES"); err == nil && dir != "" && dir != env.Home() {
		return dir
	}
	return filepath.Join(env.Home(), "Pictures")
}

func copyImage(file string) {
	image, err := os.Open(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer image.Close()
	run.Feed(run.Quick, image, "wl-copy", "--type", "image/png")
}

func shoot(mode string) int {
	dir := filepath.Join(picturesDir(), "Screenshots")
	file := filepath.Join(dir, "Screenshot from "+time.Now().Format("2006-01-02 15-04-05")+".png")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	switch capture(mode, file) {
	case 0:
	case 2:
		return 0
	default:
		return 1
	}
	copyImage(file)
	shown := file
	if rest, ok := strings.CutPrefix(file, env.Home()); ok {
		shown = "~" + rest
	}
	action := notify.Ask("--app-name=Screenshot", "--icon="+file, "--action=default=Edit", "Screenshot saved", shown+". Click to edit it.")
	if action == "default" {
		return run.Status("swappy", "-f", file, "-o", file)
	}
	return 0
}

func main() {
	mode := env.Arg(1)
	if mode == "" {
		mode = "region"
	}
	switch mode {
	case "region", "window", "screen":
		os.Exit(shoot(mode))
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
