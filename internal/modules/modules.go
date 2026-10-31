package modules

import (
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

type Module struct {
	ID    string
	Title string
}

var All = []Module{
	{"00", "Setup screens and logging"},
	{"01", "System preflight"},
	{"02", "Repositories and base packages"},
	{"03", "Kernel and drivers"},
	{"04", "Swap and memory"},
	{"05", "Snapshots and rollback"},
	{"06", "Hyprland desktop"},
	{"07", "Login screen"},
	{"08", "Terminal and shell"},
	{"09", "Command-line tools"},
	{"10", "Applications"},
	{"11", "Developer tools"},
	{"12", "Desktop session"},
	{"13", "Bar"},
	{"14", "Themes"},
	{"15", "Power and security"},
	{"16", "Backups"},
	{"17", "Final checks and report"},
}

func StatusFile() string {
	return filepath.Join(env.Or("HYPRTRIXIE_STATE_DIR", "/var/lib/hyprtrixie"), "status")
}

func StatusOf(text, id string) (string, string) {
	for _, line := range strings.Split(text, "\n") {
		if fields := strings.Split(line, "|"); fields[0] == id {
			fields = append(fields, "", "")
			return fields[1], fields[2]
		}
	}
	return "", ""
}
