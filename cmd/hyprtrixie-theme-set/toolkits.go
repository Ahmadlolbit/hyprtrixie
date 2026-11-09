package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

const (
	appsFollow  = "follow"
	appsOff     = "off"
	gtkOne      = "hyprtrixie-one"
	gtkTwo      = "hyprtrixie-two"
	gtkOurs     = "hyprtrixie-"
	gtkThemes   = "/usr/share/themes"
	gtkUserFile = "gtk.css"
	kdeBackup   = ".before-hyprtrixie"
)

var (
	toolkitFiles = []string{"gtk3.css", "gtk4.css", "kde.colors"}
	gtkKept      = []string{"color-scheme", "gtk-theme", "icon-theme"}
)

func appColors(colors theme.Colors) theme.Colors {
	name := settings.Get("apps")
	if name == appsFollow || name == appsOff {
		return colors
	}
	if !theme.ValidName(name) {
		fmt.Fprintf(os.Stderr, "no theme named %s for GTK and KDE apps\n", name)
		return colors
	}
	for _, dir := range []string{filepath.Join(userThemes(), name), filepath.Join(systemRoot, "themes", name)} {
		if fixed, err := themeColors(dir); err == nil {
			return fixed
		}
	}
	fmt.Fprintf(os.Stderr, "no theme named %s for GTK and KDE apps\n", name)
	return colors
}

func gtkBase(mode string) string {
	if mode == "light" {
		return "adw-gtk3"
	}
	return "adw-gtk3-dark"
}

func gtkFallback(mode string) string {
	if mode == "light" {
		return "Adwaita"
	}
	return "Adwaita-dark"
}

func nextGTKName() string {
	used := filepath.Join(state(), "gtk.name")
	name := gtkOne
	if fsutil.Optional(used) == gtkOne {
		name = gtkTwo
	}
	if err := atomic.Write(used, []byte(name), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return name
}

func writeGTKTheme(name, base string) {
	current := filepath.Join(state(), "theme")
	for _, part := range [][2]string{{"gtk-3.0", "gtk3.css"}, {"gtk-4.0", "gtk4.css"}} {
		text := fmt.Sprintf("@import url(%q);\n@import url(%q);\n",
			filepath.Join(gtkThemes, base, part[0], gtkUserFile), filepath.Join(current, part[1]))
		file := filepath.Join(env.DataHome(), "themes", name, part[0], gtkUserFile)
		err := os.MkdirAll(filepath.Dir(file), 0o777)
		if err == nil {
			err = atomic.Write(file, []byte(text), 0o666)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func gtkTheme(mode string) string {
	base := gtkBase(mode)
	if !fsutil.IsDir(filepath.Join(gtkThemes, base)) {
		return gtkFallback(mode)
	}
	name := nextGTKName()
	writeGTKTheme(name, base)
	return name
}

func ourLink(link string) bool {
	target, err := os.Readlink(link)
	return err == nil && strings.HasPrefix(target, filepath.Join(state(), "theme")+string(filepath.Separator))
}

func linkUnlessYours(file, link string) {
	if (fsutil.Exists(link) || fsutil.IsLink(link)) && !ourLink(link) {
		fmt.Fprintf(os.Stderr, "left your %s alone\n", link)
		return
	}
	symlink(file, link)
}

func keepKDEGlobals(globals, text string) {
	backup := globals + kdeBackup
	if text == "" || strings.Contains(text, theme.KDEScheme) || fsutil.Exists(backup) {
		return
	}
	if err := atomic.Write(backup, []byte(text), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func appsFont() string {
	if font := settings.Get("ui_font"); font != "default" {
		return font
	}
	return ""
}

func writeKDEGlobals(scheme string) {
	globals := filepath.Join(env.ConfigHome(), "kdeglobals")
	current := fsutil.Optional(globals)
	merged := theme.KDEGlobals(current, scheme, appsFont(), theme.TextScale(settings.Get("text")))
	if scheme == "" && strings.Contains(current, theme.KDEScheme) {
		merged = theme.KDEGivenBack(merged, fsutil.Optional(globals+kdeBackup))
	}
	if strings.TrimSpace(merged) == strings.TrimSpace(current) {
		return
	}
	keepKDEGlobals(globals, current)
	if err := atomic.Write(globals, []byte(merged), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func kdeColors(file string) {
	scheme, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	symlink(file, filepath.Join(env.DataHome(), "color-schemes", "Hyprtrixie.colors"))
	writeKDEGlobals(string(scheme))
}

func unlinkToolkits() {
	for _, link := range []string{filepath.Join(env.ConfigHome(), "gtk-4.0", gtkUserFile), filepath.Join(env.DataHome(), "color-schemes", "Hyprtrixie.colors")} {
		if ourLink(link) {
			os.Remove(link)
		}
	}
	writeKDEGlobals("")
}

func gtkBefore() string {
	return filepath.Join(state(), "gtk.before")
}

func rememberGTK() {
	if fsutil.Exists(gtkBefore()) {
		return
	}
	var text strings.Builder
	for _, key := range gtkKept {
		value := desktopValue(key)
		if value == "" || strings.Contains(value, gtkOurs) {
			return
		}
		fmt.Fprintf(&text, "%s\t%s\n", key, value)
	}
	if err := atomic.Write(gtkBefore(), []byte(text.String()), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func rememberedGTK() map[string]string {
	lines, _ := fsutil.Lines(gtkBefore())
	before := map[string]string{}
	for _, line := range lines {
		if key, value, found := strings.Cut(line, "\t"); found && slices.Contains(gtkKept, key) && value != "" {
			before[key] = value
		}
	}
	return before
}

func releaseGTK(mode string) {
	used := filepath.Join(state(), "gtk.name")
	before := rememberedGTK()
	if len(before) == 0 && fsutil.Exists(used) {
		desktop("set", "gtk-theme", gtkFallback(mode))
	}
	for _, key := range gtkKept {
		if value, ok := before[key]; ok {
			desktop("set", key, value)
		}
	}
	os.Remove(used)
	os.Remove(gtkBefore())
}

func tellKDEApps() {
	for _, change := range []string{"0", "2"} {
		run.Quiet(run.Quick, "gdbus", "emit", "--session", "--object-path", "/KGlobalSettings", "--signal", "org.kde.KGlobalSettings.notifyChange", change, "0")
	}
	run.Quiet(run.Quick, "gdbus", "emit", "--session", "--object-path", "/KIconLoader", "--signal", "org.kde.KIconLoader.iconChanged", "0")
	run.Quiet(run.Quick, "gdbus", "emit", "--session", "--object-path", "/KDEPlatformTheme", "--signal", "org.kde.KDEPlatformTheme.refreshFonts")
}

func linkToolkits(current string) {
	if settings.Get("apps") == appsOff {
		unlinkToolkits()
		return
	}
	if file := filepath.Join(current, "gtk4.css"); fsutil.IsFile(file) {
		linkUnlessYours(file, filepath.Join(env.ConfigHome(), "gtk-4.0", gtkUserFile))
	}
	if file := filepath.Join(current, "kde.colors"); fsutil.IsFile(file) {
		kdeColors(file)
	}
}
