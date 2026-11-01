package setup

import (
	"os"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
)

const (
	sddmTheme    = "hyprtrixie"
	oldSddmTheme = "catppuccin-mocha-blue"
	displayFile  = "/etc/X11/default-display-manager"
	sessionFile  = "/usr/share/wayland-sessions/hyprland.desktop"
)

func sddmConf() string {
	return env.Or("HYPRTRIXIE_SDDM_CONF", "/etc/sddm.conf.d/10-hyprtrixie.conf")
}

func sddmState() string {
	return env.Or("HYPRTRIXIE_SDDM_STATE", "/var/lib/sddm/state.conf")
}

func sddmThemes() string {
	return env.Or("HYPRTRIXIE_SDDM_THEMES", "/usr/share/sddm/themes")
}

func (context *Context) sddmPackages(log *runlog.Log) error {
	if err := apt.InstallLists(log, "sddm"); err != nil {
		return err
	}
	if context.Config.Get("GREETER") == "wayland" {
		return apt.InstallLists(log, "sddm-wayland")
	}
	return nil
}

func (context *Context) sddmFirstState() error {
	template, err := os.ReadFile(filepath.Join(env.Root(), "system/sddm/state.conf"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(sddmState()), programMode); err != nil {
		return err
	}
	if err := atomic.Write(sddmState(), []byte(files.Render(string(template), "USER="+context.Person.Name)), plainMode); err != nil {
		return err
	}
	run.Quiet(run.Quick, "chown", "sddm:sddm", sddmState())
	return nil
}

func (context *Context) sddmConfigure(*runlog.Log) error {
	source := "system/sddm/" + context.Config.Get("GREETER") + ".conf"
	if err := context.Deploy.File(source, sddmConf(), "THEME="+sddmTheme); err != nil {
		return err
	}
	if context.Person.Name != "" && !fileLineHasPrefix(sddmState(), "Session=") {
		if err := context.sddmFirstState(); err != nil {
			return err
		}
	}
	return atomic.Write(displayFile, []byte("/usr/bin/sddm\n"), plainMode)
}

func sddmEnable(log *runlog.Log) error {
	if err := service.Enable(log, "sddm.service"); err != nil {
		return err
	}
	current, _ := run.CaptureTo(run.Minute, log, "systemctl", "get-default")
	if current == "graphical.target" {
		return nil
	}
	return run.Logged(log, "systemctl", "set-default", "graphical.target")
}

func (context *Context) sddmThemeInstall(log *runlog.Log) error {
	if err := apt.InstallLists(log, "sddm-theme"); err != nil {
		return err
	}
	if err := context.Deploy.Tree("system/sddm/hyprtrixie", filepath.Join(sddmThemes(), sddmTheme)); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(sddmThemes(), oldSddmTheme)); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(env.StateDir(), "releases", "catppuccin-sddm"))
}

func sddmThemeReady(*runlog.Log) error {
	if err := fileThere(filepath.Join(sddmThemes(), sddmTheme, "Main.qml")); err != nil {
		return err
	}
	return expect(fileLineHasPrefix(sddmConf(), "Current="+sddmTheme), sddmConf()+" names another theme")
}

func sddmSteps(context *Context) {
	context.Run.Step("Install the login screen", context.sddmPackages)
	context.Run.Step("Install the hyprtrixie login screen", context.sddmThemeInstall)
	context.Run.Step("Configure the login screen", context.sddmConfigure)
	context.Run.Step("Start into the login screen at boot", sddmEnable)
}

func sddmChecks(context *Context) {
	context.Run.Check("The login screen starts at boot", func(*runlog.Log) error { return holds(service.Enabled("sddm.service")) })
	context.Run.Check("Hyprland is offered as a session", filesCheck(sessionFile))
	context.Run.Check("The login screen uses the hyprtrixie theme", sddmThemeReady)
}
