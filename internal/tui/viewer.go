package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewer struct {
	lines  []string
	shown  []string
	width  int
	top    int
	height int
	leave  func() tea.Cmd
}

func (box *viewer) scroll(delta int) {
	last := max(len(box.shown)-box.height, 0)
	box.top = min(max(box.top+delta, 0), last)
}

func (box *viewer) wrap(width int) {
	if width == box.width && box.shown != nil {
		return
	}
	box.width = width
	box.shown = make([]string, 0, len(box.lines))
	for _, line := range box.lines {
		for _, piece := range strings.Split(lipgloss.NewStyle().Width(width).Render(line), "\n") {
			box.shown = append(box.shown, strings.TrimRight(piece, " "))
		}
	}
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
		box.scroll(-len(box.shown))
	case "end":
		box.scroll(len(box.shown))
	case "enter", "esc", "q":
		return true, box.leave()
	}
	return false, nil
}

func (box *viewer) draw(width, height int) string {
	box.height = height
	box.wrap(width)
	box.scroll(0)
	return strings.Join(box.shown[box.top:min(box.top+height, len(box.shown))], "\n")
}

func (box *viewer) keys() string {
	return "Arrows scroll · Enter or Esc closes"
}
