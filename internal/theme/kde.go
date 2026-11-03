package theme

import (
	"slices"
	"strings"
)

const kdeScheme = "ColorScheme=Hyprtrixie"

func kdeColorGroup(header string) bool {
	return strings.HasPrefix(header, "[Colors:") || strings.HasPrefix(header, "[ColorEffects:") || header == "[WM]" || header == "[Icons]"
}

func ownKDESetting(line string) bool {
	return strings.HasPrefix(line, "ColorScheme=") || strings.HasPrefix(line, "font=")
}

func yourKDELines(current string) []string {
	var kept []string
	header := ""
	for _, line := range strings.Split(strings.TrimRight(current, "\n"), "\n") {
		if strings.HasPrefix(line, "[") {
			header = line
		}
		if kdeColorGroup(header) || header == "[General]" && ownKDESetting(line) {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

func ownKDELines(scheme string) []string {
	var own []string
	keep := false
	for _, line := range strings.Split(strings.TrimRight(scheme, "\n"), "\n") {
		if strings.HasPrefix(line, "[") {
			keep = kdeColorGroup(line)
		}
		if keep {
			own = append(own, line)
		}
	}
	return own
}

func withKDESettings(lines, settings []string) []string {
	for index, line := range lines {
		if line == "[General]" {
			return slices.Insert(lines, index+1, settings...)
		}
	}
	return append(append(lines, "[General]"), append(settings, "")...)
}

func WithoutKDEScheme(current string) string {
	text := strings.Join(yourKDELines(current), "\n")
	return strings.TrimRight(text, "\n") + "\n"
}

func KDEGlobals(current, scheme, font string) string {
	settings := []string{kdeScheme}
	if font != "" {
		settings = append(settings, "font="+font+",11,-1,5,400,0,0,0,0,0,0,0,0,0,0,1")
	}
	lines := withKDESettings(yourKDELines(current), settings)
	text := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n\n" + strings.Join(ownKDELines(scheme), "\n") + "\n"
	return strings.TrimLeft(text, "\n")
}
