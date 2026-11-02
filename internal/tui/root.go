package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/privilege"
)

const noSudoNotice = "This account can't use sudo. Log in as root and run ./hyprtrixie once: it sets the account up."

type unlocked struct {
	err  error
	then func() tea.Cmd
}

func (flow *wizard) withRoot(then func() tea.Cmd) tea.Cmd {
	if flow.ready() {
		return then()
	}
	return flow.open(flow.passwordPrompt(then))
}

func (flow *wizard) passwordPrompt(then func() tea.Cmd) *dialog {
	box := prompt("Your password, for sudo", "", func(text string, ok bool) tea.Cmd {
		if !ok {
			return nil
		}
		flow.waiting = true
		return func() tea.Msg { return unlocked{flow.unlock(text), then} }
	})
	box.field.secret = true
	return box
}

func (flow *wizard) afterUnlock(message unlocked) tea.Cmd {
	flow.waiting = false
	switch {
	case message.err == nil:
		return message.then()
	case errors.Is(message.err, privilege.ErrNoSudo):
		flow.notice = noSudoNotice
		return nil
	}
	box := flow.passwordPrompt(message.then)
	box.lines = []string{message.err.Error()}
	return flow.open(box)
}
