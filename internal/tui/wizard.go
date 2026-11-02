package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
	frameLines    = 5
	summaryParts  = 3

	profileAt  = 1
	appsAt     = 2
	settingsAt = 3
)

var reviewAt = settingsAt + len(settingsSections)

var keyGuide = []string{
	"Arrows or j and k   Move",
	"Tab, Shift+Tab      Move between rows",
	"Space               Change the row",
	"Enter               Go on",
	"Esc                 Go back",
	"/                   Search",
	"?                   These keys",
	"Ctrl+C              Leave, after asking",
	"",
	"Any key closes this.",
}

type stage interface {
	name() string
	arrive()
	press(tea.KeyMsg) tea.Cmd
	draw(width, height int) string
	help() string
	keys() string
	typing() bool
}

type wizard struct {
	settings  *config.Config
	nvidia    bool
	stages    []stage
	at        int
	width     int
	height    int
	dialogs   []*dialog
	guide     bool
	notice    string
	exit      int
	reviewing bool
}

func newWizard(settings *config.Config) *wizard {
	return &wizard{settings: settings, width: defaultWidth, height: defaultHeight}
}

func (flow *wizard) Init() tea.Cmd {
	flow.stages[flow.at].arrive()
	return nil
}

func (flow *wizard) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		flow.width = message.Width
		flow.height = message.Height
	case tea.KeyMsg:
		return flow, flow.press(message)
	}
	return flow, nil
}

func (flow *wizard) press(key tea.KeyMsg) tea.Cmd {
	flow.notice = ""
	if key.String() == "ctrl+c" {
		return flow.interrupt()
	}
	if flow.guide {
		flow.guide = false
		return nil
	}
	if len(flow.dialogs) > 0 {
		return flow.pressDialog(key)
	}
	current := flow.stages[flow.at]
	if key.String() == "?" && !current.typing() {
		flow.guide = true
		return nil
	}
	return current.press(key)
}

func (flow *wizard) pressDialog(key tea.KeyMsg) tea.Cmd {
	top := flow.dialogs[len(flow.dialogs)-1]
	if done, cmd := top.press(key); done {
		flow.close(top)
		return cmd
	}
	return nil
}

func (flow *wizard) open(box *dialog) tea.Cmd {
	flow.dialogs = append(flow.dialogs, box)
	return nil
}

func (flow *wizard) close(box *dialog) {
	for index, open := range flow.dialogs {
		if open == box {
			flow.dialogs = append(flow.dialogs[:index], flow.dialogs[index+1:]...)
			return
		}
	}
}

func (flow *wizard) interrupt() tea.Cmd {
	if len(flow.dialogs) > 0 && flow.dialogs[len(flow.dialogs)-1].title == leaveTitle {
		return tea.Quit
	}
	return flow.open(confirmation(leaveTitle, func() tea.Cmd { return tea.Quit }))
}

func (flow *wizard) backToReview() bool {
	if !flow.reviewing {
		return false
	}
	flow.reviewing = false
	flow.jump(reviewAt)
	return true
}

func (flow *wizard) forward() tea.Cmd {
	if !flow.backToReview() && flow.at < len(flow.stages)-1 {
		flow.jump(flow.at + 1)
	}
	return nil
}

func (flow *wizard) backward() tea.Cmd {
	if !flow.backToReview() && flow.at > 0 {
		flow.jump(flow.at - 1)
	}
	return nil
}

func (flow *wizard) jump(index int) {
	flow.at = index
	flow.stages[index].arrive()
}

func (flow *wizard) body(width, height int) string {
	switch {
	case flow.guide:
		return strings.Join(keyGuide, "\n")
	case len(flow.dialogs) > 0:
		return flow.dialogs[len(flow.dialogs)-1].draw(height)
	}
	return flow.stages[flow.at].draw(width, height)
}

func (flow *wizard) footer() (string, string) {
	if flow.guide {
		return "", "Any key closes this"
	}
	if len(flow.dialogs) > 0 {
		return "", flow.dialogs[len(flow.dialogs)-1].keys()
	}
	current := flow.stages[flow.at]
	if flow.notice != "" {
		return flow.notice, current.keys()
	}
	return current.help(), current.keys()
}

func (flow *wizard) header(width int) string {
	title := "hyprtrixie " + env.Version
	where := fmt.Sprintf("%s · %d of %d", flow.stages[flow.at].name(), flow.at+1, len(flow.stages))
	gap := max(width-lipgloss.Width(title)-lipgloss.Width(where), 1)
	return headingStyle.Render(title) + strings.Repeat(" ", gap) + where
}

func (flow *wizard) View() string {
	width, height := max(flow.width, 1), max(flow.height-frameLines, 1)
	rule := ruleStyle.Render(strings.Repeat("─", width))
	help, keys := flow.footer()
	body := lipgloss.NewStyle().MaxWidth(width).Height(height).MaxHeight(height).Render(flow.body(width, height))
	lines := []string{cut(flow.header(width), width), rule, body, rule, cut(dimStyle.Render(help), width), cut(dimStyle.Render(keys), width)}
	return strings.Join(lines, "\n")
}
