package setup

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/btrfs"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	mibPerGiB       = 1024
	extentFields    = 4
	offsetField     = 3
	resumeFollowUp  = "resume-boot-files"
	hibernateOption = "HIBERNATE"
)

func resumeGrub() string {
	return env.Or("HYPRTRIXIE_RESUME_GRUB", "/etc/default/grub.d/hyprtrixie-resume.cfg")
}

func resumeInitramfs() string {
	return env.Or("HYPRTRIXIE_RESUME_INITRAMFS", "/etc/initramfs-tools/conf.d/hyprtrixie-resume")
}

func (context *Context) hibernates() bool {
	return context.Config.Get(hibernateOption) == "yes" && context.number("SWAPFILE_GB") > 0 &&
		detect.SecureBoot() != "enabled" && apt.Commands("update-grub") == nil
}

func memoryGiB() int {
	return (detect.MemoryMiB() + mibPerGiB - 1) / mibPerGiB
}

func (context *Context) swapGiB() int {
	if context.hibernates() {
		return max(context.number("SWAPFILE_GB"), memoryGiB())
	}
	return context.number("SWAPFILE_GB")
}

func firstExtent(report string) string {
	for _, line := range strings.Split(report, "\n") {
		fields := strings.Fields(line)
		if len(fields) > extentFields && fields[0] == "0:" {
			return strings.Trim(fields[offsetField], ".:")
		}
	}
	return ""
}

func resumeOffset() (string, error) {
	var output string
	var err error
	if btrfs.RootIsBtrfs() {
		output, err = run.CaptureQuiet(run.Minute, "btrfs", "inspect-internal", "map-swapfile", "-r", swapFile())
	} else {
		output, err = run.CaptureQuiet(run.Minute, "filefrag", "-v", swapFile())
		output = firstExtent(output)
	}
	offset := strings.TrimSpace(output)
	if _, parsed := strconv.Atoi(offset); err != nil || parsed != nil {
		return "", errors.New("can't find where the swapfile starts on the disk")
	}
	return offset, nil
}

func resumeDevice() (string, error) {
	uuid, err := run.CaptureQuiet(run.Minute, "findmnt", "--noheadings", "--output", "UUID", "--target", swapFile())
	uuid = strings.TrimSpace(uuid)
	if err != nil || uuid == "" {
		return "", errors.New("can't find the disk the swapfile is on")
	}
	return uuid, nil
}

func resumeFiles(uuid, offset string) (string, string) {
	grub := fmt.Sprintf("GRUB_CMDLINE_LINUX_DEFAULT=\"$GRUB_CMDLINE_LINUX_DEFAULT resume=UUID=%s resume_offset=%s\"\n", uuid, offset)
	return grub, "RESUME=UUID=" + uuid + "\n"
}

type hibernation struct {
	*Context
	boot followUp
}

func (rest *hibernation) rebuild(log *runlog.Log) error {
	return rest.boot.do(func() error {
		if err := run.Logged(log, "update-initramfs", "-u"); err != nil {
			return err
		}
		if apt.Commands("update-grub") != nil {
			return nil
		}
		return run.Logged(log, "update-grub")
	})
}

func (rest *hibernation) setup(log *runlog.Log) error {
	offset, err := resumeOffset()
	if err != nil {
		return err
	}
	uuid, err := resumeDevice()
	if err != nil {
		return err
	}
	grub, initramfs := resumeFiles(uuid, offset)
	first, err := rest.Deploy.Content([]byte(grub), resumeGrub())
	if err != nil {
		return err
	}
	second, err := rest.Deploy.Content([]byte(initramfs), resumeInitramfs())
	if err != nil {
		return err
	}
	rest.boot.need(first || second)
	return rest.rebuild(log)
}

func (rest *hibernation) remove(log *runlog.Log) error {
	if err := removeFiles(resumeGrub(), resumeInitramfs()); err != nil {
		return err
	}
	rest.boot.need(true)
	return rest.rebuild(log)
}

func hibernateSteps(context *Context) {
	rest := &hibernation{Context: context, boot: followUp{name: resumeFollowUp}}
	if context.hibernates() {
		context.Run.Step("Let the computer hibernate", rest.setup)
	} else if fsutil.Exists(resumeGrub()) || rest.boot.due() {
		context.Run.Step("Stop hibernating", rest.remove)
	}
}

func resumeReady(*runlog.Log) error {
	if _, err := os.Stat(resumeGrub()); err != nil {
		return err
	}
	return nil
}
