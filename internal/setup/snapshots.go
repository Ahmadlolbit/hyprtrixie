package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/btrfs"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fetch"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
)

const (
	firstSnapshot     = "hyprtrixie: before setup"
	snapperTemplate   = "/usr/share/snapper/config-templates/default"
	snapshotSubvolume = "@snapshots"
	installerRoot     = "@rootfs"
	snapshotsMode     = 0o750
	grubBtrfs         = "grub-btrfs"
	grubScript        = "/etc/grub.d/41_snapshots-btrfs"
	grubConfig        = "/boot/grub/grub-btrfs.cfg"
	grubDaemon        = "grub-btrfsd.service"
	rollbackTool      = "/usr/local/sbin/snapper-rollback"
	rollbackMount     = "/btrfsroot"
)

var (
	snapshotTimers  = []string{"snapper-timeline.timer", "snapper-cleanup.timer"}
	containerStores = [][2]string{{"/var/lib/docker", "@docker"}, {"/var/lib/containers", "@containers"}}
)

type grubMenu struct {
	fresh bool
}

func snapperDefaults() string {
	return env.Or("HYPRTRIXIE_SNAPPER_DEFAULTS", "/etc/default/snapper")
}

func snapperConfig() string {
	return env.Or("HYPRTRIXIE_SNAPPER_CONFIG", "/etc/snapper/configs/root")
}

func snapshotDir() string {
	return env.Or("HYPRTRIXIE_SNAPSHOT_DIR", "/.snapshots")
}

func snapperArgs(args ...string) []string {
	return append([]string{"--no-dbus", "-c", "root"}, args...)
}

func empty(dir string) bool {
	entries, _ := os.ReadDir(dir)
	return len(entries) == 0
}

func holdsSnapshots(options string) bool {
	fields := strings.Split(options, ",")
	return slices.Contains(fields, "subvol="+snapshotSubvolume) || slices.Contains(fields, "subvol=/"+snapshotSubvolume)
}

func snapshotDirClear(log *runlog.Log) error {
	switch {
	case !fsutil.IsDir(snapshotDir()):
		return nil
	case !empty(snapshotDir()):
		return fmt.Errorf("%s already holds snapshots inside the root subvolume; move or delete them first", snapshotDir())
	case run.Quiet(run.Minute, "btrfs", "subvolume", "show", snapshotDir()) == 0:
		return run.Logged(log, "btrfs", "subvolume", "delete", snapshotDir())
	}
	return os.Remove(snapshotDir())
}

func snapshotsMount(log *runlog.Log) error {
	options, _ := run.CaptureQuiet(run.Minute, "findmnt", "-no", "OPTIONS", snapshotDir())
	if holdsSnapshots(options) {
		return nil
	}
	if err := snapshotDirClear(log); err != nil {
		return err
	}
	if err := btrfs.MountSubvolume(log, snapshotSubvolume, snapshotDir()); err != nil {
		return err
	}
	return os.Chmod(snapshotDir(), snapshotsMode)
}

func snapperConfigs() string {
	lines, _ := fsutil.Lines(snapperDefaults())
	configs := ""
	for _, line := range lines {
		if rest, ok := strings.CutPrefix(line, `SNAPPER_CONFIGS="`); ok && strings.HasSuffix(rest, `"`) {
			configs = strings.TrimSuffix(rest, `"`)
		}
	}
	return configs
}

func snapperRegister() error {
	configs := snapperConfigs()
	if slices.Contains(strings.Fields(configs), "root") {
		return nil
	}
	if configs != "" {
		configs += " "
	}
	return files.ConfSet(snapperDefaults(), "SNAPPER_CONFIGS", `"`+configs+`root"`)
}

func (context *Context) snapperSettings() [][2]string {
	return [][2]string{
		{"SUBVOLUME", "/"}, {"FSTYPE", "btrfs"}, {"TIMELINE_CREATE", "yes"}, {"TIMELINE_CLEANUP", "yes"}, {"TIMELINE_LIMIT_HOURLY", "0"},
		{"TIMELINE_LIMIT_DAILY", context.Config.Get("SNAPSHOT_DAILY")}, {"TIMELINE_LIMIT_WEEKLY", context.Config.Get("SNAPSHOT_WEEKLY")},
		{"TIMELINE_LIMIT_MONTHLY", context.Config.Get("SNAPSHOT_MONTHLY")}, {"TIMELINE_LIMIT_YEARLY", "0"}, {"NUMBER_CLEANUP", "yes"},
		{"NUMBER_LIMIT", "10"}, {"NUMBER_LIMIT_IMPORTANT", "5"}, {"ALLOW_GROUPS", "sudo"}, {"SYNC_ACL", "yes"},
	}
}

func (context *Context) snapperConfigure(log *runlog.Log) error {
	if !fsutil.IsFile(snapperConfig()) {
		if err := run.Logged(log, "install", "-D", "-m", "0640", snapperTemplate, snapperConfig()); err != nil {
			return err
		}
	}
	if err := snapperRegister(); err != nil {
		return err
	}
	for _, setting := range context.snapperSettings() {
		if err := files.ConfSet(snapperConfig(), setting[0], `"`+setting[1]+`"`); err != nil {
			return err
		}
	}
	return nil
}

func snapperFirst(log *runlog.Log) error {
	listing, _ := run.CaptureQuiet(run.Minute, "snapper", snapperArgs("list", "--columns", "description")...)
	if strings.Contains(listing, firstSnapshot) {
		return nil
	}
	return run.Logged(log, "snapper", snapperArgs("create", "--description", firstSnapshot, "--cleanup-algorithm", "number")...)
}

