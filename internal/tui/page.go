package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	labelWidth = 34
	pageJump   = 10
)

type field struct {
	allow   func(rune) bool
	valid   func(string) bool
	set     func(string)
	replace bool
}

type row struct {
	label   string
	value   string
	mark    string
	help    string
	heading bool
	info    bool
	input   *field
	space   func() tea.Cmd
	enter   func() tea.Cmd
}

func (line row) selectable() bool {
	return !line.heading && !line.info
}

func (line row) matches(filter string) bool {
	text := strings.ToLower(line.label + " " + line.help)
	return line.selectable() && strings.Contains(text, strings.ToLower(filter))
}

type page struct {
	wizard  *wizard
	title   string
	rows    func() []row
	cursor  int
	top     int
	search  *input
	filter  string
	draft   *input
	replace bool
	atEnd   bool
}

func (view *page) name() string {
	return view.title
}

func (view *page) arrive() {
	view.cursor = 0
	view.top = 0
	view.search = nil
	view.filter = ""
	view.draft = nil
	view.settle()
	if view.atEnd {
		view.move(1, len(view.shown()))
	}
}

func (view *page) shown() []row {
	rows := view.rows()
	if view.filter == "" {
		return rows
	}
	var found []row
	for _, line := range rows {
		if line.matches(view.filter) {
			found = append(found, line)
		}
	}
	return found
}

func (view *page) current() (row, bool) {
	rows := view.shown()
	if view.cursor >= len(rows) || !rows[view.cursor].selectable() {
		return row{}, false
	}
	return rows[view.cursor], true
}

func (view *page) settle() {
	rows := view.shown()
	view.cursor = min(view.cursor, max(len(rows)-1, 0))
	if _, found := view.current(); !found {
		view.step(1)
		view.step(-1)
	}
}

func (view *page) step(delta int) bool {
	rows := view.shown()
	for at := view.cursor + delta; at >= 0 && at < len(rows); at += delta {
		if rows[at].selectable() {
			view.cursor = at
			return true
		}
	}
	return false
}

func (view *page) move(delta, count int) {
	view.commit()
	for done := 0; done < count && view.step(delta); done++ {
	}
}

func (view *page) typing() bool {
	line, found := view.current()
	textField := found && line.input != nil && !line.input.replace
	return view.search != nil || textField
}

func (view *page) press(key tea.KeyMsg) tea.Cmd {
	if view.search != nil {
		return view.pressSearch(key)
	}
	if view.pressField(key) {
		return nil
	}
	if view.pressMove(key.String()) {
		return nil
	}
	return view.pressAction(key.String())
}

func (view *page) pressSearch(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "enter":
		view.search = nil
	case "esc":
		view.search = nil
		view.filter = ""
	default:
		view.search.press(key)
		view.filter = view.search.text
	}
	view.cursor = 0
	view.settle()
	return nil
}

func (view *page) pressField(key tea.KeyMsg) bool {
	line, found := view.current()
	if !found || line.input == nil {
		return false
	}
	typed := key.Type == tea.KeyRunes || key.Type == tea.KeySpace || key.Type == tea.KeyBackspace
	if !typed || !view.accepts(line, key) {
		return false
	}
	if view.draft == nil {
		view.draft = &input{text: line.value, allow: line.input.allow}
		view.replace = line.input.replace
	}
	if view.replace && key.Type != tea.KeyBackspace {
		view.draft.text = ""
	}
	view.replace = false
	return view.draft.press(key)
}

func (view *page) accepts(line row, key tea.KeyMsg) bool {
	if key.Type == tea.KeyBackspace {
		return true
	}
	for _, character := range key.Runes {
		if !line.input.allow(character) {
			return false
		}
	}
	return !line.input.replace || len(key.Runes) > 0
}

func (view *page) commit() {
	line, found := view.current()
	if view.draft == nil || !found || line.input == nil {
		view.draft = nil
		return
	}
	if line.input.valid(view.draft.text) {
		line.input.set(view.draft.text)
	}
	view.draft = nil
}

func (view *page) pressMove(name string) bool {
	switch name {
	case "up", "k", "shift+tab":
		view.move(-1, 1)
	case "down", "j", "tab":
		view.move(1, 1)
	case "pgup":
		view.move(-1, pageJump)
	case "pgdown":
		view.move(1, pageJump)
	case "home":
		view.move(-1, len(view.shown()))
	case "end":
		view.move(1, len(view.shown()))
	default:
		return false
	}
	return true
}

func (view *page) pressAction(name string) tea.Cmd {
	switch name {
	case " ":
		if line, found := view.current(); found && line.space != nil {
			return line.space()
		}
	case "enter":
		return view.pressEnter()
	case "esc":
		return view.pressEscape()
	case "/":
		view.commit()
		view.search = &input{text: view.filter, allow: anything}
	}
	return nil
}

func (view *page) pressEnter() tea.Cmd {
	view.commit()
	if line, found := view.current(); found && line.enter != nil {
		return line.enter()
	}
	return view.wizard.forward()
}

func (view *page) pressEscape() tea.Cmd {
	view.commit()
	if view.filter != "" {
		view.filter = ""
		view.cursor = 0
		view.settle()
		return nil
	}
	return view.wizard.backward()
}

func (view *page) text(line row, focused bool) string {
	if line.heading {
		return headingStyle.Render(line.label)
	}
	if line.info {
		return "  " + line.label
	}
	label := strings.TrimSpace(line.mark + " " + line.label)
	value := line.value
	if focused && view.draft != nil {
		value = view.draft.shown()
	}
	if value != "" {
		label = fmt.Sprintf("%-*s %s", labelWidth, label, valueStyle.Render(value))
	}
	return marked(focused, label)
}

func (view *page) draw(_, height int) string {
	rows := view.shown()
	view.scroll(height, len(rows))
	lines := make([]string, 0, height)
	for index := view.top; index < min(view.top+height, len(rows)); index++ {
		lines = append(lines, view.text(rows[index], index == view.cursor))
	}
	return strings.Join(lines, "\n")
}

func (view *page) scroll(height, total int) {
	if view.cursor < view.top {
		view.top = view.cursor
	}
	if view.cursor >= view.top+height {
		view.top = view.cursor - height + 1
	}
	view.top = max(min(view.top, total-height), 0)
}

func (view *page) help() string {
	if view.search != nil {
		return "Search: " + view.search.shown()
	}
	if view.filter != "" {
		return "Search: " + view.filter
	}
	if line, found := view.current(); found {
		return line.help
	}
	return ""
}

func (view *page) keys() string {
	if view.search != nil {
		return "Type to search · Enter keeps it · Esc clears it"
	}
	return "Arrows move · Space changes · Enter goes on · Esc back · / search · ? keys"
}
