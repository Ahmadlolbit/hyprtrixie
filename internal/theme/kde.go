package theme

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

const (
	KDEScheme      = "ColorScheme=Hyprtrixie"
	kdeFontKey     = "font="
	kdeRecord      = "[Hyprtrixie]"
	kdeRecordKey   = "GeneralFont="
	kdeFontSize    = 10
	kdeFontRest    = ",-1,5,400,0,0,0,0,0,0,0,0,0,0,1"
	kdeDefaultFont = "Noto Sans"
)

func kdeFont(font string, scale float64) string {
	if font == "" && scale == 1 {
		return ""
	}
	return cmp.Or(font, kdeDefaultFont) + "," + strconv.FormatFloat(kdeFontSize*scale, 'f', -1, 64) + kdeFontRest
}

func kdeValue(text, group, key string) string {
	header := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "[") {
			header = line
		} else if header == group && strings.HasPrefix(line, key) {
			return strings.TrimPrefix(line, key)
		}
	}
	return ""
}

func kdeColorGroup(header string) bool {
	return strings.HasPrefix(header, "[Colors:") || strings.HasPrefix(header, "[ColorEffects:") || header == "[WM]" || header == "[Icons]"
}

func ownKDESetting(line string, colors, font bool) bool {
	return colors && strings.HasPrefix(line, "ColorScheme=") || font && strings.HasPrefix(line, kdeFontKey)
}

func yourKDELines(current string, colors, font bool) []string {
	var kept []string
	header := ""
	for _, line := range strings.Split(strings.TrimRight(current, "\n"), "\n") {
		if strings.HasPrefix(line, "[") {
			header = line
		}
		if colors && kdeColorGroup(header) || header == kdeRecord || header == "[General]" && ownKDESetting(line, colors, font) {
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

func KDEGlobals(current, scheme, font string, scale float64) string {
	have := kdeValue(current, "[General]", kdeFontKey)
	yours := font == "" && have != "" && have != kdeValue(current, kdeRecord, kdeRecordKey)
	wanted := ""
	if !yours {
		wanted = kdeFont(font, scale)
	}
	var settings []string
	if scheme != "" {
		settings = append(settings, KDEScheme)
	}
	if wanted != "" {
		settings = append(settings, kdeFontKey+wanted)
	}
	lines := yourKDELines(current, scheme != "" || strings.Contains(current, KDEScheme), !yours)
	if len(settings) > 0 {
		lines = withKDESettings(lines, settings)
	}
	text := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	if own := ownKDELines(scheme); len(own) > 0 {
		text += "\n\n" + strings.Join(own, "\n")
	}
	if wanted != "" {
		text += "\n\n" + kdeRecord + "\n" + kdeRecordKey + wanted
	}
	return strings.TrimLeft(text+"\n", "\n")
}
