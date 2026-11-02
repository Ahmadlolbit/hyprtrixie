package config

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/options"
)

const (
	version        = 3
	versionKey     = "HYPRTRIXIE_CONFIG_VERSION"
	keyCharacters  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
	safeCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 ._:/@+,-"
	newApps        = "imv evince obsidian localsend calculator"
)

var renamedThemes = map[string]string{"lumon": "northern-sea", "vantablack": "pitch-black"}

type Note struct {
	Level string
	Text  string
}

type Config struct {
	Notes   []Note
	options []options.Option
	values  map[string]string
}

func RenamedThemes() map[string]string {
	return maps.Clone(renamedThemes)
}

func Path() string {
	return env.Or("HYPRTRIXIE_CONFIG", "/etc/hyprtrixie/hyprtrixie.conf")
}

func Defaults() *Config {
	current := &Config{options: options.All(), values: map[string]string{}}
	for _, option := range current.options {
		current.values[option.Key] = option.DefaultFor("developer")
	}
	return current
}

func Read(paths ...string) *Config {
	current := Defaults()
	for _, path := range paths {
		current.Load(path)
	}
	return current
}

func (current *Config) Load(path string) bool {
	if !fsutil.IsFile(path) {
		return false
	}
	current.migrate(current.apply(path, fsutil.Optional(path)))
	current.validate()
	return true
}

func (current *Config) Get(key string) string {
	return current.values[key]
}

func (current *Config) Set(key, value string) {
	current.values[key] = value
}

func (current *Config) Options() []options.Option {
	return current.options
}

func (current *Config) Facts(nvidia bool) options.Facts {
	return options.Facts{Value: current.Get, Nvidia: nvidia}
}

func (current *Config) ResetProfile() {
	for _, option := range current.options {
		if option.Preset() {
			current.values[option.Key] = option.DefaultFor(current.values["PROFILE"])
		}
	}
}

func SafeText(character rune) bool {
	return strings.ContainsRune(safeCharacters, character)
}

func (current *Config) Enabled(key, item string) bool {
	return slices.Contains(strings.Fields(current.Get(key)), item)
}

func parseLine(line string) (string, string, bool) {
	key, rest, found := strings.Cut(line, "=")
	if !found || key == "" || key[0] < 'A' || key[0] > 'Z' || strings.Trim(key, keyCharacters) != "" {
		return "", "", false
	}
	if len(rest) < 2 || rest[0] != '"' || rest[len(rest)-1] != '"' {
		return "", "", false
	}
	value := rest[1 : len(rest)-1]
	return key, value, strings.Trim(value, safeCharacters) == ""
}

func (current *Config) note(level, text string) {
	current.Notes = append(current.Notes, Note{level, text})
}

func (current *Config) applyLine(path, line string) string {
	key, value, ok := parseLine(line)
	if !ok {
		current.note("warn", "ignoring malformed line in "+path+": "+line)
		return ""
	}
	if key == versionKey {
		return value
	}
	if _, known := current.values[key]; known {
		current.values[key] = value
	} else {
		current.note("warn", "ignoring unknown setting "+key+" in "+path)
	}
	return ""
}

func (current *Config) apply(path, text string) string {
	loaded := ""
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		if found := current.applyLine(path, line); found != "" {
			loaded = found
		}
	}
	return loaded
}

func (current *Config) validate() {
	for _, option := range current.options {
		if value := current.values[option.Key]; !option.Valid(value) {
			fallback := option.DefaultFor(current.values["PROFILE"])
			current.note("warn", "invalid "+option.Key+"='"+value+"' in the configuration, using '"+fallback+"'")
			current.values[option.Key] = fallback
		}
	}
}

func (current *Config) migrate(loaded string) {
	from, err := strconv.ParseUint(loaded, 10, 64)
	if err != nil {
		from = 1
	}
	if from < 2 {
		current.note("info", "updating the configuration for version 2: apps_add_new_defaults")
		current.addNewApps()
	}
	if from < 3 {
		current.note("info", "updating the configuration for version 3: theme_names_update")
		current.renameThemes()
	}
}

func listed(list, word string) bool {
	return strings.Contains(" "+list+" ", " "+word+" ")
}

func added(list, word string) string {
	if list == "" {
		return word
	}
	return list + " " + word
}

func (current *Config) addNewApps() {
	index := slices.IndexFunc(current.options, func(option options.Option) bool { return option.Key == "APPS" })
	defaults := current.options[index].DefaultFor(current.values["PROFILE"])
	apps := current.values["APPS"]
	for _, id := range strings.Fields(newApps) {
		if listed(defaults, id) && !listed(apps, id) {
			apps = added(apps, id)
		}
	}
	current.values["APPS"] = apps
}

func (current *Config) renameThemes() {
	for _, key := range []string{"THEME", "LIGHT_THEME", "DARK_THEME"} {
		current.values[key] = cmp.Or(renamedThemes[current.values[key]], current.values[key])
	}
}

func (current *Config) Save(path string) error {
	var text strings.Builder
	fmt.Fprintf(&text, "%s=\"%d\"\n", versionKey, version)
	for _, option := range current.options {
		fmt.Fprintf(&text, "%s=\"%s\"\n", option.Key, current.values[option.Key])
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return atomic.Install(path, []byte(text.String()), 0o644)
}

func (current *Config) Show() string {
	var text strings.Builder
	for _, option := range current.options {
		fmt.Fprintf(&text, "%-22s %-28s %s\n", option.Key, option.Label, current.values[option.Key])
	}
	return text.String()
}
