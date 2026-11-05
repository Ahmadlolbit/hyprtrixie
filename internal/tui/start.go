package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
)

const notesShown = 6

func warnings(notes []config.Note) []string {
	var found []string
	for _, note := range notes {
		if note.Level == "warn" {
			found = append(found, clean(note.Text))
		}
	}
	if len(found) > notesShown {
		more := len(found) - notesShown
		found = append(found[:notesShown:notesShown], fmt.Sprintf("... and %d more", more))
	}
	return found
}

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
	return &welcome{started: time.Now(), page: &page{wizard: flow, title: "Start", rows: func() []row {
		rows := []row{{label: "Debian 13 with Hyprland, set up properly", heading: true}, {info: true}}
		for _, line := range machine {
			rows = append(rows, row{label: line, info: true})
		}
		for _, text := range warnings(flow.settings.Notes) {
			rows = append(rows, row{label: text, info: true})
		}
		return append(rows, row{info: true}, flow.testingRow())
	}}}
}
