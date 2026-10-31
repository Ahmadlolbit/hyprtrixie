package care

import (
	"encoding/json"
	"slices"
	"strings"
)

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
			waiting = append(waiting, device.Name+" "+device.Releases[0].Version)
		}
	}
	return waiting
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
