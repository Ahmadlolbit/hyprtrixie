package followtheme

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/follow"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const browserPolicy = "hyprtrixie-theme.json"

type browser struct {
	name string
	dirs []string
}

var (
	policyRoot = env.Or("HYPRTRIXIE_POLICY_ROOT", "/")
	browsers   = []browser{
		{"chromium", []string{"etc/chromium/policies/managed"}},
		{"helium-bin", []string{"etc/chromium/policies/managed", "etc/helium/policies/managed"}},
		{"google-chrome-stable", []string{"etc/opt/chrome/policies/managed"}},
		{"brave-browser", []string{"etc/brave/policies/managed"}},
	}
)

func policyDirs(installed map[string]bool) ([]string, []string) {
	var wanted, all []string
	for _, each := range browsers {
		for _, dir := range each.dirs {
			path := filepath.Join(policyRoot, dir)
			if !slices.Contains(all, path) {
				all = append(all, path)
			}
			if installed[each.name] && !slices.Contains(wanted, path) {
				wanted = append(wanted, path)
			}
		}
	}
	return wanted, all
}

func browserColor(current theme) string {
	path := filepath.Join(current.home, ".local/state/hyprtrixie/current/theme/chromium.theme")
	own, err := run.CaptureQuiet(run.Quick, "runuser", "-u", current.user, "--", "head", "-c", "64", path)
	if color, ok := follow.FromRGB(own); err == nil && ok {
		return color
	}
	return current.background
}

func dropPolicy(dir string) bool {
	err := os.Remove(filepath.Join(dir, browserPolicy))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	return true
}

func paintBrowsers(current theme) bool {
	installed, err := apt.Installed()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	wanted, all := policyDirs(installed)
	if follow.Setting(current.settings, "browsers") != "yes" {
		wanted = nil
	}
	text := fmt.Sprintf("{\n  \"BrowserThemeColor\": %q\n}\n", browserColor(current))
	ok := true
	for _, dir := range all {
		if !slices.Contains(wanted, dir) {
			ok = dropPolicy(dir) && ok
			continue
		}
		if _, err := refresh(filepath.Join(dir, browserPolicy), text); err != nil {
			fmt.Fprintln(os.Stderr, err)
			ok = false
		}
	}
	return ok
}
