package main

import (
	"os"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/engine-switch"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-background"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-care"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-follow-theme"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-installer"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-pick"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-pkg"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-record"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-screensaver"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-sensors"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-start"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-terminal"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-tip"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-weather"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/hyprtrixie-webapp"
	"github.com/Ahmadlolbit/hyprtrixie/cmd/hyprtrixie-core/tools/snapper-rollback"
)

var tools = map[string]func(){
	"engine-switch":           engineswitch.Main,
	"hyprtrixie":              wizard.Main,
	"hyprtrixie-background":   background.Main,
	"hyprtrixie-care":         care.Main,
	"hyprtrixie-follow-theme": followtheme.Main,
	"hyprtrixie-installer":    installer.Main,
	"hyprtrixie-pick":         pick.Main,
	"hyprtrixie-pkg":          pkg.Main,
	"hyprtrixie-record":       record.Main,
	"hyprtrixie-screensaver":  screensaver.Main,
	"hyprtrixie-sensors":      sensors.Main,
	"hyprtrixie-start":        start.Main,
	"hyprtrixie-terminal":     terminal.Main,
	"hyprtrixie-tip":          tip.Main,
	"hyprtrixie-weather":      weather.Main,
	"hyprtrixie-webapp":       webapp.Main,
	"snapper-rollback":        snapperrollback.Main,
}

func runTool() {
	name := filepath.Base(os.Args[0])
	if name == "hyprtrixie" && os.Getenv("HYPRTRIXIE_ENGINE") != "" {
		return
	}
	if tool, ok := tools[name]; ok {
		tool()
		os.Exit(0)
	}
}
