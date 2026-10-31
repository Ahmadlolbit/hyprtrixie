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

func presetOf(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return look.Preset(data)
}

func showBar() int {
	preset := presetOf(shellFile())
	if preset == "" {
		preset = presetOf(env.Or("HYPRTRIXIE_SHELL_SYSTEM", "/etc/hyprtrixie/shell.json"))
	}
	if preset == "" {
		preset = "full"
	}
	fmt.Println(preset)
	return 0
}

func setBar(preset string) int {
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
	updated, err := look.WithPreset(data, preset)
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
		return showBar()
	case "full", "floating", "minimal", "vertical":
		return setBar(value)
	}
	fmt.Fprintln(os.Stderr, "bar takes full, floating, minimal or vertical")
	return 2
}
