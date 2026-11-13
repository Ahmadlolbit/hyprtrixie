package look

import (
	"bytes"
	"encoding/json"

	"github.com/Ahmadlolbit/hyprtrixie/internal/ordered"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const shellPath = "/usr/share/hyprtrixie/shell"

func CallShell(target, function string, args ...string) {
	run.Quiet(run.Quick, "qs", append([]string{"-p", shellPath, "ipc", "call", target, function}, args...)...)
}

func ReloadShell() {
	CallShell("theme", "reload")
}

type barSettings struct {
	Bar struct {
		Preset   string `json:"preset"`
		Position string `json:"position"`
	} `json:"bar"`
}

func Preset(data []byte) string {
	var shell barSettings
	if json.Unmarshal(unmarked(data), &shell) != nil {
		return ""
	}
	return shell.Bar.Preset
}

func Position(data []byte) string {
	var shell barSettings
	if json.Unmarshal(unmarked(data), &shell) != nil {
		return ""
	}
	return shell.Bar.Position
}

func WithPreset(data []byte, preset string) ([]byte, error) {
	return withBar(data, "preset", preset)
}

func WithPosition(data []byte, position string) ([]byte, error) {
	return withBar(data, "position", position)
}

func withBar(data []byte, key, value string) ([]byte, error) {
	document, err := parseOrEmpty(unmarked(data))
	if err != nil {
		return nil, err
	}
	raw, _ := document.Get("bar")
	bar, err := parseOrEmpty(raw)
	if err != nil {
		return nil, err
	}
	name, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	bar.Set(key, name)
	updated, err := bar.Indented()
	if err != nil {
		return nil, err
	}
	document.Set("bar", updated)
	return document.Indented()
}

func unmarked(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte("\ufeff"))
}

func parseOrEmpty(data []byte) (*ordered.Object, error) {
	text := bytes.TrimSpace(data)
	if len(text) == 0 || bytes.Equal(text, []byte("null")) {
		return &ordered.Object{}, nil
	}
	return ordered.Parse(data)
}
