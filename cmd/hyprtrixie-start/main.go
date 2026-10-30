package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `Usage: hyprtrixie-start bar|hypridle|hyprpaper|mako|elephant|walker

Starts a program of the desktop session. With verbose logging on
(hyprtrixie verbose on), it runs with its debug flags and logs to
~/.local/state/hyprtrixie/logs/<name>.log.
`

type program struct {
	command []string
	debug   []string
}

var programs = map[string]program{
	"bar":       {[]string{"qs", "-p", "/usr/share/hyprtrixie/shell"}, []string{"-vv", "--log-times"}},
	"hypridle":  {[]string{"hypridle"}, []string{"--verbose"}},
	"hyprpaper": {[]string{"hyprpaper"}, []string{"--verbose"}},
	"elephant":  {[]string{"elephant"}, []string{"--debug"}},
	"mako":      {[]string{"mako"}, nil},
	"walker":    {[]string{"walker", "--gapplication-service"}, nil},
}

func verbose() bool {
	info, err := os.Stat(filepath.Join(env.ConfigHome(), "hyprtrixie", "verbose"))
	return err == nil && info.Mode().IsRegular()
}

func logTo(path string, command []string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "==== %s %s\n", time.Now().Format(time.DateTime), strings.Join(command, " ")); err != nil {
		return err
	}
	for _, target := range []int{1, 2} {
		if err := syscall.Dup3(int(file.Fd()), target, 0); err != nil {
			return err
		}
	}
	return nil
}

func start(name string, chosen program) int {
	command := chosen.command
	if verbose() {
		logs := filepath.Join(env.StateHome(), "hyprtrixie", "logs")
		if err := os.MkdirAll(logs, 0o777); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		full := append(slices.Clone(command), chosen.debug...)
		if err := logTo(filepath.Join(logs, name+".log"), full); err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			command = full
		}
	}
	return run.Report(run.Exec(command[0], command[1:]...))
}

func main() {
	name := env.Arg(1)
	if name == "-h" || name == "--help" {
		fmt.Print(usage)
		return
	}
	chosen, known := programs[name]
	if !known {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if name == "bar" {
		for _, dir := range []string{env.ConfigHome(), env.CacheHome()} {
			if err := os.MkdirAll(filepath.Join(dir, "hyprtrixie"), 0o777); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
	os.Exit(start(name, chosen))
}
