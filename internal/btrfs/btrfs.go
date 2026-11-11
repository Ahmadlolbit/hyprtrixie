package btrfs

import (
	"os"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	folderMode = 0o755
	tableMode  = 0o644
	noTarget   = "none"
)

func table() string {
	return env.Or("HYPRTRIXIE_FSTAB", "/etc/fstab")
}

func top() string {
	return env.Or("HYPRTRIXIE_BTRFS_TOP", "/run/hyprtrixie/btrfs")
}

func rootField(name string) string {
	output, err := run.CaptureOnce(run.Minute, "findmnt", "-no", name, "/")
	if err != nil {
		return ""
	}
	return output
}

func RootType() string {
	return rootField("FSTYPE")
}

func RootIsBtrfs() bool {
	return RootType() == "btrfs"
}

func rootUUID() string {
	return rootField("UUID")
}

func RootDevice() string {
	device, _, _ := strings.Cut(rootField("SOURCE"), "[")
	return device
}

func RootSubvolume() string {
	for _, option := range strings.Split(rootField("OPTIONS"), ",") {
		if name, found := strings.CutPrefix(option, "subvol="); found {
			return strings.TrimPrefix(name, "/")
		}
	}
	return ""
}

func Mounted(path string) bool {
	return run.Quiet(run.Quick, "mountpoint", "-q", path) == 0
}

func mountTop(log *runlog.Log) (bool, error) {
	if Mounted(top()) {
		return false, nil
	}
	if err := os.MkdirAll(top(), folderMode); err != nil {
		return false, err
	}
	return true, run.Logged(log, "mount", "-o", "subvolid=5", RootDevice(), top())
}

func ensureSubvolume(log *runlog.Log, name string) error {
	mounted, err := mountTop(log)
	if err != nil {
		return err
	}
	if mounted {
		defer run.Quiet(run.Minute, "umount", top())
	}
	path := top() + "/" + name
	if fsutil.IsDir(path) {
		return nil
	}
	return run.Logged(log, "btrfs", "subvolume", "create", path)
}

func matches(line, spec, target string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
		return false
	}
	if target == noTarget {
		return fields[0] == spec
	}
	return len(fields) > 1 && fields[1] == target
}

func writeTable(lines []string) error {
	if err := atomic.Write(table(), []byte(strings.Join(lines, "\n")+"\n"), tableMode); err != nil {
		return err
	}
	run.Quiet(run.Minute, "systemctl", "daemon-reload")
	return nil
}

func entries(lines []string, spec, target string) []int {
	var found []int
	for index, line := range lines {
		if matches(line, spec, target) {
			found = append(found, index)
		}
	}
	return found
}

func listed(spec, target, kind, options string) bool {
	lines, err := fsutil.Lines(table())
	found := entries(lines, spec, target)
	return err == nil && len(found) == 1 && lines[found[0]] == spec+" "+target+" "+kind+" "+options+" 0 0"
}

func FstabEnsure(log *runlog.Log, spec, target, kind, options string) error {
	lines, err := fsutil.Lines(table())
	if err != nil {
		return err
	}
	wanted := spec + " " + target + " " + kind + " " + options + " 0 0"
	var kept []string
	found := 0
	for _, line := range lines {
		if !matches(line, spec, target) {
			kept = append(kept, line)
			continue
		}
		if found == 0 {
			kept = append(kept, wanted)
		}
		found++
	}
	if found == 1 && slices.Contains(lines, wanted) {
		return nil
	}
	if found == 0 {
		kept = append(kept, wanted)
	}
	log.Line("info", "fstab: "+wanted)
	return writeTable(kept)
}

func FstabDrop(spec, target string) error {
	lines, err := fsutil.Lines(table())
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(lines), func(line string) bool { return matches(line, spec, target) })
	if len(kept) == len(lines) {
		return nil
	}
	return writeTable(kept)
}

func MountSubvolume(log *runlog.Log, name, target string) error {
	uuid := rootUUID()
	if uuid == "" {
		return log.Failed("cannot find the root filesystem's UUID")
	}
	spec, options := "UUID="+uuid, "subvol="+name+",noatime"
	if fsutil.IsDir(target) && listed(spec, target, "btrfs", options) && Mounted(target) {
		return nil
	}
	if err := ensureSubvolume(log, name); err != nil {
		return err
	}
	if err := os.MkdirAll(target, folderMode); err != nil {
		return err
	}
	if err := FstabEnsure(log, spec, target, "btrfs", options); err != nil {
		return err
	}
	if Mounted(target) {
		return nil
	}
	return run.Logged(log, "mount", target)
}
