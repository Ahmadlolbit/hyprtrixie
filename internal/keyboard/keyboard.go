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
	layoutHeader = "! layout"
	fallback     = "XKBMODEL=\"pc105\"\nXKBOPTIONS=\"\"\nBACKSPACE=\"guess\"\n"
)

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
			return strings.Trim(value, `"`)
		}
	}
	return "us"
}

func With(text, id string) string {
	if strings.TrimSpace(text) == "" {
		text = fallback
	}
	layout := fmt.Sprintf("%s%q", layoutKey, id)
	var lines []string
	placed := false
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), layoutKey) && !placed:
			lines, placed = append(lines, layout), true
		case strings.HasPrefix(strings.TrimSpace(line), layoutKey):
		case strings.HasPrefix(strings.TrimSpace(line), variantKey):
			lines = append(lines, variantKey+`""`)
		default:
			lines = append(lines, line)
		}
	}
	if !placed {
		lines = append(lines, layout)
	}
	return strings.Join(lines, "\n") + "\n"
}
