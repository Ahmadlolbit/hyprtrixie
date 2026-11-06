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
	"github.com/Ahmadlolbit/hyprtrixie/internal/lock"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `hyprtrixie %s - Debian 13 with Hyprland, set up properly

USAGE
  hyprtrixie                       Open the setup
  hyprtrixie install [options]     Run the setup with the saved configuration
                                     --config FILE      use this configuration file
                                     --only 04,05       run only these modules
                                     --skip 10,11       leave these modules out
                                     --yes              ask nothing, use the configuration as is
                                     --on-failure MODE  ask | halt | continue
                                     --changes FILE     show these update notes in the summary
  hyprtrixie verify [--only ...]   Run every check again
  hyprtrixie modules               List the modules and how they last ended
  hyprtrixie undo MODULE [--yes]   Take back what module 09, 10, 11 or 16 installed, wrote and turned on
                                     (run it with sudo); the setup then leaves it out until you run it again
  hyprtrixie config [show|path|edit|reset|export]
                                   export prints your settings as a preset to share, without your Debian mirror
  hyprtrixie config import FILE|URL
                                   Take a preset's settings from a file or an https address (run it with sudo);
                                     your Debian mirror stays, and it lists what changed
  hyprtrixie collect [--verbose]   Pack the logs, the report and system details for a bug report;
                                     --verbose also writes everything to ~/hyprtrixie-log-<time>.txt
  hyprtrixie verbose [on|off]      Log the desktop in detail from the next login (run it without sudo)
  hyprtrixie guide                 Read the guide in the terminal
  hyprtrixie keyboard [LAYOUT]     Show the keyboard layout, list them (list), or set one (run it with sudo)
  hyprtrixie battery-limit [N|off] Show or set where charging stops, 50 to 100 (run it with sudo to set it)
  hyprtrixie update [PART]         Update everything (run it without sudo): a snapshot, Debian, the apps,
                                     then Hyprtrixie and its installer; PART is system, apps or hyprtrixie
  hyprtrixie changes               Show what the last hyprtrixie update changed
  hyprtrixie restore [NUMBER]      List the restore points, or show what changed since one and go back to it
                                     (run it without sudo)
  hyprtrixie doctor                Check the system and offer the safe fixes (run it with sudo)
  hyprtrixie cleanup [--yes]       Delete the oldest automatic snapshots until / is under 85%% (run it with sudo)
  hyprtrixie app list              List the apps you can install, by category (* = installed)
  hyprtrixie app install|remove APP
                                   Install or remove one app and tick or untick it in the configuration
  hyprtrixie db [list]             List the databases you can start, and which are running
  hyprtrixie db start|stop|remove NAME
                                   Postgres, MySQL or Redis in a container on localhost (run it without sudo);
                                     stop keeps the data, remove deletes it
  hyprtrixie dotfiles [REPOSITORY] Apply your dotfiles with chezmoi, which sends what you change back there
                                     (run it without sudo; chezmoi runs that repository's scripts). Without one,
                                     show what chezmoi would change
  hyprtrixie fingerprint add|list|remove
                                   Add, list or remove your fingerprints (run it without sudo), once
                                     Fingerprint is on in the setup
  hyprtrixie tpm on|off|tidy|status
                                   Unlock the disk at boot with the TPM and a short PIN, the disk password kept
                                     (run it with sudo, in a terminal); moves the boot image to dracut, and tidy
                                     takes away the boot entry kept from before
  hyprtrixie webcam [relay]        List the IPU6 webcam's cameras, or relay one to /dev/video42 for apps
                                     that take only a normal webcam, until Ctrl+C (run it without sudo)
  hyprtrixie credits               Who made hyprtrixie, and the projects it builds on with their licences
  hyprtrixie version
  hyprtrixie help

Configuration: %s
Logs:          %s/ and ~/runbook-logs/
`

var commands = map[string]func() int{
	"version":       showVersion,
	"--version":     showVersion,
	"help":          showHelp,
	"-h":            showHelp,
	"--help":        showHelp,
	"verbose":       verbose,
	"changes":       changes,
	"update":        update,
	"config":        configCommand,
	"modules":       modulesCommand,
	"collect":       collect,
	"doctor":        doctor,
	"app":           appCommand,
	"db":            dbCommand,
	"dotfiles":      dotfilesCommand,
	"restore":       restore,
	"cleanup":       cleanupCommand,
	"install":       installCommand,
	"verify":        verifyCommand,
	"account":       accountCommand,
	"guide":         guide,
	"keyboard":      keyboardCommand,
	"battery-limit": batteryLimit,
	"module":        module,
	"undo":          undoCommand,
	"credits":       creditsCommand,
	"fingerprint":   fingerprintCommand,
	"tpm":           tpmCommand,
	"install-os":    installOSCommand,
	"first-start":   firstStartCommand,
	"webcam":        webcamCommand,
}

func fail(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	return 1
}

func asRoot(mode string, command func() int) int {
	if os.Geteuid() != 0 {
		return fail("Hyprtrixie needs root for this. Run it again with sudo: sudo %s %s", os.Args[0], mode)
	}
	defer cleanup.IgnoreQuit()()
	return command()
}

func alone(command func() int) func() int {
	return func() int {
		held, err := lock.Take()
		if err != nil {
			return fail("%v", err)
		}
		defer held.Close()
		return command()
	}
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
	return say("hyprtrixie " + env.Version + "\n")
}

func showHelp() int {
	return say(fmt.Sprintf(usage, env.Version, config.Path(), env.LogDir()))
}

func root() string {
	return os.Getenv("HYPRTRIXIE_ROOT")
}

func script() string {
	return filepath.Join(root(), "hyprtrixie")
}

func throughScript() int {
	return run.Report(run.Exec("bash", append([]string{script()}, os.Args[1:]...)...))
}

func main() {
	runTool()
	if root() == "" {
		os.Exit(run.Report(run.Exec("hyprtrixie", os.Args[1:]...)))
	}
	if command, ok := commands[env.Arg(1)]; ok {
		os.Exit(command())
	}
	os.Exit(fail("unknown command: %s (see: hyprtrixie help)", env.Arg(1)))
}
