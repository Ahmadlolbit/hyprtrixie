package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/look"
)

func shellFile() string {
	return filepath.Join(env.ConfigHome(), "hyprtrixie", "shell.json")
}

func barValue(path string, read func([]byte) string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return read(data)
}

func showBar(read func([]byte) string, fallback string) int {
	value := barValue(shellFile(), read)
	if value == "" {
		value = barValue(env.Or("HYPRTRIXIE_SHELL_SYSTEM", "/etc/hyprtrixie/shell.json"), read)
	}
	if value == "" {
		value = fallback
	}
	fmt.Println(value)
	return 0
}

func setBar(change func([]byte, string) ([]byte, error), value string) int {
	file := shellFile()
	if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	updated, err := change(data, value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s is not valid JSON; fix it first\n", file)
		return 1
	}
	if err := atomic.Install(file, updated, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func barLayout(args []string) int {
	switch value := arg(args, 0); value {
	case "":
		return showBar(look.Preset, "full")
	case "full", "floating", "minimal", "vertical":
		return setBar(look.WithPreset, value)
	}
	fmt.Fprintln(os.Stderr, "bar takes full, floating, minimal or vertical")
	return 2
}

func barPosition(args []string) int {
	switch value := arg(args, 0); value {
	case "":
		return showBar(look.Position, "top")
	case "top", "bottom":
		return setBar(look.WithPosition, value)
	}
	fmt.Fprintln(os.Stderr, "position takes top or bottom")
	return 2
}
