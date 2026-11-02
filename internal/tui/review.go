package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
)

const failureKey = "FAILURE_MODE"

func (flow *wizard) jumpRow(title, key string, index int) row {
	return row{label: title, value: flow.summary(key), help: "Enter opens this window", enter: func() tea.Cmd {
		flow.reviewing = true
		flow.jump(index)
		return nil
	}}
}

func (flow *wizard) saveConfiguration() tea.Cmd {
	path := config.Path()
	if err := flow.settings.Save(path); err != nil {
		flow.notice = "Could not save " + path + ": " + err.Error()
		return nil
	}
	flow.notice = "Saved to " + path
	return nil
}

func (flow *wizard) loadPrompt() *dialog {
	return prompt("Load a configuration file", config.Path(), func(path string, ok bool) tea.Cmd {
		switch {
		case !ok || path == "":
		case flow.settings.Load(path):
			flow.notice = "Loaded " + path
		default:
			flow.notice = "Could not read " + path
		}
		return nil
	})
}

func (flow *wizard) reviewRows() []row {
	rows := []row{
		flow.jumpRow("Profile", "profile", profileAt),
		flow.jumpRow("Applications", "apps", appsAt),
	}
	for index, section := range settingsSections {
		rows = append(rows, flow.jumpRow(section.title, section.key, settingsAt+index))
	}
	return append(rows,
		row{info: true},
		flow.pickRow(flow.option(failureKey)),
		row{label: "Save configuration", help: "Keep these choices without installing", enter: flow.saveConfiguration},
		row{label: "Load configuration", help: "Read the choices from a file", enter: func() tea.Cmd {
			return flow.open(flow.loadPrompt())
		}},
		row{label: "Install", help: "Set this computer up with these choices", enter: func() tea.Cmd {
			return flow.open(confirmation("Start the setup now?", flow.install.begin))
		}},
	)
}

func (flow *wizard) reviewStage() stage {
	return &page{wizard: flow, title: "Review", rows: flow.reviewRows, atEnd: true}
}
