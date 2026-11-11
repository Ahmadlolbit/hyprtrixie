package tui

import "github.com/charmbracelet/lipgloss"

const (
	leaveTitle  = "Leave without installing?"
	stopTitle   = "Stop the setup?"
	testingHelp = "Logs every command, and packs the logs into a tar in the hyprtrixie folder"
)

var (
	secondary = lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#5C5C5C", ANSI256: "240", ANSI: "8"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#A3A3A3", ANSI256: "248", ANSI: "7"},
	}
	hairline = lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#D4D4D4", ANSI256: "252", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#2E2E2E", ANSI256: "236", ANSI: "8"},
	}
	headingStyle = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(secondary)
	ruleStyle    = lipgloss.NewStyle().Foreground(hairline)
	focusStyle   = lipgloss.NewStyle().Reverse(true)
)

func marked(focused bool, text string) string {
	switch {
	case !focused:
		return "  " + text
	case focusStyle.Render(" ") == " ":
		return "> " + text
	}
	return ">" + focusStyle.Render(" "+text+" ")
}

func cut(text string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(text)
}
