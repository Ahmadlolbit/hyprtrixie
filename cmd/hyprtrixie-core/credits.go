package main

import (
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

var (
	madeBy = [][2]string{
		{"Hyprtrixie, made by Ahmad", "GPL-3.0"},
		{"hyprtrixie themes", "GPL-3.0"},
	}
	otherCredits = [][2]string{
		{"Walker", "GPL-3.0"}, {"Elephant", "GPL-3.0"}, {"adw-gtk3", "LGPL-2.1"}, {"Bibata cursors", "GPL-3.0"},
		{"JetBrains Mono Nerd Font", "OFL-1.1"}, {"Starship", "ISC"}, {"Catppuccin for SDDM", "MIT"},
		{"Bubble Tea, Lip Gloss and the Charm libraries", "MIT"}, {"Go and golang.org/x", "BSD-3-Clause"},
	}
	debianCredits = []string{"hyprland", "hyprlock", "hypridle", "hyprpaper", "hyprsunset", "xdg-desktop-portal-hyprland",
		"quickshell", "uwsm", "kitty", "mako-notifier", "sddm", "plymouth", "snapper", "btrfs-progs", "restic", "pipewire",
		"wireplumber", "btop", "fastfetch", "zsh", "tmux", "orca"}
)

func copyrightLicense(text string) string {
	everything := false
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "Files:"):
			everything = strings.TrimSpace(strings.TrimPrefix(line, "Files:")) == "*"
		case everything && strings.HasPrefix(line, "License:"):
			return strings.TrimSpace(strings.TrimPrefix(line, "License:"))
		}
	}
	return ""
}

func creditLines() []string {
	var lines []string
	for _, credit := range madeBy {
		lines = append(lines, credit[0]+"\t"+credit[1])
	}
	installed, _ := apt.Installed()
	docs := env.Or("HYPRTRIXIE_DOC_ROOT", "/usr/share/doc")
	for _, name := range debianCredits {
		if license := copyrightLicense(fsutil.Optional(filepath.Join(docs, name, "copyright"))); installed[name] && license != "" {
			lines = append(lines, name+"\t"+license)
		}
	}
	for _, credit := range otherCredits {
		lines = append(lines, credit[0]+"\t"+credit[1])
	}
	return lines
}

func creditsCommand() int {
	return say(strings.Join(creditLines(), "\n") + "\n")
}
