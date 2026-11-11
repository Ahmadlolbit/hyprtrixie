package tui

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type input struct {
	text   string
	secret bool
	allow  func(rune) bool
}

func anything(character rune) bool {
	return character >= ' ' && character != 0x7f
}

func (field *input) press(key tea.KeyMsg) bool {
	switch key.Type {
	case tea.KeyRunes, tea.KeySpace:
		field.insert(key.Runes)
	case tea.KeyBackspace:
		field.erase()
	default:
		return false
	}
	return true
}

func (field *input) insert(characters []rune) {
	var text strings.Builder
	text.Grow(len(field.text) + len(characters))
	text.WriteString(field.text)
	for _, character := range characters {
		if field.allow(character) {
			text.WriteRune(character)
		}
	}
	field.text = text.String()
}

func (field *input) erase() {
	_, size := utf8.DecodeLastRuneInString(field.text)
	field.text = field.text[:len(field.text)-size]
}

func tail(text string, room int) string {
	if len(text) <= room || room < 2 {
		return text
	}
	start, used, kept := len(text), 0, len(text)
	for start > 0 {
		character, size := utf8.DecodeLastRuneInString(text[:start])
		used += lipgloss.Width(string(character))
		if used > room {
			return "…" + text[kept:]
		}
		start -= size
		if used < room {
			kept = start
		}
	}
	return text
}

func (field *input) shown() string {
	if field.secret {
		return strings.Repeat("*", utf8.RuneCountInString(field.text)) + "_"
	}
	return field.text + "_"
}
