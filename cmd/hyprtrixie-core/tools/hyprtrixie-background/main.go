package background

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
       hyprtrixie-background workspace MONITOR WORKSPACE

Sets the wallpaper. Backgrounds come from the current theme and from
~/.config/hyprtrixie/backgrounds/<theme>. With a wallpaper per workspace on
(hyprtrixie-look workspaces on), each workspace shows the next one along,
workspaces shows each monitor's, and workspace shows one monitor's for the
workspace it just switched to.
`

func state() string {
	return env.ThemeState()
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
	return fsutil.Images(filepath.Join(state(), "theme", "backgrounds"), filepath.Join(env.AppConfig(), "backgrounds", name))
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
	monitors, _ := hypr.QuietMonitors()
	for _, monitor := range monitors {
		if monitor.ActiveWorkspace.ID == nil {
			continue
		}
		if !hypr.Wallpaper(monitor.Name, forWorkspace(found, *monitor.ActiveWorkspace.ID)) {
			return false
		}
	}
	return true
}

func forWorkspace(found []string, id int) string {
	offset := max(slices.Index(found, current()), 0)
	return found[(max(id, 1)-1+offset)%len(found)]
}

func oneWorkspace(monitor, workspace string) int {
	id, err := strconv.Atoi(workspace)
	if err != nil || !hypr.ValidMonitor(monitor) {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	if !inHyprland() || !perWorkspace() {
		return 0
	}
	found := backgrounds()
	if len(found) == 0 || hypr.Wallpaper(monitor, forWorkspace(found, id)) {
		return 0
	}
	return 1
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
	if !inHyprland() {
		return
	}
	if run.Within(run.Quick, "pkill", "-x", "hyprpaper") == 0 {
		run.Quiet(3*time.Second, "pidwait", "-x", "hyprpaper")
	}
	run.Quiet(run.Quick, "setsid", "-f", "hyprtrixie-start", "hyprpaper")
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
	if listed, err := filepath.Abs(argument); err == nil && slices.Contains(backgrounds(), listed) {
		return apply(listed)
	}
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

func Main() {
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
	case "workspace":
		os.Exit(oneWorkspace(env.Arg(2), env.Arg(3)))
	case "next":
		os.Exit(moveOn())
	default:
		os.Exit(set(command))
	}
}
