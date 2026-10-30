package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/hypr"
	"github.com/Ahmadlolbit/hyprtrixie/internal/look"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `Usage: hyprtrixie-background <file>
       hyprtrixie-background next
       hyprtrixie-background --list
       hyprtrixie-background --current
       hyprtrixie-background workspaces

Sets the wallpaper. Backgrounds come from the current theme and from
~/.config/omarchy/backgrounds/<theme>. With a wallpaper per workspace on
(hyprtrixie-look workspaces on), each workspace shows the next one along,
and workspaces shows each monitor's.
`

func state() string {
	return filepath.Join(env.StateHome(), "omarchy", "current")
}

func link() string {
	return filepath.Join(state(), "background")
}

func current() string {
	target, _ := os.Readlink(link())
	return target
}

func backgrounds() []string {
	name, _ := fsutil.Text(filepath.Join(state(), "theme.name"))
	return fsutil.Images(filepath.Join(state(), "theme", "backgrounds"), filepath.Join(env.ConfigHome(), "omarchy", "backgrounds", name))
}

func next() (string, bool) {
	found := backgrounds()
	if len(found) == 0 {
		return "", false
	}
	return found[(slices.Index(found, current())+1)%len(found)], true
}

func perWorkspace() bool {
	return look.Open().Get("workspaces") == "yes"
}

func inHyprland() bool {
	return os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != ""
}

func workspaces() bool {
	if !inHyprland() {
		return false
	}
	found := backgrounds()
	if len(found) == 0 {
		return true
	}
	offset := max(slices.Index(found, current()), 0)
	monitors, _ := hypr.QuietMonitors()
	for _, monitor := range monitors {
		if monitor.ActiveWorkspace.ID == nil {
			continue
		}
		id := max(*monitor.ActiveWorkspace.ID, 1)
		if !hypr.Wallpaper(monitor.Name, found[(id-1+offset)%len(found)]) {
			return false
		}
	}
	return true
}

func overIPC(file string) bool {
	if !inHyprland() {
		return false
	}
	monitors, ok := hypr.QuietMonitors()
	if !ok || len(monitors) == 0 {
		return false
	}
	for _, monitor := range monitors {
		if !hypr.Wallpaper(monitor.Name, file) {
			return false
		}
	}
	return true
}

func restartPaper() {
	if run.Within(run.Quick, "pkill", "-x", "hyprpaper") == 0 {
		run.Quiet(3*time.Second, "pidwait", "-x", "hyprpaper")
		run.Quiet(run.Quick, "setsid", "-f", "hyprtrixie-start", "hyprpaper")
	}
}

func apply(file string) int {
	if !fsutil.IsFile(file) {
		fmt.Fprintf(os.Stderr, "no such background: %s\n", file)
		return 1
	}
	err := os.MkdirAll(state(), 0o777)
	if err == nil {
		err = atomic.Symlink(file, link())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if perWorkspace() {
		if workspaces() {
			return 0
		}
	} else if overIPC(file) {
		return 0
	}
	restartPaper()
	return 0
}

func reason(err error) string {
	var number syscall.Errno
	if !errors.As(err, &number) {
		return err.Error()
	}
	text := number.Error()
	return strings.ToUpper(text[:1]) + text[1:]
}

func set(argument string) int {
	file, err := fsutil.RealPath(argument)
	if err != nil {
		fmt.Fprintf(os.Stderr, "realpath: %s: %s\n", argument, reason(err))
	}
	return apply(file)
}

func moveOn() int {
	file, ok := next()
	if !ok {
		fmt.Fprintln(os.Stderr, "the current theme has no backgrounds")
		return 1
	}
	return apply(file)
}

func showCurrent() int {
	target, err := os.Readlink(link())
	if err != nil {
		return 1
	}
	fmt.Println(target)
	return 0
}

func showWorkspaces() int {
	if perWorkspace() && !workspaces() {
		return 1
	}
	return 0
}

func main() {
	command := env.Arg(1)
	switch command {
	case "":
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	case "-h", "--help":
		fmt.Print(usage)
	case "--list":
		for _, file := range backgrounds() {
			fmt.Println(file)
		}
	case "--current":
		os.Exit(showCurrent())
	case "workspaces":
		os.Exit(showWorkspaces())
	case "next":
		os.Exit(moveOn())
	default:
		os.Exit(set(command))
	}
}
