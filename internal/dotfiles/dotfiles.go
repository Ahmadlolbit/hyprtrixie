package dotfiles

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	configMode = 0o600
	folderMode = 0o700
	syncBack   = "[git]\n    autoCommit = true\n"
	pushedBack = "[git]\n    autoCommit = true\n    autoPush = true\n"
	ConfigName = "chezmoi.toml"
)

var ErrNoChezmoi = errors.New("chezmoi isn't installed: sudo hyprtrixie app install chezmoi")

func folder() string {
	return filepath.Join(env.ConfigHome(), "chezmoi")
}

func Config() string {
	return filepath.Join(folder(), ConfigName)
}

func Unpushed(text string) (string, bool) {
	if text != pushedBack {
		return text, false
	}
	return syncBack, true
}

func stopPushing() (bool, error) {
	data, err := os.ReadFile(Config())
	if err != nil {
		return false, nil
	}
	text, changed := Unpushed(string(data))
	if !changed {
		return false, nil
	}
	return true, atomic.Install(Config(), []byte(text), configMode)
}

func Ready() error {
	if _, err := exec.LookPath("chezmoi"); err != nil {
		return ErrNoChezmoi
	}
	return nil
}

func SyncBack() (bool, error) {
	yours, err := filepath.Glob(filepath.Join(folder(), "chezmoi.*"))
	if err != nil {
		return false, err
	}
	if len(yours) > 0 {
		return stopPushing()
	}
	if err := os.MkdirAll(folder(), folderMode); err != nil {
		return false, err
	}
	return true, atomic.Install(Config(), []byte(syncBack), configMode)
}

func Init(repository string) int {
	return run.Status("chezmoi", "init", repository)
}

func Apply() int {
	return run.Status("chezmoi", "apply")
}

func Changes() int {
	return run.Status("chezmoi", "status")
}
