package theme

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/Ahmadlolbit/hyprtrixie/internal/follow"
)

const DefaultFont = "JetBrainsMono Nerd Font"

var (
	quoted    = pattern(`^[[:space:]]*([a-z0-9_]+)[[:space:]]*=[[:space:]]*"([^"]*)"`)
	bare      = pattern(`^[[:space:]]*([a-z0-9_]+)[[:space:]]*=[[:space:]]*([^[:space:]#"]+)`)
	safeValue = pattern(`^[A-Za-z0-9#(),.%_ -]*$`)
	hexColor  = pattern(`^#[0-9A-Fa-f]{6}$`)
	themeName = pattern(`^[a-z0-9][a-z0-9_-]*$`)
	fontName  = pattern(`^[\p{L}\p{M}\p{N}][\p{L}\p{M}\p{N} ._+-]*$`)
	markup    = pattern(`<[^>\n]+>`)
	hues      = []string{"red", "green", "yellow", "blue", "magenta", "cyan"}
	aliases   = [][2]string{
		{"background", "bg"}, {"foreground", "fg"}, {"dark_background", "dark_bg"}, {"darker_background", "darker_bg"},
		{"lighter_background", "lighter_bg"}, {"dark_foreground", "dark_fg"}, {"light_foreground", "light_fg"},
		{"bright_foreground", "bright_fg"}, {"background", "color0"}, {"foreground", "color7"}, {"red", "color1"},
		{"green", "color2"}, {"yellow", "color3"}, {"blue", "color4"}, {"magenta", "color5"}, {"cyan", "color6"},
		{"bright_red", "color9"}, {"bright_green", "color10"}, {"bright_yellow", "color11"}, {"bright_blue", "color12"},
		{"bright_magenta", "color13"}, {"bright_cyan", "color14"}, {"magenta", "purple"}, {"mode", "theme_type"},
	}
)

type Colors map[string]string

func pattern(expression string) func() *regexp.Regexp {
	return sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(expression) })
}

func Parse(text string) Colors {
	colors := Colors{}
	lines := strings.Split(text, "\n")
	for _, line := range lines[:len(lines)-1] {
		if key, value, ok := assignment(line); ok && safeValue().MatchString(value) {
			colors[key] = value
		}
	}
	return colors
}

func assignment(line string) (string, string, bool) {
	match := quoted().FindStringSubmatch(line)
	if match == nil {
		match = bare().FindStringSubmatch(line)
	}
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
}

func (colors Colors) fallback(key, other, value string) {
	if colors[key] != "" {
		return
	}
	if other != "" && colors[other] != "" {
		value = colors[other]
	}
	colors[key] = value
}

func Mix(first, second string, percent int) string {
	if !IsHex("#"+strings.TrimPrefix(first, "#")) || !IsHex("#"+strings.TrimPrefix(second, "#")) {
		return first
	}
	one, other := follow.Channels(first), follow.Channels(second)
	mixed := "#"
	for index := range one {
		mixed += fmt.Sprintf("%02x", (one[index]*(100-percent)+other[index]*percent+50)/100)
	}
	return mixed
}

func IsHex(value string) bool {
	return hexColor().MatchString(value)
}

func (colors Colors) Resolve() {
	for _, pair := range aliases {
		colors.fallback(pair[0], pair[1], "")
	}
	colors.fallback("background", "", "#1e1e2e")
	colors.fallback("foreground", "", "#cdd6f4")
	for _, hue := range hues {
		colors.fallback(hue, "foreground", "")
	}
	colors.fallback("bright_foreground", "color15", colors["foreground"])
	colors.fallback("light_foreground", "foreground", "")
	colors.fallback("lighter_background", "background", "")
	colors.fallback("dark_foreground", "color8", colors["foreground"])
	colors.fallback("muted", "color8", colors["dark_foreground"])
	colors.fallback("selection", "selection_background", colors["muted"])
	colors.fallback("selection_background", "selection", "")
	colors.fallback("selection_foreground", "bright_foreground", "")
	colors.fallback("accent", "blue", "")
	colors.fallback("orange", "yellow", "")
	colors.resolveShades()
	if colors["mode"] != "light" {
		colors["mode"] = "dark"
	}
}

func (colors Colors) resolveShades() {
	colors.fallback("brown", "", Mix(colors["orange"], "#000000", 50))
	colors.fallback("dark_background", "", Mix(colors["background"], "#000000", 25))
	colors.fallback("darker_background", "", Mix(colors["background"], "#000000", 50))
	for _, hue := range hues {
		colors.fallback("bright_"+hue, "", Mix(colors[hue], "#ffffff", 20))
	}
}

func (colors Colors) Accent(setting string) (string, bool) {
	if IsHex(setting) {
		return setting, true
	}
	if setting != "theme" && IsHex(colors[setting]) {
		return colors[setting], true
	}
	return "", false
}

func WithAccent(text, accent, original string) string {
	line := `accent = "` + accent + `"`
	lines := strings.Split(text, "\n")
	found := false
	for index := range lines {
		if isAccentLine(lines[index]) {
			lines[index], found = line, true
		}
	}
	result := strings.Join(lines, "\n")
	if !found {
		result += line + "\n"
	}
	return result + `theme_accent = "` + original + `"` + "\n"
}

func isAccentLine(line string) bool {
	rest, ok := strings.CutPrefix(line, "accent")
	return ok && strings.HasPrefix(strings.TrimLeft(rest, " "), "=")
}

func (colors Colors) Replacer() *strings.Replacer {
	var pairs []string
	for _, key := range slices.Sorted(maps.Keys(colors)) {
		pairs = append(pairs, placeholders(key, colors[key])...)
	}
	return strings.NewReplacer(pairs...)
}

func placeholders(key, value string) []string {
	pairs := []string{"{{ " + key + " }}", value, "{{ " + key + "_strip }}", strings.TrimPrefix(value, "#")}
	if IsHex(value) {
		rgb := follow.Channels(value)
		pairs = append(pairs, "{{ "+key+"_rgb }}", fmt.Sprintf("%d,%d,%d", rgb[0], rgb[1], rgb[2]))
	}
	return pairs
}

func Pretty(text string) string {
	shown := []byte(text)
	for index, char := range shown {
		switch {
		case char == '-':
			shown[index] = ' '
		case 'a' <= char && char <= 'z' && startsWord(text, index):
			shown[index] = char - 'a' + 'A'
		}
	}
	return string(shown)
}

func startsWord(text string, index int) bool {
	return index == 0 || text[index-1] == '-' || text[index-1] == '\n'
}

func Slug(value string) string {
	plain := []byte(strings.TrimRight(markup().ReplaceAllString(value, ""), "\n"))
	for index, char := range plain {
		switch {
		case 'A' <= char && char <= 'Z':
			plain[index] = char - 'A' + 'a'
		case char == ' ':
			plain[index] = '-'
		}
	}
	return string(plain)
}

func ValidName(name string) bool {
	return themeName().MatchString(name)
}

func ValidFont(name string) bool {
	return fontName().MatchString(name)
}
