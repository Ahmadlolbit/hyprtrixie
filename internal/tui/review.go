package tui

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/presets"
	"github.com/Ahmadlolbit/hyprtrixie/internal/privilege"
)

const failureKey = "FAILURE_MODE"

func (flow *wizard) jumpRow(title, key string, index int) row {
	return row{label: title, value: flow.summary(key), help: "Enter opens this window", enter: func() tea.Cmd {
		flow.reviewing = true
		flow.jump(index)
		return nil
	}}
}

func (flow *wizard) writeConfiguration() error {
	return privilege.Save(flow.settings.Text(), config.Path())
}

func (flow *wizard) saveConfiguration() tea.Cmd {
	if err := flow.writeConfiguration(); err != nil {
		flow.notice = "Could not save: " + err.Error()
		return nil
	}
	flow.notice = "Saved to " + config.Path()
	if flow.system != nil {
		flow.notice = "Saved to " + config.Path() + " until the restart: copy it to keep it"
	}
	return nil
}

func readable(path string) bool {
	if !fsutil.IsFile(path) {
		return false
	}
	_, err := fsutil.Text(path)
	return err == nil
}

func personHome() string {
	if os.Geteuid() == 0 {
		if person, found := account.Target(); found && person.Home != "" {
			return person.Home
		}
	}
	return env.Home()
}

func fromHome(path string) string {
	if rest, found := strings.CutPrefix(path, "~/"); found {
		return strings.TrimSuffix(personHome(), "/") + "/" + rest
	}
	return path
}

func (flow *wizard) load(path string) tea.Cmd {
	path = fromHome(path)
	before := len(flow.settings.Notes)
	if !readable(path) {
		flow.notice = "Could not read " + path
		return nil
	}
	presets.Import(flow.settings, path)
	if flow.system != nil {
		flow.settings.Set(kernelKey, liveKernel)
	}
	flow.notice = "Loaded " + path
	if found := warnings(flow.settings.Notes[before:]); len(found) > 0 {
		return flow.open(&viewer{lines: found, leave: func() tea.Cmd { return nil }})
	}
	return nil
}

func (flow *wizard) loadPrompt() *dialog {
	return prompt("Load a configuration file", config.Path(), func(path string, ok bool) tea.Cmd {
		if !ok || path == "" {
			return nil
		}
		return flow.load(path)
	})
}

func (flow *wizard) failureRow() row {
	line := flow.pickRow(flow.option(failureKey))
	line.enter = line.space
	return line
}

func (flow *wizard) reviewRows() []row {
	rows := append(flow.systemRows(),
		flow.jumpRow("Profile", "profile", profileAt+flow.offset),
		flow.jumpRow("Applications", "apps", appsAt+flow.offset),
	)
	for index, section := range settingsSections {
		rows = append(rows, flow.jumpRow(section.title, section.key, settingsAt+flow.offset+index))
	}
	return append(rows,
		row{info: true},
		flow.failureRow(),
		row{label: "Save configuration", help: "Keep these choices without installing", enter: func() tea.Cmd {
			return flow.withRoot(flow.saveConfiguration)
		}},
		row{label: "Load configuration", help: "Read the choices from a file", enter: func() tea.Cmd {
			return flow.open(flow.loadPrompt())
		}},
		flow.installRow(),
	)
}

func (flow *wizard) reviewStage() stage {
	return &page{wizard: flow, title: "Review", rows: flow.reviewRows, atEnd: true}
}

func (flow *wizard) installRow() row {
	if flow.system != nil {
		return flow.system.installRow(flow)
	}
	return row{label: "Install", help: "Set this computer up with these choices", enter: func() tea.Cmd {
		return flow.open(confirmation("Start the setup now?", func() tea.Cmd { return flow.withRoot(flow.install.begin) }))
	}}
}
