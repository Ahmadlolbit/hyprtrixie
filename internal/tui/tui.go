package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type window struct{}

func (window) Init() tea.Cmd { return nil }

func (window window) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if _, pressed := message.(tea.KeyMsg); pressed {
		return window, tea.Quit
	}
	return window, nil
}

func (window) View() string {
	return lipgloss.NewStyle().Bold(true).Render("hyprtrixie") + "\n"
}

func Run() int {
	if _, err := tea.NewProgram(window{}).Run(); err != nil {
		return 1
	}
	return 0
}
