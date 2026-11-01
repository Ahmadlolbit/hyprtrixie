package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apps"
	"github.com/Ahmadlolbit/hyprtrixie/internal/options"
)

const (
	appsKey    = "APPS"
	profileKey = "PROFILE"
)

var sourceNotes = map[string]string{"snap": "Snap also installs snapd, a background service nothing else here uses."}

func (flow *wizard) profileRow(option options.Option, profile string) row {
	mark := "( )"
	if flow.settings.Get(profileKey) == profile {
		mark = "(*)"
	}
	pick := func() tea.Cmd { return flow.pickProfile(profile) }
	return row{label: profile, mark: mark, help: option.Help, space: pick, enter: func() tea.Cmd {
		pick()
		return flow.forward()
	}}
}

func (flow *wizard) pickProfile(profile string) tea.Cmd {
	if flow.settings.Get(profileKey) == profile {
		return nil
	}
	flow.settings.Set(profileKey, profile)
	return flow.open(confirmation("Reset applications and tools to the "+profile+" preset?", func() tea.Cmd {
		flow.settings.ResetProfile()
		return nil
	}))
}

func (flow *wizard) option(key string) options.Option {
	for _, option := range flow.settings.Options() {
		if option.Key == key {
			return option
		}
	}
	return options.Option{}
}

func (flow *wizard) profileStage() stage {
	return &page{wizard: flow, title: "Profile", rows: func() []row {
		option := flow.option(profileKey)
		var rows []row
		for _, profile := range option.Items() {
			rows = append(rows, flow.profileRow(option, profile))
		}
		return rows
	}}
}

func (flow *wizard) sourceQuestion(option options.Option) *dialog {
	box := flow.picker(option)
	for _, item := range option.Items() {
		if note, found := sourceNotes[item]; found {
			box.lines = append(box.lines, note)
		}
	}
	return box
}

func (flow *wizard) appRow(app apps.App) row {
	line := flow.tickRow(appsKey, app.ID, app.Name)
	tick := line.space
	key, asks := options.SourceOf(app.ID)
	if !asks {
		return line
	}
	line.space = func() tea.Cmd {
		tick()
		if flow.settings.Enabled(appsKey, app.ID) {
			return flow.open(flow.sourceQuestion(flow.option(key)))
		}
		return nil
	}
	return line
}

func (flow *wizard) appRows(catalogue []apps.App) []row {
	var rows []row
	for _, category := range apps.Categories {
		found := apps.InCategory(catalogue, category[0])
		if len(found) == 0 {
			continue
		}
		rows = append(rows, row{label: category[1], heading: true})
		for _, app := range found {
			rows = append(rows, flow.appRow(app))
		}
	}
	return rows
}

func (flow *wizard) appsStage(catalogue []apps.App) stage {
	return &page{wizard: flow, title: "Applications", rows: func() []row { return flow.appRows(catalogue) }}
}
