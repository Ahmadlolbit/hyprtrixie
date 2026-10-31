package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const version = "0.1.0"

const usage = `hyprtrixie %s - Debian 13 with Hyprland, set up properly

USAGE
  hyprtrixie                       Open the setup menu
  hyprtrixie install [options]     Run the setup with the saved configuration
                                     --config FILE      use this configuration file
                                     --only 04,05       run only these modules
                                     --skip 10,11       leave these modules out
                                     --yes              ask nothing, use the configuration as is
                                     --on-failure MODE  ask | halt | continue
                                     --changes FILE     show these update notes in the summary
  hyprtrixie verify [--only ...]   Run every check again
  hyprtrixie modules               List the modules and how they last ended
  hyprtrixie config [show|path|edit|reset]
  hyprtrixie collect [--verbose]   Pack the logs, the report and system details for a bug report;
                                     --verbose also writes everything to ~/hyprtrixie-log-<time>.txt
  hyprtrixie verbose [on|off]      Log the desktop in detail from the next login (run it without sudo)
  hyprtrixie guide                 Read the guide in the terminal
  hyprtrixie update [PART]         Update everything (run it without sudo): a snapshot, Debian, the apps,
                                     then hyprtrixie and its installer; PART is system, apps or hyprtrixie
  hyprtrixie changes               Show what the last hyprtrixie update changed
  hyprtrixie doctor                Check the system and offer the safe fixes (run it with sudo)
  hyprtrixie app list              List the apps you can install, by category (* = installed)
  hyprtrixie app install|remove APP
                                   Install or remove one app and tick or untick it in the configuration
  hyprtrixie version
  hyprtrixie help

Configuration: %s
Logs:          %s/ and ~/runbook-logs/
`

var commands = map[string]func() int{
	"version":   showVersion,
	"--version": showVersion,
	"help":      showHelp,
	"-h":        showHelp,
	"--help":    showHelp,
	"verbose":   verbose,
	"changes":   changes,
	"update":    update,
	"config":    configCommand,
	"modules":   modulesCommand,
	"collect":   collect,
	"doctor":    doctor,
	"app":       appCommand,
}

func fail(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 1
}

func asRoot(mode string, command func() int) int {
	if os.Geteuid() != 0 {
		return fail("hyprtrixie needs root for this. Run it again with sudo: sudo %s %s", os.Args[0], mode)
	}
	defer cleanup.IgnoreQuit()()
	return command()
}

func say(text string) int {
	if _, err := os.Stdout.WriteString(text); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func remove(path string) int {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func showVersion() int {
	return say("hyprtrixie " + version + "\n")
}

func showHelp() int {
	return say(fmt.Sprintf(usage, version, config.Path(), env.LogDir()))
}

func root() string {
	return os.Getenv("HYPRTRIXIE_ROOT")
}

func script() string {
	return filepath.Join(root(), "hyprtrixie")
}

func backToBash() int {
	os.Setenv("HYPRTRIXIE_FROM_CORE", "1")
	return run.Report(run.Exec("bash", append([]string{script()}, os.Args[1:]...)...))
}

func main() {
	if root() == "" {
		os.Exit(run.Report(run.Exec("hyprtrixie", os.Args[1:]...)))
	}
	if command, ok := commands[env.Arg(1)]; ok {
		os.Exit(command())
	}
	os.Exit(backToBash())
}
