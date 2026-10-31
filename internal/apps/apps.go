package apps

import (
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/options"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const nameField = 5

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

func Installed() map[string]bool {
	output, _ := run.CaptureQuiet(run.Slow, "dpkg-query", "-W", "-f", "${Package}\t${Status}\n")
	found := map[string]bool{}
	for _, line := range run.Lines(output) {
		if name, status, _ := strings.Cut(line, "\t"); status == "install ok installed" {
			found[name] = true
		}
	}
	return found
}

func (app App) Present(installed map[string]bool, teamsSource string) bool {
	switch app.Source {
	case "webapp":
		return fsutil.IsFile("/usr/local/share/applications/" + app.ID + "-web.desktop")
	case "teams":
		if teamsSource == "snap" {
			return run.Quiet(run.Quick, "snap", "list", "teams-for-linux") == 0
		}
		return installed["teams-for-linux"]
	}
	for _, spec := range app.Packages {
		if !installed[strings.TrimSuffix(spec, "/backports")] {
			return false
		}
	}
	return true
}
