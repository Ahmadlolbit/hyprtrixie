package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/follow"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	firefoxStyle  = "hyprtrixie.css"
	firefoxImport = `@import url("hyprtrixie.css");`
	firefoxPref   = `user_pref("toolkit.legacyUserProfileCustomizations.stylesheets", true);`
)

var firefoxHomes = []string{".mozilla/firefox", ".librewolf"}

func firefoxProfiles() []string {
	var found []string
	for _, home := range firefoxHomes {
		prefs, _ := filepath.Glob(filepath.Join(env.Home(), home, "*", "prefs.js"))
		for _, file := range prefs {
			found = append(found, filepath.Dir(file))
		}
	}
	return found
}

func readExisting(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", true
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return "", false
	}
	return string(data), true
}

func browsersFollow() bool {
	return follow.Setting(fsutil.Optional(env.Or("HYPRTRIXIE_FOLLOW_SETTINGS", "/etc/hyprtrixie/follow-theme")), "browsers") == "yes"
}

func rewrite(path string, change func([]string) []string) {
	text, ok := readExisting(path)
	if !ok {
		return
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if text == "" {
		lines = nil
	}
	changed := change(slices.Clone(lines))
	if slices.Equal(changed, lines) {
		return
	}
	data := strings.Join(changed, "\n")
	if data != "" {
		data += "\n"
	}
	err := os.MkdirAll(filepath.Dir(path), 0o777)
	if err == nil {
		err = atomic.Write(path, []byte(data), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func withLine(line string, first bool) func([]string) []string {
	return func(lines []string) []string {
		if slices.Contains(lines, line) {
			return lines
		}
		if first {
			return append([]string{line}, lines...)
		}
		return append(lines, line)
	}
}

func withoutLine(line string) func([]string) []string {
	return func(lines []string) []string {
		return slices.DeleteFunc(lines, func(each string) bool { return each == line })
	}
}

func followFirefox(current string) {
	style := filepath.Join(current, "firefox.css")
	on := browsersFollow() && fsutil.IsFile(style)
	for _, profile := range firefoxProfiles() {
		chrome := filepath.Join(profile, "chrome")
		if !on {
			if err := os.Remove(filepath.Join(chrome, firefoxStyle)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				fmt.Fprintln(os.Stderr, err)
			}
			rewrite(filepath.Join(chrome, "userChrome.css"), withoutLine(firefoxImport))
			continue
		}
		symlink(style, filepath.Join(chrome, firefoxStyle))
		rewrite(filepath.Join(chrome, "userChrome.css"), withLine(firefoxImport, true))
		rewrite(filepath.Join(profile, "user.js"), withLine(firefoxPref, false))
	}
}
