package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/events"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	sideBySide = 100
	stepsWidth = 44
	batchSize  = 256
	logLines   = 2000
	titleWidth = 48
)

var (
	sequences = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b.`)

	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	failStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	busyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))

	marks = map[string]string{
		events.OK:   okStyle.Render("[ ok ]"),
		events.Fail: failStyle.Render("[FAIL]"),
		events.Skip: warnStyle.Render("[skip]"),
		events.Warn: warnStyle.Render("[warn]"),
	}
)

type arrived []events.Event

type ended struct {
	code int
	text string
}

type installer struct {
	flow    *wizard
	child   *run.Child
	stream  <-chan events.Event
	path    string
	steps   []string
	busy    string
	log     []string
	partial string
	closing []string
	summing bool
}

var handlers = map[string]func(*installer, events.Event){
	events.Rule:     (*installer).rule,
	events.Busy:     func(work *installer, event events.Event) { work.busy = event.Text },
	events.Done:     func(work *installer, _ events.Event) { work.busy = "" },
	events.OK:       (*installer).status,
	events.Fail:     (*installer).status,
	events.Skip:     (*installer).status,
	events.Warn:     (*installer).status,
	events.Note:     (*installer).note,
	events.Log:      (*installer).logged,
	events.Text:     (*installer).text,
	events.Result:   (*installer).result,
	events.Page:     (*installer).page,
	events.Choose:   (*installer).ask,
	events.Input:    (*installer).ask,
	events.Password: (*installer).ask,
}

func clean(line string) string {
	if at := strings.LastIndexByte(strings.TrimRight(line, "\r"), '\r'); at >= 0 {
		line = line[at+1:]
	}
	line = sequences.ReplaceAllString(line, "")
	return strings.Map(func(character rune) rune {
		switch {
		case character == '\t':
			return ' '
		case character < ' ' || character == 0x7f:
			return -1
		}
		return character
	}, line)
}

func (work *installer) name() string {
	return "Install"
}

func (work *installer) arrive() {}

func (work *installer) press(tea.KeyMsg) tea.Cmd {
	return nil
}

func (work *installer) help() string {
	return ""
}

func (work *installer) keys() string {
	return "The setup is running · Ctrl+C stops it"
}

func (work *installer) typing() bool {
	return false
}

func (work *installer) leaving() (string, func() tea.Cmd) {
	return "Stop the setup?", work.stop
}

func (work *installer) stop() tea.Cmd {
	events.Stop(work.child.Input)
	work.flow.dialogs = nil
	return nil
}

func (work *installer) environment() []string {
	return []string{"HYPRTRIXIE_ENGINE=1", "HYPRTRIXIE_ROOT=" + env.Root()}
}

func (work *installer) arguments() []string {
	args := []string{"install", "--events", "--config", work.path}
	if work.flow.testing {
		args = append(args, "--testing")
	}
	return args
}

func (work *installer) begin() tea.Cmd {
	work.path = filepath.Join(env.StateHome(), "hyprtrixie", "wizard.conf")
	if err := work.flow.settings.Save(work.path); err != nil {
		work.flow.notice = "Could not write the settings: " + err.Error()
		return nil
	}
	core := filepath.Join(env.ToolsDir(), "hyprtrixie-core")
	child, err := run.Spawn(work.environment(), core, work.arguments()...)
	if err != nil {
		work.flow.notice = "Could not start the setup: " + err.Error()
		return nil
	}
	work.child = child
	work.stream = events.Watch(child.Output)
	work.flow.jump(installAt)
	return work.next
}

func (work *installer) next() tea.Msg {
	first, open := <-work.stream
	if !open {
		code, text := work.child.Wait()
		return ended{code, text}
	}
	batch := arrived{first}
	for len(batch) < batchSize {
		select {
		case more, open := <-work.stream:
			if !open {
				return batch
			}
			batch = append(batch, more)
		default:
			return batch
		}
	}
	return batch
}

func (work *installer) take(batch arrived) tea.Cmd {
	for _, event := range batch {
		if handle, known := handlers[event.Kind]; known {
			handle(work, event)
		}
	}
	return work.next
}

func (work *installer) finish(done ended) tea.Cmd {
	os.Remove(work.path)
	if done.text != "" {
		work.closing = append(work.closing, clean(done.text))
	}
	if len(work.closing) == 0 {
		work.closing = []string{"The setup stopped before it finished a module."}
	}
	work.flow.dialogs = nil
	work.flow.asking = nil
	work.flow.exit = done.code
	work.flow.closing = work.closing
	work.flow.end.lines = work.closing
	work.flow.jump(installAt + 1)
	return nil
}

func (work *installer) rule(event events.Event) {
	if event.Text == events.SummaryTitle {
		work.summing = true
		return
	}
	work.steps = append(work.steps, "", headingStyle.Render(clean(event.Text)))
}

func (work *installer) status(event events.Event) {
	line := marks[event.Kind] + " " + clean(event.Text)
	if work.summing {
		work.closing = append(work.closing, line)
		return
	}
	work.steps = append(work.steps, line)
}

func (work *installer) note(event events.Event) {
	work.steps = append(work.steps, dimStyle.Render("       "+clean(event.Text)))
}

func (work *installer) logged(event events.Event) {
	lines := strings.Split(work.partial+event.Text, "\n")
	last := len(lines) - 1
	work.partial = lines[last]
	for _, line := range lines[:last] {
		work.log = append(work.log, clean(line))
	}
	if len(work.log) > logLines {
		work.log = work.log[len(work.log)-logLines:]
	}
}

func (work *installer) text(event events.Event) {
	lines := strings.Split(strings.TrimSuffix(cleanAll(event.Text), "\n"), "\n")
	if work.summing {
		work.closing = append(work.closing, lines...)
		return
	}
	work.steps = append(work.steps, lines...)
}

func cleanAll(text string) string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = clean(line)
	}
	return strings.Join(lines, "\n")
}

func (work *installer) result(event events.Event) {
	if len(event.Items) != 2 {
		return
	}
	style := warnStyle
	switch event.Items[1] {
	case "PASS":
		style = okStyle
	case "FAIL":
		style = failStyle
	}
	line := fmt.Sprintf("  %s  %-*s %s", clean(event.Items[0]), titleWidth, clean(event.Text), style.Render(clean(event.Items[1])))
	work.closing = append(work.closing, line)
}

func (work *installer) page(event events.Event) {
	box := &viewer{lines: strings.Split(cleanAll(event.Text), "\n"), leave: func() tea.Cmd { return nil }}
	work.flow.open(box)
}

func (work *installer) reply(text string, ok bool) tea.Cmd {
	events.Answer(work.child.Input, text, ok)
	return nil
}

func (work *installer) ask(event events.Event) {
	box := &dialog{title: clean(event.Text), locked: true, answer: work.reply}
	switch event.Kind {
	case events.Choose:
		box.items = event.Items
	case events.Password:
		box.field = &input{secret: true, allow: anything}
	default:
		box.field = &input{allow: anything}
	}
	work.flow.openBelow(box)
}

func lastOf(lines []string, count, width int) string {
	first := max(len(lines)-count, 0)
	shown := make([]string, 0, count)
	for _, line := range lines[first:] {
		shown = append(shown, cut(line, width))
	}
	return strings.Join(shown, "\n")
}

func (work *installer) stepLines() []string {
	if work.busy == "" {
		return work.steps
	}
	return append(work.steps[:len(work.steps):len(work.steps)], busyStyle.Render("[ .. ]")+" "+clean(work.busy))
}

func (work *installer) draw(width, height int) string {
	if width >= sideBySide {
		left := lipgloss.NewStyle().Width(stepsWidth).Render(lastOf(work.stepLines(), height, stepsWidth))
		right := dimStyle.Render(lastOf(work.log, height, width-stepsWidth-1))
		return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	}
	lower := height / 2
	upper := height - lower - 1
	rule := ruleStyle.Render(strings.Repeat("─", width))
	return strings.Join([]string{lastOf(work.stepLines(), upper, width), rule, dimStyle.Render(lastOf(work.log, lower, width))}, "\n")
}
