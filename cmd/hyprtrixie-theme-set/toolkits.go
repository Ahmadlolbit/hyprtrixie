package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

const (
	gtkOne      = "hyprtrixie-one"
	gtkTwo      = "hyprtrixie-two"
	gtkThemes   = "/usr/share/themes"
	gtkUserFile = "gtk.css"
)

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

func linkUnlessYours(file, link string) {
	if fsutil.Exists(link) && !fsutil.IsLink(link) {
		fmt.Fprintf(os.Stderr, "left your %s alone\n", link)
		return
	}
	symlink(file, link)
}

func kdeColors(file string) {
	scheme, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	symlink(file, filepath.Join(env.DataHome(), "color-schemes", "Hyprtrixie.colors"))
	globals := filepath.Join(env.ConfigHome(), "kdeglobals")
	merged := theme.KDEGlobals(fsutil.Optional(globals), string(scheme))
	if err := atomic.Write(globals, []byte(merged), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func linkToolkits(current string) {
	if file := filepath.Join(current, "gtk4.css"); fsutil.IsFile(file) {
		linkUnlessYours(file, filepath.Join(env.ConfigHome(), "gtk-4.0", gtkUserFile))
	}
	if file := filepath.Join(current, "kde.colors"); fsutil.IsFile(file) {
		kdeColors(file)
	}
}
