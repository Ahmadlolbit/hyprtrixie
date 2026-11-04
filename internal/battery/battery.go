package battery

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	endFile   = "charge_control_end_threshold"
	startFile = "charge_control_start_threshold"
	lowest    = 50
	Full      = 100
	headroom  = 5
)

func root() string {
	return env.Or("HYPRTRIXIE_POWER_SUPPLY", "/sys/class/power_supply")
}

func Keep() string {
	return env.Or("HYPRTRIXIE_BATTERY_KEEP", "/etc/tmpfiles.d/hyprtrixie-battery.conf")
}

func Batteries() []string {
	found, err := filepath.Glob(filepath.Join(root(), "BAT*", endFile))
	if err != nil {
		return nil
	}
	folders := make([]string, 0, len(found))
	for _, file := range found {
		folders = append(folders, filepath.Dir(file))
	}
	return folders
}

func Limit() string {
	for _, folder := range Batteries() {
		return strings.TrimSpace(fsutil.Optional(filepath.Join(folder, endFile)))
	}
	return ""
}

func Parse(value string) (int, bool) {
	if value == "off" {
		return Full, true
	}
	limit, err := strconv.Atoi(value)
	return limit, err == nil && limit >= lowest && limit <= Full
}

type Write struct {
	File  string
	Value string
}

func Writes(folder string, limit int) []Write {
	end := Write{filepath.Join(folder, endFile), strconv.Itoa(limit)}
	start := filepath.Join(folder, startFile)
	if !fsutil.Exists(start) {
		return []Write{end}
	}
	return []Write{{start, "0"}, end, {start, strconv.Itoa(limit - headroom)}}
}

func KeepLines(folders []string, limit int) string {
	var lines strings.Builder
	for _, folder := range folders {
		for _, write := range Writes(folder, limit)[1:] {
			fmt.Fprintf(&lines, "w %s - - - - %s\n", write.File, write.Value)
		}
		if len(Writes(folder, limit)) == 1 {
			fmt.Fprintf(&lines, "w %s - - - - %d\n", filepath.Join(folder, endFile), limit)
		}
	}
	return lines.String()
}
