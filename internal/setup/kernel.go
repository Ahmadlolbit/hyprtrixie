package setup

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/bootimage"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
)

const (
	kernelPackage  = "linux-image-amd64"
	kernelSpec     = kernelPackage + "/backports"
	packageFiles   = "/var/lib/dpkg/info/linux-image-"
	nvidiaSettings = "/etc/modprobe.d/hyprtrixie-nvidia.conf"
	hybridPower    = " NVreg_DynamicPowerManagement=0x02"
	kernelKept     = "The backports kernel stays installed, because the setup never removes a kernel."
	kernelBack     = "To go back to Debian's own: sudo apt install linux-image-amd64/trixie linux-headers-amd64/trixie " +
		"(leave out the headers if you don't have them) and say yes to going back, restart into Debian's kernel from " +
		"Advanced options in the boot menu, then sudo apt remove the backports kernel: dpkg -l 'linux-image-*' | grep bpo shows its name."
)

var nvidiaUnits = []string{"nvidia-suspend.service", "nvidia-resume.service", "nvidia-hibernate.service"}

type nvidiaDriver struct {
	*Context
	hardware  detect.Hardware
	initramfs followUp
}

func (context *Context) kernelSpecs(specs ...string) []string {
	if context.Config.Get("KERNEL") != "stable" {
		return specs
	}
	plain := make([]string, len(specs))
	for index, spec := range specs {
		plain[index] = apt.Bare(spec)
	}
	return plain
}

func backportsKernel() bool {
	return strings.Contains(apt.Version(kernelPackage), "bpo")
}

func (context *Context) driverSpecs(specs ...string) []string {
	if backportsKernel() {
		return specs
	}
	return context.kernelSpecs(specs...)
}

func (context *Context) kernelInstall(log *runlog.Log) error {
	return apt.Install(log, context.kernelSpecs(kernelSpec)...)
}

func (context *Context) kernelNotes() []string {
	if context.Config.Get("KERNEL") != "stable" || !backportsKernel() {
		return nil
	}
	return []string{kernelKept, kernelBack}
}

func newestKernel() string {
	return bootimage.Newest()
}

func bootedAt() int64 {
	text, _ := fsutil.Text("/proc/uptime")
	whole, _, _ := strings.Cut(text, ".")
	seconds, _ := strconv.ParseInt(whole, 10, 64)
	return time.Now().Unix() - seconds
}

func installedSinceBoot(kernel string) bool {
	info, err := os.Stat(packageFiles + kernel + ".list")
	return err == nil && info.Mode().IsRegular() && info.ModTime().Unix() > bootedAt()
}

func kernelBooted(log *runlog.Log) error {
	newest := newestKernel()
	running, _ := run.CaptureQuiet(run.Minute, "uname", "-r")
	fmt.Fprintf(log, "newest installed: %s, running: %s\n", cmp.Or(newest, "none"), running)
	if newest == "" || newest == running {
		return nil
	}
	if !installedSinceBoot(newest) {
		return errors.New("this boot did not use the newest kernel; check the default GRUB entry")
	}
	fmt.Fprintf(log, "reboot to start %s\n", newest)
	return nil
}

func kernelFromBackports(*runlog.Log) error {
	version := apt.Version(kernelPackage)
	return expect(strings.Contains(version, "bpo"), kernelPackage+" is at version "+version)
}

func nvidiaBuilt(*runlog.Log) error {
	kernel := newestKernel()
	return quietly("there's no nvidia module for "+kernel, "modinfo", "-k", kernel, "nvidia")
}

func (driver *nvidiaDriver) wanted() bool {
	return driver.hardware.HasGPU("nvidia") && driver.Config.Get("GRAPHICS") == "proprietary"
}

func (driver *nvidiaDriver) install(log *runlog.Log) error {
	list := "nvidia-closed"
	if driver.hardware.NvidiaOpen {
		list = "nvidia-open"
	}
	listed, err := apt.List(log, list)
	if err != nil {
		return err
	}
	return apt.Install(log, driver.driverSpecs(listed...)...)
}

func (driver *nvidiaDriver) configure(log *runlog.Log) error {
	power := ""
	if driver.hardware.Hybrid() {
		power = hybridPower
	}
	changed, err := driver.Deploy.Changed("system/modprobe/nvidia.conf", nvidiaSettings, "POWER="+power)
	driver.initramfs.need(changed)
	if err != nil {
		return err
	}
	if err := service.Enable(log, nvidiaUnits...); err != nil {
		return err
	}
	rebuild := bootimage.Rebuild(true)
	return driver.initramfs.do(func() error { return run.Logged(log, rebuild[0], rebuild[1:]...) })
}

func (context *Context) webcamWanted() bool {
	return context.Config.Get("WEBCAM") == "auto" && context.hardware().IPU6
}

func (context *Context) webcamInstall(log *runlog.Log) error {
	listed, err := apt.List(log, "webcam")
	if err != nil {
		return err
	}
	return apt.Install(log, context.driverSpecs(listed...)...)
}

func webcamReady(*runlog.Log) error {
	return apt.Commands("cam", "gst-launch-1.0")
}

func kernelSteps(context *Context) {
	driver := nvidiaDriver{Context: context, hardware: context.hardware(), initramfs: followUp{name: "nvidia-initramfs"}}
	context.Run.Step("Install the "+context.Config.Get("KERNEL")+" kernel", context.kernelInstall)
	for _, note := range context.kernelNotes() {
		context.Run.Note(note)
	}
	if driver.wanted() {
		context.Run.Step("Install the NVIDIA driver", driver.install)
		context.Run.Step("Configure the NVIDIA driver", driver.configure)
	}
	if context.webcamWanted() {
		context.Run.Step("Set up the IPU6 webcam", context.webcamInstall)
	}
}

func kernelChecks(context *Context) {
	driver := nvidiaDriver{Context: context, hardware: context.hardware()}
	if context.Config.Get("KERNEL") == "backports" {
		context.Run.Check("The kernel comes from backports", kernelFromBackports)
	}
	context.Run.Check("The newest kernel is running (or starts after a reboot)", kernelBooted)
	if driver.wanted() {
		context.Run.Check("The NVIDIA module is built for the newest kernel", nvidiaBuilt)
	}
	if context.webcamWanted() {
		context.Run.Check("The IPU6 webcam's tools are there", webcamReady)
	}
}
