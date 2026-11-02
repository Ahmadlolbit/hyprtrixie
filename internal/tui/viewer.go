package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type viewer struct {
	lines  []string
	top    int
	height int
	leave  func() tea.Cmd
}

func (box *viewer) scroll(delta int) {
	last := max(len(box.lines)-box.height, 0)
	box.top = min(max(box.top+delta, 0), last)
}

func (box *viewer) press(key tea.KeyMsg) (bool, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		box.scroll(-1)
	case "down", "j":
		box.scroll(1)
	case "pgup":
		box.scroll(-box.height)
	case "pgdown", " ":
		box.scroll(box.height)
	case "home":
		box.scroll(-len(box.lines))
	case "end":
		box.scroll(len(box.lines))
	case "enter", "esc", "q":
		return true, box.leave()
	}
	return false, nil
}

func (box *viewer) draw(height int) string {
	box.height = height
	box.scroll(0)
	return strings.Join(box.lines[box.top:min(box.top+height, len(box.lines))], "\n")
}

func (box *viewer) keys() string {
	return "Arrows scroll · Enter or Esc closes"
}
