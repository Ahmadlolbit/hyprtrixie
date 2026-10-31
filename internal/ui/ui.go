package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

const (
	width = 64

	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	red    = "\x1b[31m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
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
	Print("\n" + paint(accent+bold, "── "+text+" "+strings.Repeat("─", width-len(text)-4)) + "\n")
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

func clearYes(answer string) bool {
	switch strings.ToLower(strings.Trim(answer, " \t")) {
	case "y", "yes":
		return true
	}
	return false
}
