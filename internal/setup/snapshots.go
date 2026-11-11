package setup

import (
	"errors"
	"fmt"
	"io/fs"
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
	"github.com/Ahmadlolbit/hyprtrixie/internal/ledger"
	"github.com/Ahmadlolbit/hyprtrixie/internal/modules"
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
	aptSnapshots      = "DISABLE_APT_SNAPSHOT"
	bootSnapshots     = "snapper-boot.timer"
	snapshotsModule   = "05"
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
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return true
	}
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 0
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

func firstTaken() string {
	return filepath.Join(env.StateDir(), "first-snapshot")
}

func firstTakenBefore() bool {
	status, _ := modules.StatusOf(fsutil.Optional(modules.StatusFile()), "05")
	return fsutil.Exists(firstTaken()) || status == "PASS"
}

func firstIn(listing string) (string, string, bool) {
	for _, line := range run.Lines(listing) {
		if fields := strings.SplitN(line, "\t", 3); len(fields) == 3 && fields[2] == firstSnapshot {
			return fields[0], fields[1], true
		}
	}
	return "", "", false
}

func snapperFirst(log *runlog.Log) error {
	listing, err := run.CaptureQuiet(run.Minute, "snapper", snapperArgs("--csvout", "--no-headers", "--separator", "\t", "list", "--columns", "number,cleanup,description")...)
	if err != nil {
		return errors.New("snapper can't list the snapshots of /")
	}
	number, cleanup, found := firstIn(listing)
	switch {
	case found && cleanup != "":
		err = run.Logged(log, "snapper", snapperArgs("modify", "--cleanup-algorithm", "", number)...)
	case !found && firstTakenBefore():
		log.Line("info", "the first snapshot, "+firstSnapshot+", was deleted; it isn't taken again, since it would show this finished system")
	case !found:
		err = run.Logged(log, "snapper", snapperArgs("create", "--description", firstSnapshot)...)
	}
	if err != nil || fsutil.Exists(firstTaken()) {
		return err
	}
	if err := os.MkdirAll(env.StateDir(), programMode); err != nil {
		return err
	}
	return atomic.Write(firstTaken(), nil, plainMode)
}

func snapperReady(*runlog.Log) error {
	configs, _ := run.CaptureQuiet(run.Minute, "snapper", "--no-dbus", "list-configs")
	if !lineHasPrefix(configs, "root ") {
		return errors.New("snapper has no config for /")
	}
	return quietly("snapper can't list the snapshots of /", "snapper", snapperArgs("list")...)
}

func snapshotsStopped() string {
	return filepath.Join(env.StateDir(), "snapshots-stopped")
}

func everySnapshotTimer() []string {
	return append(slices.Clone(snapshotTimers), bootSnapshots)
}

func aptSnapshotsOff() bool {
	return strings.Trim(settingIn(snapperDefaults(), aptSnapshots), `"'`) == "yes"
}

func anyTimerOn(timers []string) bool {
	for _, state := range []string{"is-enabled", "is-active"} {
		if run.Quiet(run.Minute, "systemctl", append([]string{state, "--quiet"}, timers...)...) == 0 {
			return true
		}
	}
	return false
}

func snapshotsTurnOn(log *runlog.Log) error {
	stopped, _ := fsutil.Lines(snapshotsStopped())
	timers := slices.Clone(snapshotTimers)
	if slices.Contains(stopped, bootSnapshots) {
		timers = append(timers, bootSnapshots)
	}
	if err := service.Start(log, timers...); err != nil {
		return err
	}
	if slices.Contains(stopped, aptSnapshots) {
		if err := files.ConfSet(snapperDefaults(), aptSnapshots, `"no"`); err != nil {
			return err
		}
	}
	return removeFiles(snapshotsStopped())
}

func snapshotsStillTaken() bool {
	setUp := slices.Contains(ledger.Load(snapshotsModule).Values(ledger.Wants), "snapper")
	return setUp && fsutil.IsFile(snapperDefaults()) && (!aptSnapshotsOff() || anyTimerOn(everySnapshotTimer()))
}

func noteStopped(names []string) error {
	kept, _ := fsutil.Lines(snapshotsStopped())
	for _, name := range names {
		if !slices.Contains(kept, name) {
			kept = append(kept, name)
		}
	}
	if err := os.MkdirAll(env.StateDir(), programMode); err != nil {
		return err
	}
	return atomic.Write(snapshotsStopped(), []byte(strings.Join(kept, "\n")+"\n"), plainMode)
}

func snapshotsStop(log *runlog.Log) error {
	timers := slices.DeleteFunc(everySnapshotTimer(), func(timer string) bool { return !service.Enabled(timer) && !service.Active(timer) })
	changed := slices.Clone(timers)
	if !aptSnapshotsOff() {
		changed = append(changed, aptSnapshots)
	}
	if err := noteStopped(changed); err != nil {
		return err
	}
	if err := service.Disable(log, timers...); err != nil {
		return err
	}
	if slices.Contains(changed, aptSnapshots) {
		if err := files.ConfSet(snapperDefaults(), aptSnapshots, `"yes"`); err != nil {
			return err
		}
	}
	log.Line("info", "no new snapshots are taken from now on; the ones taken so far stay: sudo snapper delete removes those you no longer want, and sudo hyprtrixie cleanup removes old automatic ones when / is nearly full")
	return nil
}

func snapshotsOff(context *Context) {
	if !snapshotsStillTaken() {
		context.Run.SkipModule("Snapshots are turned off in the configuration")
		return
	}
	context.Run.Step("Stop taking new snapshots", snapshotsStop)
}

func timersActive(*runlog.Log) error {
	for _, timer := range snapshotTimers {
		if !service.Active(timer) {
			return errors.New(timer + " is not active")
		}
	}
	return nil
}

func containerStorage(log *runlog.Log) error {
	for _, store := range containerStores {
		dir, subvolume := store[0], store[1]
		if btrfs.Mounted(dir) {
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
		if !btrfs.Mounted(store[0]) && empty(store[0]) {
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
	pinned, err := fetch.GetSource(log, grubBtrfs)
	if err != nil {
		return err
	}
	stamp := filepath.Join(env.StateDir(), "grub-btrfs.commit")
	built, _ := fsutil.Text(stamp)
	if fsutil.Runnable(grubScript) && strings.TrimSpace(built) == pinned.Commit {
		return nil
	}
	folder, err := fetch.GitCached(log, grubBtrfs)
	if err != nil {
		return err
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
	if err := context.Deploy.ToolAt("snapper-rollback", rollbackTool); err != nil {
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
		snapshotsOff(context)
		return
	}
	if btrfs.RootSubvolume() == installerRoot && !btrfs.Mounted("/home") {
		context.Run.Note("Root is the installer's " + installerRoot + "; /home is inside root snapshots.")
	}
	menu := grubMenu{}
	context.Run.Step("Install snapper", packages("snapshots"))
	context.Run.Step("Mount the snapshots subvolume", snapshotsMount)
	context.Run.Step("Configure snapper", context.snapperConfigure)
	context.Run.Step("Turn on the snapshot timers", snapshotsTurnOn)
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
