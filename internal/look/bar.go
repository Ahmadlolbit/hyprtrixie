package look

import (
	"bytes"
	"encoding/json"

	"github.com/Ahmadlolbit/hyprtrixie/internal/ordered"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const shellPath = "/usr/share/hyprtrixie/shell"

func ReloadShell() {
	run.Quiet(run.Quick, "qs", "-p", shellPath, "ipc", "call", "theme", "reload")
}

func Preset(data []byte) string {
	var shell struct {
		Bar struct {
			Preset string `json:"preset"`
		} `json:"bar"`
	}
	if json.Unmarshal(data, &shell) != nil {
		return ""
	}
	return shell.Bar.Preset
}

func WithPreset(data []byte, preset string) ([]byte, error) {
	document, err := parseOrEmpty(data)
	if err != nil {
		return nil, err
	}
	raw, _ := document.Get("bar")
	bar, err := parseOrEmpty(raw)
	if err != nil {
		return nil, err
	}
	name, err := json.Marshal(preset)
	if err != nil {
		return nil, err
	}
	bar.Set("preset", name)
	updated, err := bar.Indented()
	if err != nil {
		return nil, err
	}
	document.Set("bar", updated)
	return document.Indented()
}

func parseOrEmpty(data []byte) (*ordered.Object, error) {
	text := bytes.TrimSpace(data)
	if len(text) == 0 || bytes.Equal(text, []byte("null")) {
		return &ordered.Object{}, nil
	}
	return ordered.Parse(data)
}
