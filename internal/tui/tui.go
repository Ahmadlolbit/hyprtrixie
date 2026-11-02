package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

func canShow() bool {
	return term.IsTerminal(os.Stdin) && term.IsTerminal(os.Stdout)
}

func Run() int {
	if !canShow() {
		fmt.Fprintln(os.Stderr, "the setup needs a terminal; use: hyprtrixie install --yes")
		return 1
	}
	flow := newWizard(config.Read(config.Path()))
	flow.stages = []stage{flow.startStage(run.Lines(detect.Summary()))}
	if _, err := tea.NewProgram(flow, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return flow.exit
}
