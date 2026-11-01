package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/options"
)

const twoValues = 2

type section struct {
	title string
	key   string
}

var settingsSections = []section{
	{"Developer tools", "dev"},
	{"Look and feel", "look"},
	{"Shell", "shell"},
	{"Input and display", "input"},
	{"System", "system"},
	{"Storage", "storage"},
	{"Security and network", "security"},
}

func digit(character rune) bool {
	return character >= '0' && character <= '9'
}

func (flow *wizard) shown(option options.Option) bool {
	return option.Shows(flow.settings.Facts(flow.nvidia))
}

func (flow *wizard) tick(key, item string) {
	words := strings.Fields(flow.settings.Get(key))
	if index := slices.Index(words, item); index >= 0 {
		words = slices.Delete(words, index, index+1)
	} else {
		words = append(words, item)
	}
	flow.settings.Set(key, strings.Join(words, " "))
}

func (flow *wizard) tickRow(key, item, label string) row {
	mark := "[ ]"
	if flow.settings.Enabled(key, item) {
		mark = "[x]"
	}
	return row{label: label, mark: mark, space: func() tea.Cmd {
		flow.tick(key, item)
		return nil
	}}
}

func (flow *wizard) swapRow(option options.Option, values []string) row {
	return flow.valueRow(option, func() tea.Cmd {
		current := slices.Index(values, flow.settings.Get(option.Key))
		flow.settings.Set(option.Key, values[(current+1)%len(values)])
		return nil
	})
}

func (flow *wizard) pickRow(option options.Option) row {
	return flow.valueRow(option, func() tea.Cmd {
		return flow.open(flow.picker(option))
	})
}

func (flow *wizard) picker(option options.Option) *dialog {
	box := question(option.Label, option.Items(), func(text string, ok bool) tea.Cmd {
		if ok {
			flow.settings.Set(option.Key, text)
		}
		return nil
	})
	box.lines = []string{option.Help}
	box.cursor = max(slices.Index(option.Items(), flow.settings.Get(option.Key)), 0)
	return box
}

func (flow *wizard) valueRow(option options.Option, change func() tea.Cmd) row {
	return row{label: option.Label, value: flow.settings.Get(option.Key), help: option.Help, space: change}
}

func (flow *wizard) fieldRow(option options.Option) row {
	line := flow.valueRow(option, nil)
	line.input = &field{
		allow: config.SafeText,
		valid: option.Valid,
		set:   func(text string) { flow.settings.Set(option.Key, text) },
	}
	if option.Kind == options.Number {
		line.input.allow = digit
		line.input.replace = true
	}
	return line
}

func (flow *wizard) optionRows(option options.Option) []row {
	switch option.Kind {
	case options.Multi:
		var rows []row
		for _, item := range option.Items() {
			rows = append(rows, flow.tickRow(option.Key, item, item))
		}
		return rows
	case options.YesNo:
		return []row{flow.swapRow(option, []string{"yes", "no"})}
	case options.Choice:
		if items := option.Items(); len(items) == twoValues {
			return []row{flow.swapRow(option, items)}
		}
		return []row{flow.pickRow(option)}
	}
	return []row{flow.fieldRow(option)}
}

func (flow *wizard) sectionRows(section string) []row {
	var rows []row
	for _, option := range flow.settings.Options() {
		if option.Section == section && flow.shown(option) {
			rows = append(rows, flow.optionRows(option)...)
		}
	}
	return rows
}

func (flow *wizard) settingsStage(title, section string) stage {
	return &page{wizard: flow, title: title, rows: func() []row { return flow.sectionRows(section) }}
}

func (flow *wizard) display(option options.Option) string {
	if option.Kind == options.Multi {
		return fmt.Sprintf("%d of %d", len(strings.Fields(flow.settings.Get(option.Key))), len(option.Items()))
	}
	if value := flow.settings.Get(option.Key); value != "" {
		return value
	}
	return "none"
}

func (flow *wizard) summary(section string) string {
	var parts []string
	for _, option := range flow.settings.Options() {
		if option.Section == section && flow.shown(option) && len(parts) < summaryParts {
			parts = append(parts, flow.display(option))
		}
	}
	return strings.Join(parts, " · ")
}
