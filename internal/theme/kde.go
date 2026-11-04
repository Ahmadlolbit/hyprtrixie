package theme

import (
	"slices"
	"strings"
)

const kdeScheme = "ColorScheme=Hyprtrixie"

func kdeColorGroup(header string) bool {
	return strings.HasPrefix(header, "[Colors:") || strings.HasPrefix(header, "[ColorEffects:") || header == "[WM]"
}

func yourKDELines(current string) []string {
	var kept []string
	header := ""
	for _, line := range strings.Split(strings.TrimRight(current, "\n"), "\n") {
		if strings.HasPrefix(line, "[") {
			header = line
		}
		if kdeColorGroup(header) || header == "[UiSettings]" || header == "[General]" && strings.HasPrefix(line, "ColorScheme=") {
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

func withKDEScheme(lines []string) []string {
	for index, line := range lines {
		if line == "[General]" {
			return slices.Insert(lines, index+1, kdeScheme)
		}
	}
	return append(lines, "[General]", kdeScheme, "")
}

func KDEGlobals(current, scheme string) string {
	lines := withKDEScheme(yourKDELines(current))
	own := append(ownKDELines(scheme), "", "[UiSettings]", kdeScheme)
	text := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n\n" + strings.Join(own, "\n") + "\n"
	return strings.TrimLeft(text, "\n")
}
