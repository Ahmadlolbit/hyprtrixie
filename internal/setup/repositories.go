package setup

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const securityMirror = "https://security.debian.org/debian-security"

func sourcesList() string {
	return env.Or("HYPRTRIXIE_SOURCES_LIST", "/etc/apt/sources.list")
}

func anyEntry(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, blanks), "deb")
}

func debianEntry(line string) bool {
	rest, found := strings.CutPrefix(strings.TrimLeft(line, blanks), "deb")
	if !found {
		return false
	}
	rest = strings.TrimPrefix(rest, "-src")
	return strings.IndexAny(rest, blanks) == 0 && (strings.Contains(line, "debian.org") || strings.Contains(line, "cdrom:"))
}

func moveDebianEntries(log *runlog.Log) error {
	path := sourcesList()
	lines, err := fsutil.Lines(path)
	if err != nil || !slices.ContainsFunc(lines, anyEntry) {
		return err
	}
	if err := keepOld(log, path); err != nil {
		return err
	}
	var kept strings.Builder
	for _, line := range lines {
		if !debianEntry(line) {
			kept.WriteString(line + "\n")
		}
	}
	if err := atomic.Write(path, []byte(kept.String()), plainMode); err != nil {
		return err
	}
	log.Line("info", "moved the Debian entries out of "+path+" (backup in "+path+".hyprtrixie-old)")
	return nil
}

type sourcesSetup struct {
	*Context
	update followUp
}

func (sources *sourcesSetup) write(log *runlog.Log) error {
	if err := apt.Install(log, "ca-certificates"); err != nil {
		return err
	}
	if err := moveDebianEntries(log); err != nil {
		return err
	}
	mirror := "MIRROR=" + sources.Config.Get("DEBIAN_MIRROR")
	debian, err := sources.Deploy.Changed("system/apt/debian.sources", filepath.Join(apt.SourcesDir(), "debian.sources"), mirror, "SECURITY="+securityMirror)
	if err != nil {
		return err
	}
	sources.update.need(debian)
	backports, err := sources.Deploy.Changed("system/apt/debian-backports.sources", filepath.Join(apt.SourcesDir(), "debian-backports.sources"), mirror)
	if err != nil {
		return err
	}
	sources.update.need(backports)
	if !sources.update.owed {
		return apt.Update(log)
	}
	return sources.update.do(func() error { return apt.ForceUpdate(log) })
}

func extrepoSetup(log *runlog.Log) error {
	if err := apt.Install(log, "extrepo"); err != nil {
		return err
	}
	for _, policy := range []string{"main", "contrib", "non-free"} {
		if err := apt.ExtrepoPolicy(policy); err != nil {
			return err
		}
	}
	return nil
}

func extrepoReady(*runlog.Log) error {
	if err := apt.Present("extrepo"); err != nil {
		return err
	}
	allowed, err := apt.ExtrepoAllows("non-free")
	if err != nil {
		return err
	}
	return expect(allowed, "extrepo can't use non-free repositories")
}

func firmwareLists(hardware detect.Hardware) []string {
	lists := []string{"firmware"}
	switch hardware.CPU {
	case "intel":
		lists = append(lists, "cpu-intel", "audio-sof")
	case "amd":
		lists = append(lists, "cpu-amd")
	}
	for _, vendor := range []string{"intel", "amd", "nvidia"} {
		if hardware.HasGPU(vendor) {
			lists = append(lists, "gpu-"+vendor)
		}
	}
	for _, vendor := range []string{"intel", "realtek", "mediatek", "atheros", "broadcom"} {
		if hardware.HasWifi(vendor) {
			lists = append(lists, "wifi-"+vendor)
		}
	}
	return lists
}

func (context *Context) firmwarePackages(log *runlog.Log, lists []string) ([]string, error) {
	var packages []string
	for _, list := range lists {
		more, err := apt.List(log, list)
		if err != nil {
			return nil, err
		}
		packages = append(packages, more...)
	}
	if context.Config.Get("KERNEL") == "stable" {
		for index, name := range packages {
			packages[index] = apt.Bare(name)
		}
	}
	return packages, nil
}

func (context *Context) firmwareInstall(log *runlog.Log) error {
	lists := firmwareLists(detect.ReadHardware())
	packages, err := context.firmwarePackages(log, lists)
	if err != nil {
		return err
	}
	fmt.Fprintf(log, "firmware for: %s \n", strings.Join(lists, " "))
	return apt.Install(log, packages...)
}

func (context *Context) firmwarePresent(log *runlog.Log) error {
	packages, err := context.firmwarePackages(log, firmwareLists(detect.ReadHardware()))
	if err != nil {
		return err
	}
	return apt.Present(packages...)
}

func aptPolicy() string {
	output, _ := run.CaptureQuiet(run.Minute, "apt-cache", "policy")
	return output
}

func backportsListed(*runlog.Log) error {
	return holds(strings.Contains(aptPolicy(), "n=trixie-backports,"))
}

func firmwareComponent(*runlog.Log) error {
	return expect(slices.ContainsFunc(run.Lines(aptPolicy()), func(line string) bool {
		return strings.Contains(line, "n=trixie,") && strings.Contains(line, "c=non-free-firmware,")
	}), "apt doesn't list non-free-firmware for trixie")
}

func repositorySteps(context *Context) {
	sources := sourcesSetup{Context: context}
	context.Run.Step("Use Debian over HTTPS with backports", sources.write)
	context.Run.Step("Prepare vendor repositories", extrepoSetup)
	context.Run.Step("Install base packages", packages("base"))
	context.Run.Step("Install firmware for this machine", context.firmwareInstall)
}

func repositoryChecks(context *Context) {
	context.Run.Check("Base packages are installed", func(log *runlog.Log) error { return apt.ListsPresent(log, "base") })
	context.Run.Check("Vendor repositories can be enabled", extrepoReady)
	context.Run.Check("Firmware for this machine is installed", context.firmwarePresent)
	context.Run.Check("Backports are available", backportsListed)
	context.Run.Check("Firmware and non-free components are on", firmwareComponent)
}
