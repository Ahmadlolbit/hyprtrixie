package tui

import tea "github.com/charmbracelet/bubbletea"

func (flow *wizard) testingRow() row {
	value := "off"
	if flow.testing {
		value = "on"
	}
	return row{label: "Testing mode", value: value, help: testingHelp, space: func() tea.Cmd {
		flow.testing = !flow.testing
		return nil
	}}
}

func (flow *wizard) startStage(machine []string) stage {
	return &page{wizard: flow, title: "Start", rows: func() []row {
		rows := []row{{label: "Debian 13 with Hyprland, set up properly", heading: true}, {info: true}}
		for _, line := range machine {
			rows = append(rows, row{label: line, info: true})
		}
		for _, note := range flow.settings.Notes {
			if note.Level == "warn" {
				rows = append(rows, row{label: note.Text, info: true})
			}
		}
		return append(rows, row{info: true}, flow.testingRow())
	}}
}
