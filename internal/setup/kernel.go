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
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
)

const (
	kernelPackage  = "linux-image-amd64"
	kernelSpec     = kernelPackage + "/backports"
	bootDir        = "/boot"
	kernelPrefix   = "vmlinuz-"
	packageFiles   = "/var/lib/dpkg/info/linux-image-"
	nvidiaSettings = "/etc/modprobe.d/hyprtrixie-nvidia.conf"
	hybridPower    = " NVreg_DynamicPowerManagement=0x02"
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

func (context *Context) kernelInstall(log *runlog.Log) error {
	return apt.Install(log, context.kernelSpecs(kernelSpec)...)
}

func newestKernel() string {
	output, _ := run.CaptureQuiet(run.Minute, "ls", "-v", "--quoting-style=literal", bootDir)
	newest := ""
	for _, name := range run.Lines(output) {
		if version, ok := strings.CutPrefix(name, kernelPrefix); ok {
			newest = version
		}
	}
	return newest
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
	return apt.Install(log, driver.kernelSpecs(listed...)...)
}

func (driver *nvidiaDriver) configure(log *runlog.Log) error {
	power := ""
	if driver.hardware.Hybrid() {
		power = hybridPower
	}
	changed, err := driver.Deploy.Changed("system/modprobe/nvidia.conf", nvidiaSettings, "POWER="+power)
	if err != nil {
		return err
	}
	driver.initramfs.need(changed)
	if err := service.Enable(log, nvidiaUnits...); err != nil {
		return err
	}
	return driver.initramfs.do(func() error { return run.Logged(log, "update-initramfs", "-u", "-k", "all") })
}

func kernelSteps(context *Context) {
	driver := nvidiaDriver{Context: context, hardware: detect.ReadHardware()}
	context.Run.Step("Install the "+context.Config.Get("KERNEL")+" kernel", context.kernelInstall)
	if driver.wanted() {
		context.Run.Step("Install the NVIDIA driver", driver.install)
		context.Run.Step("Configure the NVIDIA driver", driver.configure)
	}
}

func kernelChecks(context *Context) {
	driver := nvidiaDriver{Context: context, hardware: detect.ReadHardware()}
	if context.Config.Get("KERNEL") == "backports" {
		context.Run.Check("The kernel comes from backports", kernelFromBackports)
	}
	context.Run.Check("The newest kernel is running (or starts after a reboot)", kernelBooted)
	if driver.wanted() {
		context.Run.Check("The NVIDIA module is built for the newest kernel", nvidiaBuilt)
	}
}
