package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apps"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

func canShow() bool {
	return term.IsTerminal(os.Stdin) && term.IsTerminal(os.Stdout)
}

func (flow *wizard) allStages(machine []string, catalogue []apps.App) []stage {
	stages := []stage{flow.startStage(machine), flow.profileStage(), flow.appsStage(catalogue)}
	for _, section := range settingsSections {
		stages = append(stages, flow.settingsStage(section.title, section.key))
	}
	flow.install = &installer{flow: flow}
	flow.end = &ending{viewer: viewer{leave: func() tea.Cmd { return tea.Quit }}}
	return append(stages, flow.reviewStage(), flow.install, flow.end)
}

func Run() int {
	if !canShow() {
		fmt.Fprintln(os.Stderr, "the setup needs a terminal; use: hyprtrixie install --yes")
		return 1
	}
	flow := newWizard(config.Read(config.Path()))
	flow.nvidia = detect.ReadHardware().HasGPU("nvidia")
	flow.stages = flow.allStages(run.Lines(detect.Summary()), apps.All())
	if _, err := tea.NewProgram(flow, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, line := range flow.closing {
		fmt.Println(line)
	}
	return flow.exit
}
