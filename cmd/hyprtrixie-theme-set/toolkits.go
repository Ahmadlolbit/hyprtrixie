package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	gtkThemes   = "/usr/share/themes"
	gtkUserFile = "gtk.css"
)

var toolkitFiles = []string{"gtk3.css", "gtk4.css", "kde.colors"}

func appColors(colors theme.Colors) theme.Colors {
	name := settings.Get("apps")
	if name == appsFollow || name == appsOff {
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
	backup := globals + ".before-hyprtrixie"
	if text == "" || strings.Contains(text, theme.KDEScheme) || fsutil.Exists(backup) {
		return
	}
	if err := atomic.Write(backup, []byte(text), 0o666); err != nil {
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
	globals := filepath.Join(env.ConfigHome(), "kdeglobals")
	current := fsutil.Optional(globals)
	keepKDEGlobals(globals, current)
	font := settings.Get("ui_font")
	if font == "default" {
		font = ""
	}
	merged := theme.KDEGlobals(current, string(scheme), font)
	if err := atomic.Write(globals, []byte(merged), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func unlinkToolkits() {
	for _, link := range []string{filepath.Join(env.ConfigHome(), "gtk-4.0", gtkUserFile), filepath.Join(env.DataHome(), "color-schemes", "Hyprtrixie.colors")} {
		if ourLink(link) {
			os.Remove(link)
		}
	}
	globals := filepath.Join(env.ConfigHome(), "kdeglobals")
	text := fsutil.Optional(globals)
	if !strings.Contains(text, theme.KDEScheme) {
		return
	}
	if err := atomic.Write(globals, []byte(theme.WithoutKDEScheme(text)), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func releaseGTK(mode string) {
	used := filepath.Join(state(), "gtk.name")
	if !fsutil.Exists(used) {
		return
	}
	desktop("set", "gtk-theme", gtkFallback(mode))
	os.Remove(used)
}

func tellKDEApps() {
	for _, change := range []string{"0", "2"} {
		run.Quiet(run.Quick, "gdbus", "emit", "--session", "--object-path", "/KGlobalSettings", "--signal", "org.kde.KGlobalSettings.notifyChange", change, "0")
	}
	run.Quiet(run.Quick, "gdbus", "emit", "--session", "--object-path", "/KIconLoader", "--signal", "org.kde.KIconLoader.iconChanged", "0")
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
