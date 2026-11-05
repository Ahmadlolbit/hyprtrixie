package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Ahmadlolbit/hyprtrixie/internal/events"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

const (
	width      = 64
	titleWidth = width - 16
	minFill    = 3

	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	red    = "\x1b[31m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	blue   = "\x1b[34m"
	accent = "\x1b[35m"
)

var (
	writeFailed bool
	stream      *events.Stream
)

func Attach(to *events.Stream) {
	stream = to
}

func send(kind, text string, items ...string) bool {
	if stream == nil {
		return false
	}
	stream.Send(events.Event{Kind: kind, Text: text, Items: items})
	return true
}

func ask(kind, text string, items ...string) (string, bool) {
	return stream.Ask(events.Event{Kind: kind, Text: text, Items: items})
}

func Mirror(data []byte) {
	send(events.Log, string(data))
}

func Print(text string) {
	if send(events.Text, text) {
		return
	}
	if _, err := os.Stdout.WriteString(text); err != nil && !writeFailed {
		writeFailed = true
		fmt.Fprintln(os.Stderr, err)
	}
}

func ExitCode() int {
	if writeFailed {
		return 1
	}
	return 0
}

func paint(style, text string) string {
	if term.IsTerminal(os.Stdout) {
		return style + text + reset
	}
	return text
}

func Header(title, line string) {
	Print("\n" + paint(bold, title) + "\n" + line + "\n")
}

func Rule(text string) {
	if send(events.Rule, text) {
		return
	}
	fill := max(width-utf8.RuneCountInString(text)-4, minFill)
	Print("\n" + paint(accent+bold, "── "+text+" "+strings.Repeat("─", fill)) + "\n")
}

func status(kind, colour, tag, text string) {
	if send(kind, text) {
		return
	}
	Print("  " + paint(colour, "["+tag+"]") + " " + text + "\n")
}

func OK(text string) {
	status(events.OK, green, " ok ", text)
}

func Fail(text string) {
	status(events.Fail, red, "FAIL", text)
}

func Skip(text string) {
	status(events.Skip, yellow, "skip", text)
}

func Warn(text string) {
	status(events.Warn, yellow, "warn", text)
}

func Busy(text string) {
	if !send(events.Busy, text) && term.IsTerminal(os.Stdout) {
		Print("  " + paint(blue, "[ .. ]") + " " + text)
	}
}

func Done() {
	if !send(events.Done, "") && term.IsTerminal(os.Stdout) {
		Print("\r\x1b[K")
	}
}

func ModuleResult(id, title, result string) {
	if send(events.Result, title, id, result) {
		return
	}
	colour := yellow
	switch result {
	case "PASS":
		colour = green
	case "FAIL":
		colour = red
	}
	Print(fmt.Sprintf("  %s  %-*s %s\n", id, titleWidth, title, paint(colour, result)))
}

func Page(text string) {
	if text == "" || send(events.Page, text) {
		return
	}
	if term.IsTerminal(os.Stdout) {
		run.Page(text)
		return
	}
	Print(text + "\n")
}

func Note(text string) {
	if !send(events.Note, text) {
		Print("         " + paint(dim, text) + "\n")
	}
}

func CanAsk() bool {
	return stream != nil || term.IsTerminal(os.Stdin) && term.IsTerminal(os.Stdout)
}

func Confirm(prompt string) bool {
	if !CanAsk() {
		return false
	}
	answer, err := term.ReadAnswer(prompt + " [y/N] ")
	return err == nil && clearYes(answer)
}

func Proceed(prompt string) bool {
	answer, err := term.ReadAnswer(prompt + " [Y/n] ")
	return err == nil && !clearNo(answer)
}

func Input(header string) (string, bool) {
	if stream != nil {
		answer, ok := ask(events.Input, header)
		return strings.TrimSpace(answer), ok
	}
	answer, err := term.ReadAnswer(header + " ")
	return strings.TrimSpace(answer), err == nil
}

func Password(header string) (string, bool) {
	if stream != nil {
		return ask(events.Password, header)
	}
	answer, err := term.ReadSecret(header + " ")
	return answer, err == nil
}

func Choose(header string, items []string) (string, bool) {
	if stream != nil {
		return ask(events.Choose, header, items...)
	}
	for {
		Print(header + "\n")
		for index, item := range items {
			Print(fmt.Sprintf("  %2d) %s\n", index+1, item))
		}
		answer, err := term.ReadAnswer("> ")
		if err != nil {
			return "", false
		}
		number, err := strconv.Atoi(strings.TrimSpace(answer))
		if err == nil && number >= 1 && number <= len(items) {
			return items[number-1], true
		}
	}
}

func clearYes(answer string) bool {
	switch strings.ToLower(strings.Trim(answer, " \t")) {
	case "y", "yes":
		return true
	}
	return false
}

func clearNo(answer string) bool {
	switch strings.ToLower(strings.Trim(answer, " \t")) {
	case "n", "no":
		return true
	}
	return false
}
