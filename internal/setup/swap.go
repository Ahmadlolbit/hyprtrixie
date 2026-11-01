package setup

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/btrfs"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
)

const (
	swapMode      = 0o700
	swapFileMode  = "0600"
	swapPriority  = "10"
	bytesPerGiB   = 1 << 30
	zramUnit      = "zramswap.service"
	zramPriority  = "100"
	zramDevice    = "/dev/zram"
	swapSubvolume = "@swap"
	swappinessKey = "vm.swappiness"
	pageClusterID = "vm.page-cluster"
)

func swapDir() string {
	return env.Or("HYPRTRIXIE_SWAP_DIR", "/swap")
}

func swapFile() string {
	return filepath.Join(swapDir(), "swapfile")
}

func zramConfig() string {
	return env.Or("HYPRTRIXIE_ZRAM_CONFIG", "/etc/default/zramswap")
}

func swapSettings() string {
	return filepath.Join(env.Or("HYPRTRIXIE_SYSCTL_DIR", "/etc/sysctl.d"), "90-hyprtrixie-swap.conf")
}

func (context *Context) number(key string) int {
	value, _ := strconv.Atoi(context.Config.Get(key))
	return value
}

func swapsInUse() []string {
	output, _ := run.CaptureQuiet(run.Minute, "swapon", "--noheadings", "--show=NAME")
	return run.Lines(output)
}

func swapDirPrepare(log *runlog.Log) error {
	if btrfs.RootIsBtrfs() {
		if err := btrfs.MountSubvolume(log, swapSubvolume, swapDir()); err != nil {
			return err
		}
	} else if err := os.MkdirAll(swapDir(), programMode); err != nil {
		return err
	}
	return os.Chmod(swapDir(), swapMode)
}

func (context *Context) swapFileFits() bool {
	info, err := os.Stat(swapFile())
	return err == nil && info.Mode().IsRegular() && info.Size() == int64(context.number("SWAPFILE_GB"))*bytesPerGiB &&
		run.Quiet(run.Quick, "swaplabel", swapFile()) == 0
}

func swapOff(log *runlog.Log) error {
	if !slices.Contains(swapsInUse(), swapFile()) {
		return nil
	}
	return run.Logged(log, "swapoff", swapFile())
}

func (context *Context) swapFileCreate(log *runlog.Log) error {
	if context.swapFileFits() {
		return nil
	}
	if err := swapOff(log); err != nil {
		return err
	}
	if err := removeFiles(swapFile()); err != nil {
		return err
	}
	size := context.Config.Get("SWAPFILE_GB")
	if btrfs.RootIsBtrfs() {
		return run.Logged(log, "btrfs", "filesystem", "mkswapfile", "--size", size+"g", swapFile())
	}
	if err := run.Logged(log, "fallocate", "-l", size+"G", swapFile()); err != nil {
		return err
	}
	if err := run.Logged(log, "chmod", swapFileMode, swapFile()); err != nil {
		return err
	}
	return run.Logged(log, "mkswap", swapFile())
}

func swapFileEnable(log *runlog.Log) error {
	if err := btrfs.FstabEnsure(log, swapFile(), "none", "swap", "defaults,pri="+swapPriority); err != nil {
		return err
	}
	if slices.Contains(swapsInUse(), swapFile()) {
		return nil
	}
	return run.Logged(log, "swapon", "--priority", swapPriority, swapFile())
}

func swapFileDisable(log *runlog.Log) error {
	if err := swapOff(log); err != nil {
		return err
	}
	return btrfs.FstabDrop(swapFile(), "none")
}

type zramSetup struct {
	*Context
	restart followUp
}

func (zram *zramSetup) settings(log *runlog.Log) error {
	if err := apt.Install(log, "zram-tools"); err != nil {
		return err
	}
	before, _ := files.Hash(zramConfig())
	defer func() {
		after, _ := files.Hash(zramConfig())
		zram.restart.need(before != after)
	}()
	for _, setting := range [][2]string{{"ALGO", "zstd"}, {"PERCENT", zram.Config.Get("ZRAM_PERCENT")}, {"PRIORITY", zramPriority}} {
		if err := files.ConfSet(zramConfig(), setting[0], setting[1]); err != nil {
			return err
		}
	}
	return nil
}

func (zram *zramSetup) configure(log *runlog.Log) error {
	if zram.number("ZRAM_PERCENT") == 0 {
		return service.Disable(log, zramUnit)
	}
	if err := zram.settings(log); err != nil {
		return err
	}
	if err := service.Enable(log, zramUnit); err != nil {
		return err
	}
	zram.restart.owed = zram.restart.owed || !service.Active(zramUnit)
	return zram.restart.do(func() error { return service.Restart(log, zramUnit) })
}

func (context *Context) swappinessApply(log *runlog.Log) error {
	if err := context.Deploy.File("system/sysctl/90-hyprtrixie-swap.conf", swapSettings(), "SWAPPINESS="+context.Config.Get("SWAPPINESS")); err != nil {
		return err
	}
	return run.Logged(log, "sysctl", "-q", "-p", swapSettings())
}

func kernelValue(key string) string {
	output, _ := run.CaptureQuiet(run.Minute, "sysctl", "-n", key)
	return output
}

func (context *Context) swappinessSet(*runlog.Log) error {
	if value := kernelValue(swappinessKey); value != context.Config.Get("SWAPPINESS") {
		return errors.New(swappinessKey + " is " + value)
	}
	value := kernelValue(pageClusterID)
	return expect(value == "0", pageClusterID+" is "+value)
}

func swapFileOn(*runlog.Log) error {
	return holds(slices.Contains(swapsInUse(), swapFile()))
}

func swapFileNoCow(*runlog.Log) error {
	if !btrfs.RootIsBtrfs() {
		return nil
	}
	output, _ := run.CaptureQuiet(run.Minute, "lsattr", swapFile())
	attributes, _, _ := strings.Cut(output, " ")
	return holds(strings.Contains(attributes, "C"))
}

func zramOn(*runlog.Log) error {
	return holds(slices.ContainsFunc(swapsInUse(), func(name string) bool { return strings.HasPrefix(name, zramDevice) }))
}

func swapSteps(context *Context) {
	if context.number("SWAPFILE_GB") > 0 {
		context.Run.Step("Prepare a place for the swapfile", swapDirPrepare)
		context.Run.Step("Create a "+context.Config.Get("SWAPFILE_GB")+" GiB swapfile", context.swapFileCreate)
		context.Run.Step("Turn the swapfile on", swapFileEnable)
	} else {
		context.Run.Step("Turn the swapfile off", swapFileDisable)
	}
	zram := zramSetup{Context: context, restart: followUp{name: "zram-restart"}}
	context.Run.Step("Compress memory with zram", zram.configure)
	context.Run.Step("Tune swappiness", context.swappinessApply)
}

func swapChecks(context *Context) {
	if context.number("SWAPFILE_GB") > 0 {
		context.Run.Check("The swapfile is on", swapFileOn)
		context.Run.Check("The swapfile is not copy-on-write", swapFileNoCow)
	}
	context.Run.Check("Swappiness is "+context.Config.Get("SWAPPINESS"), context.swappinessSet)
	if context.number("ZRAM_PERCENT") > 0 {
		context.Run.Check("zram swap is on", zramOn)
	}
}
