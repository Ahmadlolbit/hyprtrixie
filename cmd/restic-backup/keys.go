package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/restic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

func keysFile() string {
	return filepath.Join(configDir(), "env")
}

func savedKeys() string {
	data, err := os.ReadFile(keysFile())
	if err != nil {
		return ""
	}
	return string(data)
}

func loadKeys() {
	for _, key := range restic.Keys(savedKeys()) {
		if err := os.Setenv(key[0], key[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func askKeys(prompts []restic.Prompt) (string, bool) {
	var typed strings.Builder
	for _, prompt := range prompts {
		value, err := term.ReadSecret(prompt.Label + ": ")
		if err != nil || value == "" {
			return "", false
		}
		typed.WriteString(prompt.Name + "=" + value + "\n")
	}
	return typed.String(), true
}

func typeKeys(location string) int {
	prompts := restic.Prompts(location)
	if len(prompts) == 0 {
		fmt.Println("This location needs no keys.")
		return 0
	}
	typed, ok := askKeys(prompts)
	if !ok {
		fmt.Fprintln(os.Stderr, "The keys were not changed.")
		return 1
	}
	if err := atomic.Install(keysFile(), []byte(typed), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func keys() int {
	if !useRepository() {
		return 1
	}
	return typeKeys(os.Getenv("RESTIC_REPOSITORY"))
}

func keysReady(location string) bool {
	if strings.HasPrefix(location, "rclone:") {
		return rcloneReady()
	}
	return restic.HasKeys(savedKeys(), location) || typeKeys(location) == 0
}

func rcloneReady() bool {
	if _, err := exec.LookPath("rclone"); err != nil {
		fmt.Fprintln(os.Stderr, "rclone is not installed: sudo apt install rclone, then rclone config.")
		return false
	}
	return true
}