func snapperReady(*runlog.Log) error {
	configs, _ := run.CaptureQuiet(run.Minute, "snapper", "--no-dbus", "list-configs")
	if !lineHasPrefix(configs, "root ") {
		return errors.New("snapper has no config for /")
	}
	return quietly("snapper can't list the snapshots of /", "snapper", snapperArgs("list")...)
}

func timersActive(*runlog.Log) error {
	for _, timer := range snapshotTimers {
		if !service.Active(timer) {
			return errors.New(timer + " is not active")
		}
	}
	return nil
}

func mountedAt(dir string) bool {
	return run.Quiet(run.Minute, "mountpoint", "-q", dir) == 0
}

func containerStorage(log *runlog.Log) error {
	for _, store := range containerStores {
		dir, subvolume := store[0], store[1]
		if mountedAt(dir) {
			continue
		}
		if !empty(dir) {
			fmt.Fprintf(log, "%s already has data; it stays inside the root subvolume\n", dir)
			continue
		}
		if err := btrfs.MountSubvolume(log, subvolume, dir); err != nil {
			return err
		}
	}
	return nil
}

func containerStorageOK(*runlog.Log) error {
	for _, store := range containerStores {
		if !mountedAt(store[0]) && empty(store[0]) {
			return errors.New(store[0] + " is neither its own subvolume nor in use")
		}
	}
	return nil
}

func grubBuild(log *runlog.Log, source string) error {
	folder, err := os.MkdirTemp("", "hyprtrixie")
	if err != nil {
		return err
	}
	defer os.RemoveAll(folder)
	if err := fsutil.CopyTree(source, folder); err != nil {
		return err
	}
	return run.Logged(log, "make", "-C", folder, "install", "GRUB_UPDATE_EXCLUDE=true")
}

func (menu *grubMenu) install(log *runlog.Log) error {
	menu.fresh = false
	if err := apt.InstallLists(log, grubBtrfs); err != nil {
		return err
	}
	folder, err := fetch.GitCached(log, grubBtrfs)
	if err != nil {
		return err
	}
	pinned, err := fetch.GetSource(log, grubBtrfs)
	if err != nil {
		return err
	}
	stamp := filepath.Join(env.StateDir(), "grub-btrfs.commit")
	built, _ := fsutil.Text(stamp)
	if fsutil.Runnable(grubScript) && strings.TrimSpace(built) == pinned.Commit {
		return nil
	}
	if err := grubBuild(log, folder); err != nil {
		return err
	}
	if err := os.MkdirAll(env.StateDir(), programMode); err != nil {
		return err
	}
	if err := atomic.Write(stamp, []byte(pinned.Commit+"\n"), plainMode); err != nil {
		return err
	}
	menu.fresh = true
	return nil
}

func (menu *grubMenu) enable(log *runlog.Log) error {
	if err := service.Start(log, grubDaemon); err != nil {
		return err
	}
	if !menu.fresh && fsutil.IsFile(grubConfig) {
		return nil
	}
	return run.Logged(log, "update-grub")
}

func grubReady(*runlog.Log) error {
	switch {
	case !fsutil.Runnable(grubScript):
		return errors.New(grubScript + " is missing")
	case !service.Active(grubDaemon):
		return errors.New(grubDaemon + " is not active")
	}
	return nil
}

func (context *Context) rollbackInstall(*runlog.Log) error {
	if err := removeFiles(rollbackTool); err != nil {
		return err
	}
	if err := context.Deploy.Program(filepath.Join(env.ToolsDir(), "snapper-rollback"), rollbackTool); err != nil {
		return err
	}
	if err := context.Deploy.File("system/snapper-rollback.conf", "/etc/snapper-rollback.conf", "SUBVOL="+btrfs.RootSubvolume(), "DEVICE="+btrfs.RootDevice()); err != nil {
		return err
	}
	return os.MkdirAll(rollbackMount, programMode)
}

func rollbackReady(*runlog.Log) error {
	return holds(run.Quiet(run.Minute, rollbackTool, "--help") == 0)
}

func snapshotSteps(context *Context) {
	switch {
	case !btrfs.RootIsBtrfs():
		context.Run.SkipModule("Snapshots need a btrfs root; this system uses " + btrfs.RootType())
		return
	case context.Config.Get("SNAPSHOTS") != "yes":
		context.Run.SkipModule("Snapshots are turned off in the configuration")
		return
	}
	if btrfs.RootSubvolume() == installerRoot {
		context.Run.Note("Root is the installer's " + installerRoot + "; /home is inside root snapshots.")
	}
	menu := grubMenu{}
	context.Run.Step("Install snapper", packages("snapshots"))
	context.Run.Step("Mount the snapshots subvolume", snapshotsMount)
	context.Run.Step("Configure snapper", context.snapperConfigure)
	context.Run.Step("Turn on the snapshot timers", func(log *runlog.Log) error { return service.Start(log, snapshotTimers...) })
	context.Run.Step("Take the first snapshot", snapperFirst)
	context.Run.Step("Keep container storage out of snapshots", containerStorage)
	context.Run.Step("Install grub-btrfs", menu.install)
	context.Run.Step("Show snapshots in the boot menu", menu.enable)
	context.Run.Step("Install snapper-rollback", context.rollbackInstall)
}

func snapshotChecks(context *Context) {
	if !btrfs.RootIsBtrfs() || context.Config.Get("SNAPSHOTS") != "yes" {
		return
	}
	context.Run.Check("Container storage is outside root snapshots", containerStorageOK)
	context.Run.Check("Snapshots appear in the boot menu", grubReady)
	context.Run.Check("snapper-rollback works", rollbackReady)
	context.Run.Check("Snapper is set up for /", snapperReady)
	context.Run.Check("Snapshot timers are running", timersActive)
}
