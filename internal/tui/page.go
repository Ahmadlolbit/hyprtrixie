package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Ahmadlolbit/hyprtrixie/internal/words"
)

const (
	labelWidth  = 34
	markColumns = 3
	infoIndent  = 2
	pageJump    = 10
)

const (
	emptyRefused     = "That row can't be empty, so it keeps what it had"
	lineBreakRefused = "A row can't take a tab or a line break, so the paste isn't taken"
)

type field struct {
	allow   func(rune) bool
	valid   func(string) bool
	set     func(string)
	refused string
	small   bool
	replace bool
	secret  bool
}

func (input *field) folded(key tea.KeyMsg) tea.KeyMsg {
	if input.small {
		key.Runes = []rune(strings.ToLower(string(key.Runes)))
	}
	return key
}

type row struct {
	label   string
	value   string
	mark    string
	help    string
	heading bool
	info    bool
	chosen  bool
	input   *field
	space   func() tea.Cmd
	enter   func() tea.Cmd
}

func (line row) refusing(why string) row {
	line.input.refused = why
	return line
}

func (line row) lowered() row {
	line.input.small = true
	return line
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
	ready   func() string
}

func (view *page) base() *page {
	return view
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
	for index, line := range view.shown() {
		if line.chosen {
			view.cursor = index
		}
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
	if _, found := view.current(); !found {
		view.step(1)
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

func (view *page) leaving() (string, func() tea.Cmd) {
	return leaveTitle, func() tea.Cmd { return tea.Quit }
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
	switch key.Type {
	case tea.KeyEnter:
		view.search = nil
	case tea.KeyEsc:
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
	if !typed {
		return false
	}
	key = line.input.folded(key)
	if !view.accepts(line, key) {
		return view.refuse(line, key)
	}
	if view.draft == nil {
		view.draft = &input{allow: line.input.allow, secret: line.input.secret}
		if !line.input.secret {
			view.draft.text = line.value
		}
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

func (view *page) refuse(line row, key tea.KeyMsg) bool {
	if line.input.replace && pageKey(key) {
		return false
	}
	view.wizard.notice = line.input.refused
	if slices.ContainsFunc(key.Runes, unicode.IsControl) {
		view.wizard.notice = lineBreakRefused
	}
	return true
}

func pageKey(key tea.KeyMsg) bool {
	switch key.String() {
	case " ", "j", "k", "/":
		return true
	}
	return false
}

func (view *page) commit() {
	line, found := view.current()
	if view.draft == nil || !found || line.input == nil {
		view.draft = nil
		return
	}
	switch text := view.draft.text; {
	case line.input.valid(text):
		line.input.set(text)
	case text == "":
		view.wizard.notice = emptyRefused
	default:
		view.wizard.notice = line.input.refused
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

func (view *page) text(line row, focused bool, width int) string {
	if line.heading {
		return headingStyle.Render(words.T(line.label))
	}
	if line.info {
		return "  " + words.T(line.label)
	}
	label := strings.TrimSpace(line.mark + " " + words.T(line.label))
	if focused && view.draft != nil {
		label = fmt.Sprintf("%-*s ", labelWidth, label)
		return marked(focused, label+tail(view.draft.shown(), width-lipgloss.Width(label)-markColumns))
	}
	if line.value != "" {
		label = fmt.Sprintf("%-*s %s", labelWidth, label, line.value)
	}
	return marked(focused, label)
}

func (view *page) lines(line row, focused bool, width int) []string {
	text := view.text(line, focused, width)
	if !line.info || width <= infoIndent || lipgloss.Width(text) <= width {
		return []string{text}
	}
	wrapped := strings.Split(lipgloss.NewStyle().Width(width-infoIndent).Render(words.T(line.label)), "\n")
	for index, piece := range wrapped {
		wrapped[index] = strings.Repeat(" ", infoIndent) + strings.TrimRight(piece, " ")
	}
	return wrapped
}

func (view *page) draw(width, height int) string {
	text, _ := view.render(width, height)
	return text
}

func (view *page) render(width, height int) (string, bool) {
	rows := view.shown()
	spans := make([]int, len(rows))
	wrapped := false
	for index, line := range rows {
		spans[index] = 1
		if line.info && len(words.T(line.label))+infoIndent > width {
			spans[index] = len(view.lines(line, false, width))
			wrapped = wrapped || spans[index] > 1
		}
	}
	view.scroll(height, spans)
	lines := make([]string, 0, height)
	for index := view.top; index < len(rows) && len(lines) < height; index++ {
		if spans[index] == 1 {
			lines = append(lines, view.text(rows[index], index == view.cursor, width))
			continue
		}
		lines = append(lines, view.lines(rows[index], index == view.cursor, width)...)
	}
	return strings.Join(lines[:min(len(lines), height)], "\n"), wrapped
}

func (view *page) scroll(height int, spans []int) {
	if len(spans) == 0 {
		view.top = 0
		return
	}
	cursor := min(view.cursor, len(spans)-1)
	view.top = min(view.top, cursor)
	for view.top < cursor && total(spans[view.top:cursor+1]) > height {
		view.top++
	}
	for view.top > 0 && total(spans[view.top-1:]) <= height {
		view.top--
	}
}

func total(spans []int) int {
	sum := 0
	for _, span := range spans {
		sum += span
	}
	return sum
}

func (view *page) help() string {
	if view.search != nil {
		return words.T("Search: ") + view.search.shown()
	}
	if view.filter != "" {
		return words.T("Search: ") + view.filter
	}
	if line, found := view.current(); found {
		return words.T(line.help)
	}
	return ""
}

func (view *page) keys() string {
	if view.search != nil {
		return "Type to search · Enter keeps it · Esc clears it"
	}
	if view.typing() {
		return "Type to change · Arrows move · Enter goes on · Esc back"
	}
	return "Arrows move · Space changes · Enter goes on · Esc back · / search · ? keys"
}
