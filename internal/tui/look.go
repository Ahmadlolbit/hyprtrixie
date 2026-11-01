package tui

import "github.com/charmbracelet/lipgloss"

const leaveTitle = "Leave without installing?"

var (
	headingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
	ruleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	focusStyle   = lipgloss.NewStyle().Bold(true)
)

func marked(focused bool, text string) string {
	if focused {
		return focusStyle.Render("> " + text)
	}
	return "  " + text
}

func cut(text string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(text)
}
