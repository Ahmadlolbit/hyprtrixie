package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/editors"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

type codeEditor struct {
	command    string
	settings   string
	extensions string
}

type editorPack struct {
	file string
	sum  string
}

type palette struct {
	name   string
	label  string
	colors theme.Colors
}

type rendered struct {
	name  string
	label string
	mode  string
	data  []byte
}

var codeEditors = []codeEditor{
	{"code", ".config/Code/User/settings.json", ".vscode/extensions"},
	{"codium", ".config/VSCodium/User/settings.json", ".vscode-oss/extensions"},
}

func editorState(parts ...string) string {
	return filepath.Join(append([]string{env.StateHome(), "hyprtrixie", "editors"}, parts...)...)
}

func editorTemplate(name string) string {
	for _, dir := range []string{filepath.Join(env.AppConfig(), "themed", "editors"), filepath.Join(systemRoot, "themed", "editors")} {
		if path := filepath.Join(dir, name); fsutil.IsFile(path) {
			return path
		}
	}
	return ""
}

func themeDir(name string) string {
	if own := filepath.Join(userThemes(), name); fsutil.IsFile(filepath.Join(own, "colors.toml")) {
		return own
	}
	return filepath.Join(systemRoot, "themes", name)
}

func editorPalettes() []palette {
	var found []palette
	for _, name := range themeNames() {
		colors, err := themeColors(themeDir(name))
		if err != nil {
			continue
		}
		if accent, ok := colors.Accent(settings.Get("accent")); ok {
			colors["accent"] = accent
		}
		found = append(found, palette{name, editors.Prefix + theme.Pretty(name), colors})
	}
	return found
}

func renderEditors(template string, palettes []palette) []rendered {
	data, err := os.ReadFile(template)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}
	var found []rendered
	for _, each := range palettes {
		text := each.colors.Replacer().Replace(string(data))
		if !json.Valid([]byte(text)) {
			fmt.Fprintf(os.Stderr, "%s: %s doesn't make valid JSON\n", each.name, template)
			continue
		}
		found = append(found, rendered{each.name, each.label, each.colors["mode"], []byte(text)})
	}
	return found
}

func buildPack(palettes []palette) *editorPack {
	template := editorTemplate("vscode.json.tpl")
	if template == "" {
		return nil
	}
	var found []editors.Theme
	for _, item := range renderEditors(template, palettes) {
		found = append(found, editors.NewTheme(item.label, item.mode, item.name+".json", item.data))
	}
	if len(found) == 0 {
		return nil
	}
	pack := &editorPack{file: editorState("hyprtrixie-themes.vsix"), sum: editors.Fingerprint(found)}
	if have, _ := fsutil.Text(editorState("vsix.sum")); have == pack.sum && fsutil.IsFile(pack.file) {
		return pack
	}
	data, err := editors.VSIX("1.0."+strconv.FormatInt(time.Now().Unix(), 10), found)
	if err == nil {
		err = os.MkdirAll(editorState(), 0o777)
	}
	if err == nil {
		err = atomic.Write(pack.file, data, 0o666)
	}
	if err == nil {
		err = atomic.Write(editorState("vsix.sum"), []byte(pack.sum+"\n"), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}
	return pack
}

func (editor codeEditor) installed() bool {
	found, _ := filepath.Glob(filepath.Join(env.Home(), editor.extensions, editors.Publisher+"."+editors.Name+"-*"))
	return len(found) > 0
}

func (editor codeEditor) refresh(pack *editorPack) {
	mark := editorState(editor.command + ".sum")
	if have, _ := fsutil.Text(mark); have == pack.sum && editor.installed() {
		return
	}
	if run.Quiet(run.Minute, editor.command, "--install-extension", pack.file, "--force") != 0 {
		fmt.Fprintf(os.Stderr, "%s didn't take the hyprtrixie themes\n", editor.command)
		return
	}
	if err := atomic.Write(mark, []byte(pack.sum+"\n"), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func followSetting(path, key, value string) {
	current, ok := readExisting(path)
	if !ok {
		return
	}
	changed, ok := editors.SetTheme(current, key, value)
	if !ok {
		return
	}
	err := os.MkdirAll(filepath.Dir(path), 0o777)
	if err == nil {
		err = atomic.Write(path, []byte(changed), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func followCode(name string, palettes func() []palette) {
	var pack *editorPack
	for _, editor := range codeEditors {
		if _, err := exec.LookPath(editor.command); err != nil {
			continue
		}
		if pack == nil {
			if pack = buildPack(palettes()); pack == nil {
				return
			}
		}
		editor.refresh(pack)
		followSetting(filepath.Join(env.Home(), editor.settings), "workbench.colorTheme", editors.Prefix+theme.Pretty(name))
	}
}

func followEditors(name string) {
	if settings.Get("editors") != "yes" {
		return
	}
	palettes := sync.OnceValue(editorPalettes)
	followCode(name, palettes)
	followZed(name, palettes)
}

func refreshEditors() int {
	held, err := lock()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer held.Close()
	name, err := fsutil.Text(filepath.Join(state(), "theme.name"))
	if err != nil || name == "" {
		fmt.Fprintln(os.Stderr, "no theme is applied yet")
		return 1
	}
	followFirefox(filepath.Join(state(), "theme"))
	followEditors(name)
	return 0
}
