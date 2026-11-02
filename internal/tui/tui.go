package tui

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apps"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/lock"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

const stoppedCode = 130

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

func accountHelper() int {
	os.Setenv("HYPRTRIXIE_ENGINE", "1")
	return run.Report(run.Exec(filepath.Join(env.ToolsDir(), "hyprtrixie-core"), "account"))
}

func Run() int {
	if os.Geteuid() == 0 && os.Getenv("SUDO_USER") != "" {
		fmt.Fprintln(os.Stderr, "run it as yourself, without sudo: ./hyprtrixie")
		return 1
	}
	if os.Geteuid() == 0 {
		return accountHelper()
	}
	if !canShow() {
		fmt.Fprintln(os.Stderr, "the setup needs a terminal; use: hyprtrixie install --yes")
		return 1
	}
	if lock.Busy() {
		fmt.Fprintln(os.Stderr, "another hyprtrixie run is still going")
		return 1
	}
	flow := newWizard(config.Read(config.Path()))
	flow.show = canShow
	flow.nvidia = detect.ReadHardware().HasGPU("nvidia") || detect.NvidiaOnBus()
	flow.stages = flow.allStages(run.Lines(detect.Summary()), apps.All())
	if _, err := tea.NewProgram(flow, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, line := range flow.end.lines {
		fmt.Println(line)
	}
	if flow.install.running() {
		return stoppedCode
	}
	return flow.exit
}
