package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	neovimScheme = "hyprtrixie"
	neovimReload = `execute("if get(g:, 'colors_name', '') ==# 'hyprtrixie' | colorscheme hyprtrixie | endif")`
)

func linkNeovim(current string) {
	if file := filepath.Join(current, "nvim.lua"); fsutil.IsFile(file) {
		symlink(file, filepath.Join(env.DataHome(), "nvim", "site", "colors", neovimScheme+".lua"))
	}
}

func neovimSockets() []string {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		return nil
	}
	found, _ := filepath.Glob(filepath.Join(runtime, "nvim.*"))
	var sockets []string
	for _, path := range found {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			sockets = append(sockets, path)
		}
	}
	return sockets
}

func reloadNeovim() {
	if _, err := exec.LookPath("nvim"); err != nil {
		return
	}
	for _, socket := range neovimSockets() {
		detached("timeout", "5", "nvim", "--server", socket, "--remote-expr", neovimReload)
	}
}
