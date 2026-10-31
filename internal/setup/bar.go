package setup

import (
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	barSettings = "/etc/hyprtrixie/shell.json"
	barSensors  = "hyprtrixie-sensors"
)

func barDir() string {
	return filepath.Join(env.ShareDir(), "shell")
}

func barReady(*runlog.Log) error {
	if err := fileThere(filepath.Join(barDir(), "shell.qml")); err != nil {
		return err
	}
	if err := fileThere(barSettings); err != nil {
		return err
	}
	sensors := filepath.Join(env.BinDir(), barSensors)
	if err := expect(fsutil.Runnable(sensors), sensors+" won't run"); err != nil {
		return err
	}
	return apt.Commands("qs")
}

func (context *Context) barInstall(*runlog.Log) error {
	if err := context.Deploy.Tree("shell", barDir()); err != nil {
		return err
	}
	return context.Deploy.Tool(barSensors)
}

func (context *Context) barSetup(*runlog.Log) error {
	settings := context.Config
	return context.Deploy.File("system/shell.json", barSettings, "POSITION="+settings.Get("BAR_POSITION"), "CLOCK="+settings.Get("CLOCK_FORMAT"),
		"STYLE="+settings.Get("WINDOW_STYLE"), "PRESET="+settings.Get("BAR_PRESET"))
}

func barSteps(context *Context) {
	context.Run.Step("Install the bar", context.barInstall)
	context.Run.Step("Apply the bar settings", context.barSetup)
}

func barChecks(context *Context) {
	context.Run.Check("The bar is installed", barReady)
}
