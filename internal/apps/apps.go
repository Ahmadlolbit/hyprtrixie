package apps

import (
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/options"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	WebFolder = "/usr/local/share/applications"
	TeamsApp  = "teams-for-linux"
	nameField = 5
)

var Categories = [][2]string{
	{"browsers", "Browsers"}, {"chat", "Chat and calls"}, {"office", "Office and notes"}, {"viewers", "Viewers"},
	{"files", "Files and sharing"}, {"security", "Passwords and VPN"}, {"creative", "Creative"}, {"media", "Audio and video"},
	{"system", "System tools"}, {"games", "Games"},
}

type App struct {
	ID       string
	Category string
	Source   string
	Packages []string
	Name     string
}

func parse(line string) (App, bool) {
	fields := strings.Fields(line)
	if len(fields) <= nameField {
		return App{}, false
	}
	app := App{ID: fields[0], Category: fields[1], Source: fields[3], Name: strings.Join(fields[nameField:], " ")}
	if fields[4] != "-" {
		app.Packages = strings.Split(fields[4], ",")
	}
	return app, true
}

func All() []App {
	var found []App
	for _, line := range strings.Split(options.Catalogue(), "\n") {
		if app, ok := parse(line); ok {
			found = append(found, app)
		}
	}
	return found
}

func InCategory(catalogue []App, category string) []App {
	var found []App
	for _, app := range catalogue {
		if app.Category == category {
			found = append(found, app)
		}
	}
	return found
}

func (app App) Present(installed map[string]bool, teamsSource string) bool {
	switch app.Source {
	case "webapp":
		return fsutil.IsFile(filepath.Join(WebFolder, app.ID+"-web.desktop"))
	case "teams":
		if teamsSource == "snap" {
			return run.Quiet(run.Minute, "snap", "list", TeamsApp) == 0
		}
		return installed[TeamsApp]
	}
	for _, spec := range app.Packages {
		if !installed[apt.Bare(spec)] {
			return false
		}
	}
	return true
}
