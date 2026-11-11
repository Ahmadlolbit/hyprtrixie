package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const screenReader = "orca"

func readerRunning() bool {
	return run.Quiet(run.Quick, "pgrep", "-x", screenReader) == 0
}

func readerSaid(text string) {
	notify.Post([]string{"--app-name=Screen reader"}, text, "")
}

func readerOn() int {
	return startReader(readerRunning)
}

func stopped() bool {
	return false
}

func startReader(running func() bool) int {
	if _, err := exec.LookPath(screenReader); err != nil {
		fmt.Fprintln(os.Stderr, "The screen reader isn't installed: turn it on in Setup, under Look and feel.")
		readerSaid("Not installed: turn it on in Setup, under Look and feel")
		return 1
	}
	if !put("reader", "yes") {
		return 1
	}
	if !running() {
		run.Quiet(run.Quick, "setsid", "-f", "hyprtrixie-start", screenReader)
	}
	readerSaid("Screen reader on")
	return 0
}

func readerOff() int {
	if !put("reader", "no") {
		return 1
	}
	run.Within(run.Quick, "pkill", "-x", screenReader)
	readerSaid("Screen reader off")
	return 0
}

func reader(args []string) int {
	if strings.ToLower(arg(args, 0)) != "toggle" {
		return onOff{"reader", readerOn, readerOff}.run(args)
	}
	if readerRunning() {
		return readerOff()
	}
	return startReader(stopped)
}
