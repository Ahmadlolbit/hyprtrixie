package config

import (
	"cmp"
	"fmt"
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

type Config struct {
	options []options.Option
	values  map[string]string
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

func Read(path string) *Config {
	current := Defaults()
	if fsutil.IsFile(path) {
		current.migrate(current.apply(fsutil.Optional(path)))
		current.validate()
	}
	return current
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

func (current *Config) apply(text string) string {
	loaded := ""
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := parseLine(line)
		if ok && key == versionKey {
			loaded = value
		} else if _, known := current.values[key]; ok && known {
			current.values[key] = value
		}
	}
	return loaded
}

func (current *Config) validate() {
	for _, option := range current.options {
		if !option.Valid(current.values[option.Key]) {
			current.values[option.Key] = option.DefaultFor(current.values["PROFILE"])
		}
	}
}

func (current *Config) migrate(loaded string) {
	from, err := strconv.ParseUint(loaded, 10, 64)
	if err != nil {
		from = 1
	}
	if from < 2 {
		current.addNewApps()
	}
	if from < 3 {
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
