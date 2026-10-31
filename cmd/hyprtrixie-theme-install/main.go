package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `Usage: hyprtrixie-theme-install <https URL of a theme repository>

Clones an Omarchy theme into ~/.config/hyprtrixie/themes and applies it. A theme
from a repository can change colours and backgrounds, but not run code.
`

var validName = regexp.MustCompile(`^[a-z0-9_][a-z0-9._+-]*$`)

func themeName(url string) string {
	name := path.Base(strings.TrimRight(url, "/"))
	name = strings.TrimSuffix(name, ".git")
	name = strings.TrimPrefix(name, "omarchy-")
	name = strings.TrimSuffix(name, "-theme")
	return strings.ToLower(name)
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

func install(url, dir string) bool {
	if err := os.MkdirAll(filepath.Dir(dir), 0o777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	part := dir + ".part"
	os.RemoveAll(part)
	defer os.RemoveAll(part)
	if run.Status("git", "-c", "http.lowSpeedLimit=1000", "-c", "http.lowSpeedTime=60", "clone", "--quiet", "--depth", "1", "--", url, part) != 0 {
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
	if !validName.MatchString(name) {
		fmt.Fprintf(os.Stderr, "cannot tell the theme name from %s\n", url)
		os.Exit(1)
	}
	if !install(url, filepath.Join(env.AppConfig(), "themes", name)) {
		os.Exit(1)
	}
	os.Exit(run.Report(run.Exec("hyprtrixie-theme-set", name)))
}
