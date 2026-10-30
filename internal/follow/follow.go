package follow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var consoleKeys = []string{"background", "red", "green", "yellow", "blue", "magenta", "cyan", "foreground", "muted",
	"bright_red", "bright_green", "bright_yellow", "bright_blue", "bright_magenta", "bright_cyan", "bright_foreground"}

func Setting(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if value, ok := strings.CutPrefix(line, key+"="); ok {
			return value
		}
	}
	return ""
}

func Color(text, key string) string {
	found := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + ` *= *"(#[0-9a-fA-F]{6})"`).FindStringSubmatch(text)
	if found == nil {
		return ""
	}
	return found[1]
}

func Channels(color string) [3]int {
	var values [3]int
	fmt.Sscanf(strings.TrimPrefix(color, "#"), "%02x%02x%02x", &values[0], &values[1], &values[2])
	return values
}

func Floats(color string) string {
	var parts []string
	for _, value := range Channels(color) {
		scaled := value * 1000 / 255
		parts = append(parts, fmt.Sprintf("%d.%03d", scaled/1000, scaled%1000))
	}
	return strings.Join(parts, ", ")
}

func Blend(foreground, background string) string {
	front, back := Channels(foreground), Channels(background)
	mixed := "#"
	for index := range front {
		mixed += fmt.Sprintf("%02x", (front[index]*3+back[index]*2)/5)
	}
	return mixed
}

func Palette(text, foreground string) string {
	var rows [3][]string
	for _, key := range consoleKeys {
		value := Color(text, key)
		if plain, bright := strings.CutPrefix(key, "bright_"); value == "" && bright {
			value = Color(text, plain)
		}
		if value == "" {
			value = foreground
		}
		for index, channel := range Channels(value) {
			rows[index] = append(rows[index], strconv.Itoa(channel))
		}
	}
	return strings.Join(rows[0], ",") + "\n" + strings.Join(rows[1], ",") + "\n" + strings.Join(rows[2], ",") + "\n"
}

func Extension(path string) (string, bool) {
	extension := path[strings.LastIndex(path, ".")+1:]
	for _, char := range []byte(extension) {
		if !alphanumeric(char) {
			return "", false
		}
	}
	return extension, extension != ""
}

func alphanumeric(char byte) bool {
	return ('a' <= char && char <= 'z') || ('A' <= char && char <= 'Z') || ('0' <= char && char <= '9')
}

func Fill(template string, pairs ...string) string {
	for _, pair := range pairs {
		key, value, _ := strings.Cut(pair, "=")
		template = strings.ReplaceAll(template, "{{ "+key+" }}", value)
	}
	return template
}
