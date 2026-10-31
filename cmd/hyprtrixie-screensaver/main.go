package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

const usage = `Usage: hyprtrixie-screensaver start|toggle|status

start   shows the screensaver on every monitor (the idle timer runs this)
toggle  turns the screensaver off or back on for your account
status  prints on or off

The text comes from ~/.config/hyprtrixie/screensaver.txt when it exists,
otherwise from /usr/share/hyprtrixie/screensaver.txt.
`

const (
	class      = "hyprtrixie.screensaver"
	sharedText = "/usr/share/hyprtrixie/screensaver.txt"
	kitty      = "hl.dsp.exec_cmd([[kitty --class " + class + " -o background=#000000 -o background_opacity=1 -o window_padding_width=0 -o confirm_os_window_close=0 -o font_size=16 -e hyprtrixie-screensaver run]])"
	mouseOn    = "\x1b[?1003h\x1b[?1006h"
	mouseOff   = "\x1b[?1006l\x1b[?1003l"

	effectLimit = time.Hour
)

var (
	effects = []string{"beams", "binarypath", "blackhole", "bouncyballs", "bubbles", "burn", "colorshift", "crumble", "decrypt", "errorcorrect", "expand", "fireworks",
		"highlight", "laseretch", "matrix", "middleout", "orbittingvolley", "overflow", "pour", "print", "rain", "randomsequence", "rings", "scattered", "slice",
		"slide", "spotlights", "spray", "swarm", "sweep", "synthgrid", "unstable", "vhstape", "waves", "wipe"}
	monitorName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

func configDir() string {
	return filepath.Join(env.ConfigHome(), "hyprtrixie")
}

func offFile() string {
	return filepath.Join(configDir(), "screensaver-off")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func textFile() string {
	own := filepath.Join(configDir(), "screensaver.txt")
	if info, err := os.Stat(own); err == nil && info.Mode().IsRegular() {
		return own
	}
	return sharedText
}

func running() bool {
	return run.Quiet(run.Quick, "pgrep", "-f", "kitty --class "+class) == 0
}

func locked() bool {
	return run.Quiet(run.Quick, "pidof", "hyprlock") == 0
}

func monitors() []string {
	output, err := run.Capture(run.Quick, "hyprctl", "monitors", "-j")
	if err != nil {
		return nil
	}
	var found []struct {
		Name    string `json:"name"`
		Focused bool   `json:"focused"`
	}
	if err := json.Unmarshal([]byte(output), &found); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}
	var others, last []string
	for _, monitor := range found {
		switch {
		case !monitorName.MatchString(monitor.Name):
		case monitor.Focused:
			last = append(last, monitor.Name)
		default:
			others = append(others, monitor.Name)
		}
	}
	return append(others, last...)
}

func dispatch(command string) int {
	_, err := run.Capture(run.Quick, "hyprctl", "dispatch", command)
	return run.Report(err)
}

func start() int {
	if exists(offFile()) || running() || locked() {
		return 0
	}
	code := 0
	for _, monitor := range monitors() {
		dispatch(`hl.dsp.focus({ monitor = "` + monitor + `" })`)
		code = dispatch(kitty)
	}
	return code
}

func focused() bool {
	output, err := run.Capture(run.Quick, "hyprctl", "activewindow", "-j")
	if err != nil {
		return false
	}
	var window struct {
		Class string `json:"class"`
	}
	return json.Unmarshal([]byte(output), &window) == nil && window.Class == class
}

func cursor(invisible string) {
	run.Quiet(run.Quick, "hyprctl", "eval", "hl.config({ cursor = { invisible = "+invisible+" } })")
}

func show(file string) <-chan struct{} {
	finished := make(chan struct{}, 1)
	effect := effects[rand.IntN(len(effects))]
	go func() {
		run.Within(effectLimit, "tte", "--input-file", file, "--frame-rate", "60", "--canvas-width", "0", "--canvas-height", "0",
			"--anchor-canvas", "c", "--anchor-text", "c", effect)
		finished <- struct{}{}
	}()
	return finished
}

func keys() <-chan struct{} {
	pressed := make(chan struct{}, 1)
	go func() {
		key := make([]byte, 1)
		if count, _ := os.Stdin.Read(key); count > 0 {
			pressed <- struct{}{}
		}
	}()
	return pressed
}

func watch(file string, pressed <-chan struct{}, signals <-chan os.Signal) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	finished := show(file)
	for {
		select {
		case <-pressed:
			return
		case <-signals:
			return
		case <-finished:
			finished = nil
		case <-ticker.C:
			if !focused() || locked() {
				return
			}
			if finished == nil {
				finished = show(file)
			}
		}
	}
}

func saver() {
	signal.Ignore(syscall.SIGTSTP)
	file := textFile()
	signals := make(chan os.Signal, 1)
	cleanup.Notify(signals)
	cursor("true")
	time.Sleep(500 * time.Millisecond)
	restore := term.Raw()
	fmt.Print(mouseOn)
	watch(file, keys(), signals)
	fmt.Print(mouseOff)
	restore()
	cursor("false")
	run.Within(run.Quick, "pkill", "-f", "kitty --class "+class)
}

func toggle() int {
	if exists(offFile()) {
		if err := os.Remove(offFile()); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return notify.Post("--app-name=Screensaver", "Screensaver on")
	}
	err := os.MkdirAll(configDir(), 0o777)
	if err == nil {
		err = atomic.Write(offFile(), nil, 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return notify.Post("--app-name=Screensaver", "Screensaver off")
}

func main() {
	switch env.Arg(1) {
	case "start":
		os.Exit(start())
	case "run":
		saver()
	case "toggle":
		os.Exit(toggle())
	case "status":
		if exists(offFile()) {
			fmt.Println("off")
		} else {
			fmt.Println("on")
		}
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
