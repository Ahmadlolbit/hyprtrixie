package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/editors"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

const zedSchema = "https://zed.dev/schema/themes/v0.2.0.json"

type zedTheme struct {
	Name       string          `json:"name"`
	Appearance string          `json:"appearance"`
	Style      json.RawMessage `json:"style"`
}

type zedFamily struct {
	Schema string     `json:"$schema"`
	Name   string     `json:"name"`
	Author string     `json:"author"`
	Themes []zedTheme `json:"themes"`
}

func zedInstalled() bool {
	if fsutil.Runnable(filepath.Join(env.Home(), ".local/zed.app/bin/zed")) {
		return true
	}
	for _, name := range []string{"zed", "zeditor"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func zedThemes(palettes []palette) ([]byte, bool) {
	template := editorTemplate("zed.json.tpl")
	if template == "" {
		return nil, false
	}
	family := zedFamily{Schema: zedSchema, Name: "hyprtrixie", Author: "hyprtrixie"}
	for _, item := range renderEditors(template, palettes) {
		family.Themes = append(family.Themes, zedTheme{item.label, item.mode, item.data})
	}
	data, err := json.MarshalIndent(family, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, false
	}
	return append(data, '\n'), len(family.Themes) > 0
}

func writeChanged(file string, data []byte) bool {
	if current, err := os.ReadFile(file); err == nil && bytes.Equal(current, data) {
		return true
	}
	err := os.MkdirAll(filepath.Dir(file), 0o777)
	if err == nil {
		err = atomic.Write(file, data, 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	return true
}

func followZed(name string, palettes func() []palette) {
	if !zedInstalled() {
		return
	}
	data, ok := zedThemes(palettes())
	if !ok {
		return
	}
	if !writeChanged(filepath.Join(env.ConfigHome(), "zed", "themes", "hyprtrixie.json"), data) {
		return
	}
	followSetting(filepath.Join(env.ConfigHome(), "zed", "settings.json"), "theme", editors.Prefix+theme.Pretty(name))
}
