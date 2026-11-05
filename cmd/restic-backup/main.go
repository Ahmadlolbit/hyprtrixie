package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `Usage: restic-backup init <location>       set up where backups go (a folder, a USB drive path, s3:, b2: or rclone:)
       restic-backup [run]                 back up your home folder
       restic-backup list                  show the backups
       restic-backup restore <id> <folder> restore a backup into a folder
       restic-backup mount <folder>        browse the backups as files
       restic-backup check                 verify the backup data
       restic-backup prune                 keep 7 daily, 4 weekly and 6 monthly backups, drop the rest
       restic-backup usb on|off            back up by itself whenever the backup drive is plugged in
       restic-backup keys                  type the keys for an s3: or b2: location again

The location is remembered in ~/.config/restic-backup/repository and the
password in ~/.config/restic-backup/password. Keep a copy of the password
somewhere safe: without it the backups cannot be read. Keys for cloud locations
are in ~/.config/restic-backup/env, which only you can read. Extra patterns to
leave out go in ~/.config/restic-backup/excludes, one per line.
`

const (
	passwordBytes = 32
	probeLimit    = 2 * time.Minute
)

func configDir() string {
	return filepath.Join(env.ConfigHome(), "restic-backup")
}

func repositoryFile() string {
	return filepath.Join(configDir(), "repository")
}

func passwordFile() string {
	return filepath.Join(configDir(), "password")
}

func hasContent(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

func useRepository() bool {
	if !hasContent(repositoryFile()) {
		fmt.Fprintln(os.Stderr, "No backup location yet. Run: restic-backup init <location>")
		return false
	}
	location, err := fsutil.Text(repositoryFile())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if err := os.Setenv("RESTIC_REPOSITORY", location); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	loadKeys()
	return true
}

func newPassword() error {
	secret := make([]byte, passwordBytes)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	return atomic.Install(passwordFile(), []byte(base64.StdEncoding.EncodeToString(secret)+"\n"), 0o600)
}

func prepare() error {
	if err := os.MkdirAll(configDir(), 0o777); err != nil {
		return err
	}
	if err := os.Chmod(configDir(), 0o700); err != nil {
		return err
	}
	if hasContent(passwordFile()) {
		return nil
	}
	if err := newPassword(); err != nil {
		return err
	}
	fmt.Printf("A new password is in %s. Keep a copy somewhere safe.\n", passwordFile())
	return nil
}

func initialize(args []string) int {
	if len(args) == 0 || args[0] == "" {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !keysReady(args[0]) {
		return 1
	}
	if err := atomic.Write(repositoryFile(), []byte(args[0]+"\n"), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !useRepository() {
		return 1
	}
	if run.Quiet(probeLimit, "restic", "cat", "config") == 0 {
		return 0
	}
	return run.Status("restic", "init")
}

func backup() int {
	home := env.Home()
	args := []string{"backup", home, "--exclude-caches", "--exclude", home + "/.cache", "--exclude", home + "/.local/share/Trash",
		"--exclude", home + "/.local/share/containers", "--exclude", "node_modules"}
	if !useRepository() {
		return 1
	}
	excludes := filepath.Join(configDir(), "excludes")
	if fsutil.IsFile(excludes) {
		args = append(args, "--exclude-file", excludes)
	}
	return run.Status("restic", args...)
}

func repository(args ...string) int {
	if !useRepository() {
		return 1
	}
	return run.Status("restic", args...)
}

func restore(args []string) int {
	if len(args) != 2 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	return repository("restore", args[0], "--target", args[1])
}

func mount(args []string) int {
	if len(args) != 1 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	return repository("mount", args[0])
}

func commands() map[string]func([]string) int {
	return map[string]func([]string) int{
		"init":    initialize,
		"run":     func([]string) int { return backup() },
		"list":    func([]string) int { return repository("snapshots") },
		"restore": restore,
		"mount":   mount,
		"check":   func([]string) int { return repository("check") },
		"prune": func([]string) int {
			return repository("forget", "--keep-daily", "7", "--keep-weekly", "4", "--keep-monthly", "6", "--prune")
		},
		"usb":  usb,
		"keys": func([]string) int { return keys() },
	}
}

func main() {
	if err := os.Setenv("RESTIC_PASSWORD_FILE", passwordFile()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	name := env.Arg(1)
	if name == "" {
		name = "run"
	}
	if name == "-h" || name == "--help" || name == "help" {
		fmt.Print(usage)
		return
	}
	command, known := commands()[name]
	if !known {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	os.Exit(command(os.Args[min(2, len(os.Args)):]))
}
