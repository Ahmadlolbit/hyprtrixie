package tui

import (
	"fmt"
	"slices"
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

var (
	reviewAt  = settingsAt + len(settingsSections)
	installAt = reviewAt + 1
)

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
	leaving() (string, func() tea.Cmd)
}

type overlay interface {
	press(tea.KeyMsg) (bool, tea.Cmd)
	draw(height int) string
	keys() string
}

type wizard struct {
	settings  *config.Config
	nvidia    bool
	testing   bool
	stages    []stage
	at        int
	width     int
	height    int
	dialogs   []overlay
	asking    *dialog
	guide     bool
	notice    string
	exit      int
	reviewing bool
	install   *installer
	end       *ending
	closing   []string
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
		if message.Width > 0 && message.Height > 0 {
			flow.width = message.Width
			flow.height = message.Height
		}
	case tea.KeyMsg:
		return flow, flow.press(message)
	case arrived:
		return flow, flow.install.take(message)
	case ended:
		return flow, flow.install.finish(message)
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

func (flow *wizard) open(box overlay) tea.Cmd {
	flow.dialogs = append(flow.dialogs, box)
	return nil
}

func (flow *wizard) openBelow(box overlay) {
	if last := len(flow.dialogs) - 1; last >= 0 {
		if _, reading := flow.dialogs[last].(*viewer); reading {
			flow.dialogs = slices.Insert(flow.dialogs, last, box)
			return
		}
	}
	flow.open(box)
}

func (flow *wizard) close(box overlay) {
	flow.dialogs = slices.DeleteFunc(flow.dialogs, func(open overlay) bool { return open == box })
	if box == overlay(flow.asking) {
		flow.asking = nil
	}
}

func (flow *wizard) interrupt() tea.Cmd {
	title, leave := flow.stages[flow.at].leaving()
	if title == "" {
		return leave()
	}
	if flow.asking != nil {
		flow.close(flow.asking)
		return leave()
	}
	flow.asking = confirmation(title, leave)
	return flow.open(flow.asking)
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
