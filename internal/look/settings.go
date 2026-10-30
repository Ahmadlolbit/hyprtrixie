package look

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

var Keys = []string{"accent", "font", "ui_font", "glass", "corners", "auto", "light", "dark", "light_at", "dark_at", "location",
	"rotate", "workspaces", "video", "video_battery", "weather", "weather_units", "quiet", "quiet_from", "quiet_to"}

var defaults = map[string]string{
	"accent": "theme", "font": "JetBrainsMono Nerd Font", "ui_font": "default", "glass": "no", "corners": "no", "auto": "off",
	"light": "catppuccin-latte", "dark": "catppuccin", "light_at": "07:00", "dark_at": "19:00", "location": "zone", "rotate": "0",
	"workspaces": "no", "video": "off", "video_battery": "pause", "weather": "off", "weather_units": "celsius", "quiet": "no",
	"quiet_from": "22:00", "quiet_to": "07:00",
}

type Store struct {
	path   string
	user   string
	system string
	loaded bool
}

func Open() *Store {
	return &Store{path: filepath.Join(env.ConfigHome(), "hyprtrixie", "look.conf")}
}

func Default(key string) (string, bool) {
	value, ok := defaults[key]
	return value, ok
}

func (store *Store) load() {
	if store.loaded {
		return
	}
	store.loaded = true
	store.user = text(store.path)
	store.system = text(env.Or("HYPRTRIXIE_LOOK_SYSTEM", "/etc/hyprtrixie/look.conf"))
}

func text(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func (store *Store) Get(key string) string {
	store.load()
	if value := Value(store.user, key); value != "" {
		return value
	}
	if value := Value(store.system, key); value != "" {
		return value
	}
	return defaults[key]
}

func (store *Store) Put(key, value string) error {
	if err := os.MkdirAll(filepath.Dir(store.path), 0o777); err != nil {
		return err
	}
	current, err := os.ReadFile(store.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	updated := Without(string(current), key) + key + "=" + value + "\n"
	if err := atomic.Install(store.path, []byte(updated), 0o600); err != nil {
		return err
	}
	store.user = updated
	return nil
}

func (store *Store) Coordinates() (string, bool) {
	value := store.Get("location")
	if IsCoordinates(value) {
		return value, true
	}
	name, ok := zone()
	if !ok {
		return "", false
	}
	return ZoneLookup(env.Or("HYPRTRIXIE_ZONEINFO", "/usr/share/zoneinfo"), name)
}

func zone() (string, bool) {
	if name := os.Getenv("TZ"); name != "" {
		return strings.TrimPrefix(name, ":"), true
	}
	path, err := fsutil.RealPath(env.Or("HYPRTRIXIE_LOCALTIME", "/etc/localtime"))
	if err != nil {
		return "", false
	}
	_, name, found := strings.Cut(path, "/zoneinfo/")
	return name, found
}

func Value(text, key string) string {
	prefix := key + "="
	value := ""
	for text != "" {
		var line string
		line, text, _ = strings.Cut(text, "\n")
		if found, ok := strings.CutPrefix(line, prefix); ok {
			value = found
		}
	}
	return value
}

func Without(text, key string) string {
	prefix := key + "="
	var kept strings.Builder
	for text != "" {
		var line string
		line, text, _ = strings.Cut(text, "\n")
		if !strings.HasPrefix(line, prefix) {
			kept.WriteString(line + "\n")
		}
	}
	return kept.String()
}
