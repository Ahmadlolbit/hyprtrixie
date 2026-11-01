package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type dialog struct {
	title  string
	lines  []string
	items  []string
	cursor int
	field  *input
	locked bool
	answer func(text string, ok bool) tea.Cmd
}

func question(title string, items []string, answer func(string, bool) tea.Cmd) *dialog {
	return &dialog{title: title, items: items, answer: answer}
}

func confirmation(title string, yes func() tea.Cmd) *dialog {
	return question(title, []string{"Yes", "No"}, func(text string, _ bool) tea.Cmd {
		if text == "Yes" {
			return yes()
		}
		return nil
	})
}

func prompt(title, text string, answer func(string, bool) tea.Cmd) *dialog {
	return &dialog{title: title, field: &input{text: text, allow: anything}, answer: answer}
}

func (box *dialog) press(key tea.KeyMsg) (bool, tea.Cmd) {
	switch key.String() {
	case "esc":
		return box.cancelled()
	case "enter":
		return true, box.answer(box.chosen(), true)
	}
	if box.field != nil {
		box.field.press(key)
		return false, nil
	}
	box.move(key.String())
	return false, nil
}

func (box *dialog) cancelled() (bool, tea.Cmd) {
	if box.locked {
		return false, nil
	}
	return true, box.answer("", false)
}

func (box *dialog) chosen() string {
	if box.field != nil {
		return box.field.text
	}
	return box.items[box.cursor]
}

func (box *dialog) move(name string) {
	switch name {
	case "up", "k", "shift+tab":
		box.cursor = max(box.cursor-1, 0)
	case "down", "j", "tab":
		box.cursor = min(box.cursor+1, len(box.items)-1)
	}
}

func (box *dialog) body() []string {
	lines := []string{headingStyle.Render(box.title), ""}
	lines = append(lines, box.lines...)
	if len(box.lines) > 0 {
		lines = append(lines, "")
	}
	if box.field != nil {
		return append(lines, "  "+box.field.shown())
	}
	for index, item := range box.items {
		lines = append(lines, marked(index == box.cursor, item))
	}
	return lines
}

func (box *dialog) draw(height int) string {
	lines := box.body()
	first := max(box.cursor+len(lines)-len(box.items)-height+1, 0)
	if box.field != nil || len(lines) <= height {
		first = 0
	}
	return strings.Join(lines[first:min(first+height, len(lines))], "\n")
}

func (box *dialog) keys() string {
	switch {
	case box.locked && box.field != nil:
		return "Type, then Enter"
	case box.locked:
		return "Arrows move · Enter picks"
	case box.field != nil:
		return "Type, then Enter · Esc cancels"
	}
	return "Arrows move · Enter picks · Esc cancels"
}
