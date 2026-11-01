package modules

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	statusMode = 0o644
	folderMode = 0o755
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
	return filepath.Join(env.StateDir(), "status")
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

func SaveStatus(results map[string]string) error {
	if err := os.MkdirAll(env.StateDir(), folderMode); err != nil {
		return err
	}
	var lines []string
	for _, line := range strings.Split(fsutil.Optional(StatusFile()), "\n") {
		id, _, _ := strings.Cut(line, "|")
		if _, fresh := results[id]; line != "" && !fresh {
			lines = append(lines, line)
		}
	}
	now := time.Now().Format(time.DateTime)
	for _, id := range slices.Sorted(maps.Keys(results)) {
		lines = append(lines, id+"|"+results[id]+"|"+now)
	}
	slices.Sort(lines)
	var text strings.Builder
	for _, line := range lines {
		text.WriteString(line + "\n")
	}
	return atomic.Install(StatusFile(), []byte(text.String()), statusMode)
}
