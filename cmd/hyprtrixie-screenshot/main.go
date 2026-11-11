package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/hypr"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `Usage: hyprtrixie-screenshot [region|window|screen]

Saves a screenshot to ~/Pictures/Screenshots, copies it to the clipboard and
shows a notification. Click the notification to annotate it.
`

const cancelled = 2

func windowArea() (string, bool) {
	window, ok := hypr.ActiveWindow()
	if !ok {
		return "", false
	}
	if window.Size[0] == 0 || window.Size[1] == 0 {
		fmt.Fprintln(os.Stderr, "no active window")
		notify.Post([]string{"--app-name=Screenshot"}, "No window to capture", "")
		return "", false
	}
	return fmt.Sprintf("%d,%d %dx%d", window.At[0], window.At[1], window.Size[0], window.Size[1]), true
}

func target(mode string) ([]string, int) {
	switch mode {
	case "region":
		area, err := run.Output("slurp", "-d")
		if err != nil {
			return nil, cancelled
		}
		return []string{"-g", area}, 0
	case "window":
		area, ok := windowArea()
		if !ok {
			return nil, 1
		}
		return []string{"-g", area}, 0
	default:
		output, ok := hypr.FocusedOutput()
		if !ok {
			return nil, 1
		}
		return []string{"-o", output}, 0
	}
}

func capture(mode, dir string) (string, int) {
	where, code := target(mode)
	if code != 0 {
		return "", code
	}
	file, err := atomic.Fresh(dir, "Screenshot from "+time.Now().Format("2006-01-02 15-04-05"), ".png", 0o666)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return "", 1
	}
	if run.Within(run.Quick, "grim", append(where, file)...) != 0 {
		os.Remove(file)
		return "", 1
	}
	return file, 0
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
	dir := filepath.Join(run.UserDir("PICTURES", filepath.Join(env.Home(), "Pictures")), "Screenshots")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	file, code := capture(mode, dir)
	switch code {
	case 0:
	case cancelled:
		return 0
	default:
		return 1
	}
	copyImage(file)
	shown := file
	if rest, ok := strings.CutPrefix(file, env.Home()); ok {
		shown = "~" + rest
	}
	action := notify.Ask([]string{"--app-name=Screenshot", "--icon=" + file, "--action=default=Edit"}, "Screenshot saved", shown+". Click to edit it.")
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
