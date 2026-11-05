package care

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode"
)

var markup = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func clean(text string) string {
	return strings.Join(strings.FieldsFunc(text, func(letter rune) bool {
		return unicode.IsSpace(letter) || unicode.IsControl(letter)
	}), " ")
}

func Updates(data []byte) []string {
	var found struct {
		Devices []struct {
			Name     string
			Releases []struct {
				Version string
			}
		}
	}
	if json.Unmarshal(data, &found) != nil {
		return nil
	}
	var waiting []string
	for _, device := range found.Devices {
		if len(device.Releases) > 0 {
			waiting = append(waiting, clean(device.Name+" "+device.Releases[0].Version))
		}
	}
	return waiting
}

func Markup(text string) string {
	return markup.Replace(text)
}

func Unseen(waiting []string, seen string) []string {
	known := strings.Split(seen, "\n")
	var fresh []string
	for _, update := range waiting {
		if !slices.Contains(known, update) {
			fresh = append(fresh, update)
		}
	}
	return fresh
}
