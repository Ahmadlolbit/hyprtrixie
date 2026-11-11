package tui

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/words"
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
	return &dialog{title: words.T(title), items: items, answer: answer}
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
	return &dialog{title: words.T(title), field: &input{text: text, allow: anything}, answer: answer}
}

func (box *dialog) press(key tea.KeyMsg) (bool, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		return box.cancelled()
	case tea.KeyEnter:
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

func (box *dialog) answers(width int) []string {
	if box.field != nil {
		return []string{"  " + tail(box.field.shown(), width-2)}
	}
	lines := make([]string, 0, len(box.items))
	for index, item := range box.items {
		lines = append(lines, marked(index == box.cursor, words.T(item)))
	}
	return lines
}

func (box *dialog) context(room int) []string {
	shown := box.lines[min(max(len(box.lines)-room, 0), len(box.lines)):]
	if len(shown) == 0 {
		return nil
	}
	return append(slices.Clone(shown), "")
}

func (box *dialog) draw(width, height int) string {
	answers := box.answers(width)
	lines := []string{headingStyle.Render(box.title), ""}
	lines = append(lines, box.context(height-len(lines)-len(answers)-1)...)
	first := max(len(lines)+box.cursor-height+1, 0)
	lines = append(lines, answers...)
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
