package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/privilege"
	"github.com/Ahmadlolbit/hyprtrixie/internal/words"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
	frameLines    = 5
	summaryParts  = 3
	terminalCheck = 5 * time.Second

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

type checked struct{}

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
	draw(width, height int) string
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
	show      func() bool
	end       *ending
	waiting   bool
	ready     func() bool
	unlock    func(string) error
	offset    int
	system    *system
}

func newWizard(settings *config.Config) *wizard {
	return &wizard{settings: settings, width: defaultWidth, height: defaultHeight, ready: privilege.Ready, unlock: privilege.Unlock}
}

func (flow *wizard) watchTerminal() tea.Cmd {
	if flow.show == nil {
		return nil
	}
	return tea.Tick(terminalCheck, func(time.Time) tea.Msg { return checked{} })
}

func (flow *wizard) Init() tea.Cmd {
	flow.stages[flow.at].arrive()
	return flow.watchTerminal()
}

func (flow *wizard) checkTerminal() tea.Cmd {
	if !flow.show() {
		return tea.Quit
	}
	return flow.watchTerminal()
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
	case checked:
		return flow, flow.checkTerminal()
	case unlocked:
		return flow, flow.afterUnlock(message)
	}
	return flow, nil
}

func (flow *wizard) press(key tea.KeyMsg) tea.Cmd {
	if key.Type != tea.KeyRunes || key.Paste || len(key.Runes) < 2 {
		return flow.pressOne(key)
	}
	commands := make([]tea.Cmd, 0, len(key.Runes))
	for _, character := range key.Runes {
		commands = append(commands, flow.pressOne(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{character}}))
	}
	return tea.Batch(commands...)
}

func (flow *wizard) pressOne(key tea.KeyMsg) tea.Cmd {
	if flow.waiting {
		return nil
	}
	flow.notice = ""
	if key.Type == tea.KeyCtrlC {
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
	flow.guide = false
	title, leave := flow.stages[flow.at].leaving()
	if title == "" {
		return leave()
	}
	if flow.asking != nil {
		flow.close(flow.asking)
		return leave()
	}
	flow.asking = confirmation(title, leave)
	if title == stopTitle {
		flow.asking.cursor = 1
	}
	return flow.open(flow.asking)
}

func (flow *wizard) backToReview() bool {
	if !flow.reviewing {
		return false
	}
	flow.reviewing = false
	flow.jump(reviewAt + flow.offset)
	return true
}

func (flow *wizard) forward() tea.Cmd {
	if gate, ok := flow.stages[flow.at].(*page); ok && gate.ready != nil {
		if why := gate.ready(); why != "" {
			flow.notice = why
			return nil
		}
	}
	if !flow.backToReview() && flow.at+1 < min(len(flow.stages), reviewAt+flow.offset+1) {
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

func translated(lines []string) []string {
	shown := make([]string, len(lines))
	for index, line := range lines {
		shown[index] = words.T(line)
	}
	return shown
}

func (flow *wizard) body(width, height int) string {
	switch {
	case flow.guide:
		return strings.Join(translated(keyGuide), "\n")
	case len(flow.dialogs) > 0:
		return flow.dialogs[len(flow.dialogs)-1].draw(width, height)
	}
	return flow.stages[flow.at].draw(width, height)
}

func (flow *wizard) footer() (string, string) {
	if flow.waiting {
		return "", "Checking your password"
	}
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
	where := fmt.Sprintf(words.T("%s · %d of %d"), words.T(flow.stages[flow.at].name()), flow.at+1, len(flow.stages))
	gap := max(width-lipgloss.Width(title)-lipgloss.Width(where), 1)
	return headingStyle.Render(title) + strings.Repeat(" ", gap) + where
}

func (flow *wizard) View() string {
	width, height := max(flow.width, 1), max(flow.height-frameLines, 1)
	rule := ruleStyle.Render(strings.Repeat("─", width))
	help, keys := flow.footer()
	body := lipgloss.NewStyle().MaxWidth(width).Height(height).MaxHeight(height).Render(flow.body(width, height))
	lines := []string{cut(flow.header(width), width), rule, body, rule, cut(dimStyle.Render(words.T(help)), width), cut(dimStyle.Render(words.T(keys)), width)}
	return strings.Join(lines, "\n")
}
