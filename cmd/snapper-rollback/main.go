package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
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
root as it is now stays on the disk under a dated name. Nothing on the disk
changes until you type CONFIRM. It needs root.
`
	confirmWord = "CONFIRM"
	lockFile    = "/run/snapper-rollback.lock"
	lockMode    = 0o600
	folderMode  = 0o755
	usageCode   = 2
	fstabPath   = 1
	fstabFlags  = 3
)

var unstoppable = []os.Signal{
	syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGPIPE, syscall.SIGTSTP, syscall.SIGTTIN, syscall.SIGTTOU,
}

type layout struct {
	mountpoint string
	main       string
	snapshots  string
	device     string
}

type rootPaths struct {
	root   string
	source string
	kept   string
}

func readLayout(path string) (layout, error) {
	if !fsutil.IsFile(path) {
		return layout{}, errors.New(path + " is missing")
	}
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
		} else if key, value, ok := strings.Cut(line, "="); ok && section == "root" {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	found := layout{values["mountpoint"], values["subvol_main"], values["subvol_snapshots"], values["dev"]}
	if found.mountpoint == "" || found.main == "" || found.snapshots == "" {
		return found, fmt.Errorf("%s needs mountpoint, subvol_main and subvol_snapshots under [root]", path)
	}
	return found, nil
}

func pathsFor(top layout, number string, now time.Time) rootPaths {
	root := filepath.Join(top.mountpoint, top.main)
	return rootPaths{
		root:   root,
		source: filepath.Join(top.mountpoint, top.snapshots, number, "snapshot"),
		kept:   root + now.Format("2006-01-02T15:04"),
	}
}

func (top layout) mount() (bool, error) {
	if err := os.MkdirAll(top.mountpoint, folderMode); err != nil {
		return false, err
	}
	if run.Quiet(run.Quick, "mountpoint", "-q", top.mountpoint) == 0 {
		return false, nil
	}
	args := []string{"-o", "subvolid=5"}
	if top.device != "" {
		args = append(args, top.device)
	}
	if run.Within(run.Minute, "mount", append(args, top.mountpoint)...) != 0 {
		return false, fmt.Errorf("cannot mount the top of the disk at %s", top.mountpoint)
	}
	return true, nil
}

func (top layout) unmount() {
	if run.Quiet(run.Minute, "umount", top.mountpoint) != 0 {
		fmt.Fprintln(os.Stderr, top.mountpoint, "stays mounted until the next restart")
	}
}

func (top layout) whileMounted(work func() error) error {
	mounted, err := top.mount()
	if err != nil {
		return err
	}
	if mounted {
		defer top.unmount()
	}
	return work()
}

func lock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, lockMode)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("another rollback is running")
	}
	return file, nil
}

func diskID(path string) string {
	output, err := run.CaptureQuiet(run.Quick, "findmnt", "-no", "UUID", path)
	if err != nil {
		return ""
	}
	return output
}

func sameDisk(mountpoint string) bool {
	here, root := diskID(mountpoint), diskID("/")
	return here == "" || root == "" || here == root
}

func bootsByNumber(table, commandLine string) bool {
	for _, word := range strings.Fields(commandLine) {
		if strings.HasPrefix(word, "rootflags=") && strings.Contains(word, "subvolid=") {
			return true
		}
	}
	return slices.ContainsFunc(strings.Split(table, "\n"), func(line string) bool {
		fields := strings.Fields(line)
		return len(fields) > fstabFlags && !strings.HasPrefix(fields[0], "#") && fields[fstabPath] == "/" && strings.Contains(fields[fstabFlags], "subvolid=")
	})
}

func startsByNumber() bool {
	return bootsByNumber(fsutil.Optional(env.Or("HYPRTRIXIE_FSTAB", "/etc/fstab")), fsutil.Optional(env.Or("HYPRTRIXIE_CMDLINE", "/proc/cmdline")))
}

func (top layout) check(paths rootPaths) error {
	switch {
	case !sameDisk(top.mountpoint):
		return fmt.Errorf("%s holds a different disk from the root: check what's mounted there and the dev line in the config", top.mountpoint)
	case startsByNumber():
		return errors.New("this machine starts its root by subvolume number, so a new root wouldn't be used: change it to subvol= first")
	case !fsutil.IsDir(paths.root):
		return fmt.Errorf("%s is missing: is the disk mounted at %s with subvolid=5?", paths.root, top.mountpoint)
	case !fsutil.IsDir(paths.source):
		return fmt.Errorf("there is no such snapshot: %s is missing", paths.source)
	case fsutil.Exists(paths.kept):
		return fmt.Errorf("%s is already there: try again in a minute", paths.kept)
	}
	return nil
}

func nested(root string) []string {
	output, _ := run.CaptureQuiet(run.Minute, "btrfs", "subvolume", "list", "-o", root)
	var found []string
	for _, line := range run.Lines(output) {
		if _, path, ok := strings.Cut(line, " path "); ok {
			found = append(found, path)
		}
	}
	return found
}

func step(args ...string) bool {
	return run.Within(run.Slow, args[0], args[1:]...) == 0
}

func (paths rootPaths) discard() error {
	if fsutil.Exists(paths.kept) && !step("btrfs", "-q", "subvolume", "delete", paths.kept) {
		return fmt.Errorf("the root didn't change, but the unused copy %s stays on the disk", filepath.Base(paths.kept))
	}
	return errors.New("nothing changed")
}

func (paths rootPaths) stillHolds(root os.FileInfo) bool {
	current, err := os.Stat(paths.root)
	return err == nil && os.SameFile(root, current)
}

func (paths rootPaths) swap() error {
	root, err := os.Stat(paths.root)
	if err != nil {
		return errors.New("nothing changed")
	}
	if !step("btrfs", "-q", "subvolume", "snapshot", paths.source, paths.kept) {
		return paths.discard()
	}
	step("mv", "--exchange", "--no-copy", "-T", paths.kept, paths.root)
	if paths.stillHolds(root) {
		return paths.discard()
	}
	if !step("btrfs", "-q", "subvolume", "set-default", paths.root) {
		fmt.Println("The default subvolume didn't change, so a machine that starts without subvol= would still use the old root.")
	}
	return nil
}

func (top layout) ready(paths rootPaths) error {
	if err := top.check(paths); err != nil {
		return err
	}
	if inside := nested(paths.root); len(inside) > 0 {
		fmt.Println("These stay with the old root, because a snapshot doesn't hold them:", strings.Join(inside, ", "))
	}
	return nil
}

func (top layout) replace(paths rootPaths) error {
	if err := top.check(paths); err != nil {
		return err
	}
	return paths.swap()
}

func goBack(top layout, number string) error {
	paths := pathsFor(top, number, time.Now())
	if err := top.whileMounted(func() error { return top.ready(paths) }); err != nil {
		return err
	}
	answer, err := term.ReadAnswer("Are you SURE you want to go back to snapshot " + number + "? Type '" + confirmWord + "' to continue: ")
	if err != nil || answer != confirmWord {
		return errors.New("nothing changed")
	}
	signal.Ignore(unstoppable...)
	err = top.whileMounted(func() error { return top.replace(paths) })
	syscall.Sync()
	if err != nil {
		return err
	}
	fmt.Printf("The rollback is done. What was %s stays on the disk as %s.\n", top.main, filepath.Base(paths.kept))
	fmt.Printf("Restart now to use snapshot %s. Until then, anything that updates the boot menu, like a new kernel, would point it at the old root.\n", number)
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

func rollBack(config, number string) error {
	top, err := readLayout(config)
	if err != nil {
		return err
	}
	held, err := lock(lockFile)
	if err != nil {
		return err
	}
	defer held.Close()
	return goBack(top, number)
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
	if err := rollBack(config, number); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
