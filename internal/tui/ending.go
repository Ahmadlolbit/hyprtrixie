package tui

import tea "github.com/charmbracelet/bubbletea"

type ending struct {
	viewer
}

func (end *ending) name() string {
	return "Summary"
}

func (end *ending) arrive() {}

func (end *ending) press(key tea.KeyMsg) tea.Cmd {
	_, cmd := end.viewer.press(key)
	return cmd
}

func (end *ending) draw(_, height int) string {
	return end.viewer.draw(height)
}

func (end *ending) help() string {
	return ""
}

func (end *ending) typing() bool {
	return false
}

func (end *ending) leaving() (string, func() tea.Cmd) {
	return "", end.leave
}
