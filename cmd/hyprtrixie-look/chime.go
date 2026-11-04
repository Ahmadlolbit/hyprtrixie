package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/chime"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

var (
	ownSounds = []string{"startup.ogg", "startup.oga", "startup.wav", "startup.flac", "startup.mp3"}
	players   = []string{"pw-play", "paplay"}
)

func ownSound() string {
	for _, name := range ownSounds {
		if path := filepath.Join(env.AppConfig(), name); fsutil.IsFile(path) {
			return path
		}
	}
	return ""
}

func themeMode() string {
	data, _ := os.ReadFile(filepath.Join(env.ThemeState(), "theme", "colors.toml"))
	colors := theme.Parse(string(data))
	colors.Resolve()
	return colors["mode"]
}

func madeSound() (string, error) {
	mode := themeMode()
	path := filepath.Join(env.CacheHome(), "hyprtrixie", "chime-1-"+mode+".wav")
	if fsutil.IsFile(path) {
		return path, nil
	}
	notes := chime.Dark
	if mode == "light" {
		notes = chime.Light
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return "", err
	}
	return path, atomic.Write(path, chime.WAV(notes), 0o666)
}

func chimePlay() int {
	sound := ownSound()
	if sound == "" {
		made, err := madeSound()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		sound = made
	}
	for _, player := range players {
		if _, err := exec.LookPath(player); err == nil {
			return run.Quiet(run.Minute, player, sound)
		}
	}
	fmt.Fprintln(os.Stderr, "no sound player: install pipewire-bin for pw-play")
	return 1
}

func chimeCommand(args []string) int {
	if strings.ToLower(arg(args, 0)) == "play" {
		return chimePlay()
	}
	return onOff{"chime", putThen("chime", "yes", nothing), putThen("chime", "no", nothing)}.run(args)
}
