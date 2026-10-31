package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

const (
	width   = 64
	minFill = 3

	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	red    = "\x1b[31m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	blue   = "\x1b[34m"
	accent = "\x1b[35m"
)

var writeFailed bool

func Print(text string) {
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
	fill := max(width-utf8.RuneCountInString(text)-4, minFill)
	Print("\n" + paint(accent+bold, "── "+text+" "+strings.Repeat("─", fill)) + "\n")
}

func status(colour, tag, text string) {
	Print("  " + paint(colour, "["+tag+"]") + " " + text + "\n")
}

func OK(text string) {
	status(green, " ok ", text)
}

func Fail(text string) {
	status(red, "FAIL", text)
}

func Skip(text string) {
	status(yellow, "skip", text)
}

func Warn(text string) {
	status(yellow, "warn", text)
}

func Busy(text string) {
	if term.IsTerminal(os.Stdout) {
		Print("  " + paint(blue, "[ .. ]") + " " + text)
	}
}

func Done() {
	if term.IsTerminal(os.Stdout) {
		Print("\r\x1b[K")
	}
}

func Page(text string) {
	if text == "" {
		return
	}
	if term.IsTerminal(os.Stdout) {
		run.Page(text)
		return
	}
	Print(text + "\n")
}

func Note(text string) {
	Print("         " + paint(dim, text) + "\n")
}

func CanAsk() bool {
	return term.IsTerminal(os.Stdin) && term.IsTerminal(os.Stdout)
}

func Confirm(prompt string) bool {
	if !CanAsk() {
		return false
	}
	answer, err := term.ReadAnswer(prompt + " [y/N] ")
	return err == nil && clearYes(answer)
}

func Choose(header string, items []string) (string, bool) {
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
