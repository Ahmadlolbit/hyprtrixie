package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

const usage = `Usage: hyprtrixie-theme-install <https URL of a theme repository>

Clones a theme into ~/.config/hyprtrixie/themes and applies it. A theme
from a repository can change colours and backgrounds, but not run code.
Installing it again takes its newest version; a folder of the same name
that isn't a clone of that repository stays, and the install stops.
`

func themeName(url string) string {
	name := path.Base(strings.TrimRight(url, "/"))
	name = strings.TrimSuffix(name, ".git")
	name = strings.TrimSuffix(name, "-theme")
	return theme.NameOf(name)
}

func plain(url string) string {
	return strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(url), "/"), ".git")
}

func taken(dir, url string) bool {
	if _, err := os.Lstat(dir); err != nil {
		return false
	}
	if !fsutil.IsDir(filepath.Join(dir, ".git")) {
		return true
	}
	origin, err := run.CaptureQuiet(run.Quick, "git", "-C", dir, "config", "--get", "remote.origin.url")
	return err != nil || plain(origin) != plain(url)
}

func swap(part, dir string) error {
	old := filepath.Join(filepath.Dir(dir), "."+filepath.Base(dir)+".old")
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if err := os.Rename(dir, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(part, dir); err != nil {
		os.Rename(old, dir)
		return err
	}
	return os.RemoveAll(old)
}

func complete(part, dir string) bool {
	name := filepath.Base(dir)
	for _, folder := range []string{part, filepath.Join(env.ThemeRoot(), "themes", name)} {
		if fsutil.IsFile(filepath.Join(folder, "colors.toml")) {
			return true
		}
	}
	fmt.Fprintf(os.Stderr, "theme %s has no colors.toml\n", name)
	return false
}

func install(url, dir string) bool {
	if err := os.MkdirAll(filepath.Dir(dir), 0o777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	part := dir + ".part"
	os.RemoveAll(part)
	defer os.RemoveAll(part)
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
	if run.Status("git", "-c", "http.lowSpeedLimit=1000", "-c", "http.lowSpeedTime=60", "clone", "--quiet", "--depth", "1", "--", url, part) != 0 || !complete(part, dir) {
		return false
	}
	if err := swap(part, dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	return true
}

func main() {
	url := env.Arg(1)
	switch {
	case url == "-h" || url == "--help":
		fmt.Print(usage)
		return
	case !strings.HasPrefix(url, "https://"):
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	name := themeName(url)
	if !theme.ValidName(name) {
		fmt.Fprintf(os.Stderr, "cannot tell the theme name from %s\n", url)
		os.Exit(1)
	}
	dir := filepath.Join(env.AppConfig(), "themes", name)
	if taken(dir, url) {
		fmt.Fprintf(os.Stderr, "%s is already there and isn't a clone of %s; move it away to install this one\n", dir, url)
		os.Exit(1)
	}
	if !install(url, dir) {
		os.Exit(1)
	}
	os.Exit(run.Report(run.Exec("hyprtrixie-theme-set", name)))
}
