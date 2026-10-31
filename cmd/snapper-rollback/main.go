package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/snapshots"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

const (
	usage = `Usage: snapper-rollback [-c FILE] SNAPSHOT

Makes snapshot number SNAPSHOT the root filesystem from the next restart. The
root as it is now stays on the disk under a dated name. Nothing changes until
you type CONFIRM. It needs root.
`
	word       = "CONFIRM"
	folderMode = 0o755
	usageCode  = 2
)

type layout struct {
	mountpoint string
	main       string
	snapshots  string
	device     string
}

func readLayout(path string) (layout, error) {
	lines, err := fsutil.Lines(path)
	if err != nil {
		return layout{}, err
	}
	values := map[string]string{}
	section := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]")
		} else if key, value, ok := strings.Cut(line, "="); ok && section == "root" && !strings.ContainsAny(line[:1], "#;") {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	found := layout{values["mountpoint"], values["subvol_main"], values["subvol_snapshots"], values["dev"]}
	if found.mountpoint == "" || found.main == "" || found.snapshots == "" {
		return found, fmt.Errorf("%s needs mountpoint, subvol_main and subvol_snapshots under [root]", path)
	}
	return found, nil
}

func (top layout) mount() error {
	if err := os.MkdirAll(top.mountpoint, folderMode); err != nil {
		return err
	}
	if run.Quiet(run.Quick, "mountpoint", "-q", top.mountpoint) == 0 {
		return nil
	}
	args := []string{"-o", "subvolid=5"}
	if top.device != "" {
		args = append(args, top.device)
	}
	if run.Status("mount", append(args, top.mountpoint)...) != 0 {
		return fmt.Errorf("cannot mount the top of the disk at %s", top.mountpoint)
	}
	return nil
}

func (top layout) check(root, source, kept string) error {
	switch {
	case !fsutil.IsDir(root):
		return fmt.Errorf("%s is missing: is the disk mounted at %s with subvolid=5?", root, top.mountpoint)
	case !fsutil.IsDir(source):
		return fmt.Errorf("there is no such snapshot: %s is missing", source)
	case fsutil.Exists(kept):
		return fmt.Errorf("%s is already there: try again in a minute", kept)
	}
	return nil
}

func undo(root, kept string, copied bool) error {
	if copied && run.Status("btrfs", "subvolume", "delete", root) != 0 {
		return fmt.Errorf("the new root is half in place: delete %s and move %s back to it before restarting", root, kept)
	}
	if err := os.Rename(kept, root); err != nil {
		return fmt.Errorf("the old root is at %s: move it back to %s before restarting: %w", kept, root, err)
	}
	return errors.New("nothing changed")
}

func swap(root, kept, source string) error {
	if err := os.Rename(root, kept); err != nil {
		return err
	}
	if run.Status("btrfs", "subvolume", "snapshot", source, root) != 0 {
		return undo(root, kept, false)
	}
	if run.Status("btrfs", "subvolume", "set-default", root) != 0 {
		return undo(root, kept, true)
	}
	return nil
}

func goBack(top layout, number string) error {
	root := filepath.Join(top.mountpoint, top.main)
	source := filepath.Join(top.mountpoint, top.snapshots, number, "snapshot")
	kept := root + time.Now().Format("2006-01-02T15:04")
	if err := top.mount(); err != nil {
		return err
	}
	if err := top.check(root, source, kept); err != nil {
		return err
	}
	answer, err := term.ReadAnswer("Are you SURE you want to go back to snapshot " + number + "? Type '" + word + "' to continue: ")
	if err != nil || answer != word {
		return errors.New("nothing changed")
	}
	signal.Ignore(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	if err := swap(root, kept, source); err != nil {
		return err
	}
	fmt.Printf("The rollback is done. Restart to use snapshot %s. The old root stays at %s.\n", number, kept)
	return nil
}

func arguments() (string, string, bool) {
	args := os.Args[1:]
	config := "/etc/snapper-rollback.conf"
	if len(args) == 3 && args[0] == "-c" {
		config, args = args[1], args[2:]
	}
	if len(args) != 1 || !snapshots.IsNumber(args[0]) {
		return "", "", false
	}
	return config, args[0], true
}

func main() {
	if first := env.Arg(1); first == "-h" || first == "--help" {
		fmt.Print(usage)
		return
	}
	config, number, ok := arguments()
	if !ok {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(usageCode)
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "snapper-rollback needs root: run it with sudo")
		os.Exit(1)
	}
	top, err := readLayout(config)
	if err == nil {
		err = goBack(top, number)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
