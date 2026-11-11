package keyboard

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	layoutKey    = "XKBLAYOUT="
	variantKey   = "XKBVARIANT="
	optionsKey   = "XKBOPTIONS="
	groupToggle  = "grp:alt_shift_toggle"
	layoutHeader = "! layout"
	fallback     = "XKBMODEL=\"pc105\"\nXKBOPTIONS=\"\"\nBACKSPACE=\"guess\"\n"
)

var nonLatin = strings.Fields("af am ara bd bg bt by et ge gn gr il in iq ir kg kh kz la lk ma mk mm mn mv my np pk rs ru sy th tj ua uz")

type Layout struct {
	ID    string
	Label string
}

func File() string {
	return env.Or("HYPRTRIXIE_KEYBOARD_FILE", "/etc/default/keyboard")
}

func rules() string {
	return fsutil.Optional(env.Or("HYPRTRIXIE_XKB_RULES", "/usr/share/X11/xkb/rules/base.lst"))
}

func Layouts() []Layout {
	var found []Layout
	listing := false
	for _, line := range strings.Split(rules(), "\n") {
		if strings.HasPrefix(line, "!") {
			listing = strings.TrimSpace(line) == layoutHeader
			continue
		}
		if id, label, ok := strings.Cut(strings.TrimSpace(line), " "); listing && ok {
			found = append(found, Layout{id, strings.TrimSpace(label)})
		}
	}
	slices.SortFunc(found, func(first, second Layout) int { return strings.Compare(first.Label, second.Label) })
	return found
}

func Known(id string) bool {
	return slices.ContainsFunc(Layouts(), func(layout Layout) bool { return layout.ID == id })
}

func Current() string {
	for _, line := range strings.Split(fsutil.Optional(File()), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), layoutKey); ok {
			return single(strings.Trim(value, `"`))
		}
	}
	return "us"
}

func single(layout string) string {
	if rest, paired := strings.CutPrefix(layout, "us,"); paired && slices.Contains(nonLatin, rest) {
		return rest
	}
	return layout
}

func With(text, id string) string {
	if strings.TrimSpace(text) == "" {
		text = fallback
	}
	layout, variant := id, ""
	if slices.Contains(nonLatin, id) {
		layout, variant, text = "us,"+id, ",", withToggle(text)
	}
	var lines []string
	placed := false
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), layoutKey) && !placed:
			lines, placed = append(lines, fmt.Sprintf("%s%q", layoutKey, layout)), true
		case strings.HasPrefix(strings.TrimSpace(line), layoutKey):
		case strings.HasPrefix(strings.TrimSpace(line), variantKey):
			lines = append(lines, fmt.Sprintf("%s%q", variantKey, variant))
		default:
			lines = append(lines, line)
		}
	}
	if !placed {
		lines = append(lines, fmt.Sprintf("%s%q", layoutKey, layout))
	}
	return strings.Join(lines, "\n") + "\n"
}

func withToggle(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for index, line := range lines {
		options, found := strings.CutPrefix(strings.TrimSpace(line), optionsKey)
		if !found {
			continue
		}
		if options = strings.Trim(options, `"`); !strings.Contains(options, "grp:") {
			lines[index] = fmt.Sprintf("%s%q", optionsKey, strings.TrimPrefix(options+","+groupToggle, ","))
		}
		return strings.Join(lines, "\n") + "\n"
	}
	return strings.Join(append(lines, fmt.Sprintf("%s%q", optionsKey, groupToggle)), "\n") + "\n"
}
