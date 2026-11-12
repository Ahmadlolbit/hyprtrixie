package battery

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	endFile   = "charge_control_end_threshold"
	startFile = "charge_control_start_threshold"
	lowest    = 50
	Full      = 100
	headroom  = 5
	keptWords = 7
)

func root() string {
	return env.Or("HYPRTRIXIE_POWER_SUPPLY", "/sys/class/power_supply")
}

func Keep() string {
	return env.Or("HYPRTRIXIE_BATTERY_KEEP", "/etc/udev/rules.d/90-hyprtrixie-battery.rules")
}

func older() string {
	return env.Or("HYPRTRIXIE_BATTERY_OLD", "/etc/tmpfiles.d/hyprtrixie-battery.conf")
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
	writes := []Write{{start, "0"}, end}
	if limit < Full {
		writes = append(writes, Write{start, strconv.Itoa(limit - headroom)})
	}
	return writes
}

func Rules(folders []string, limit int) string {
	var rules strings.Builder
	for _, folder := range folders {
		fmt.Fprintf(&rules, `ACTION=="add|change", SUBSYSTEM=="power_supply", KERNEL=="%s", ATTR{%s}!="%d"`, filepath.Base(folder), endFile, limit)
		for _, write := range Writes(folder, limit) {
			fmt.Fprintf(&rules, `, ATTR{%s}="%s"`, filepath.Base(write.File), write.Value)
		}
		rules.WriteString("\n")
	}
	return rules.String()
}

func gone(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func Save(folders []string, limit int) error {
	if limit == Full {
		return errors.Join(gone(Keep()), gone(older()))
	}
	if err := os.MkdirAll(filepath.Dir(Keep()), 0o755); err != nil {
		return err
	}
	if err := atomic.Write(Keep(), []byte(Rules(folders, limit)), 0o644); err != nil {
		return err
	}
	return gone(older())
}

func kept(text string) (int, bool) {
	for _, line := range strings.Split(text, "\n") {
		words := strings.Fields(line)
		if len(words) == keptWords && words[0] == "w" && filepath.Base(words[1]) == endFile {
			return Parse(words[keptWords-1])
		}
	}
	return 0, false
}

func Upgrade() error {
	limit, found := kept(fsutil.Optional(older()))
	folders := Batteries()
	if !found || len(folders) == 0 {
		return nil
	}
	return Save(folders, limit)
}

type Reading struct {
	Percent int
	Plugged bool
	AtLimit bool
}

func number(folder, name string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(fsutil.Optional(filepath.Join(folder, name))), 64)
	return value, err == nil
}

func stored(folder string) (float64, float64, bool) {
	for _, kind := range []string{"energy", "charge"} {
		now, haveNow := number(folder, kind+"_now")
		full, haveFull := number(folder, kind+"_full")
		if haveNow && haveFull && full > 0 {
			return now, full, true
		}
	}
	return 0, 0, false
}

func percent(levels []int, now, full float64, weighed bool) int {
	if weighed && full > 0 {
		return min(max(int(math.Round(now*Full/full)), 0), Full)
	}
	total := 0
	for _, level := range levels {
		total += level
	}
	return (total + len(levels)/2) / len(levels)
}

func Read() (Reading, bool) {
	folders, err := filepath.Glob(filepath.Join(root(), "BAT*"))
	if err != nil {
		return Reading{}, false
	}
	var levels []int
	var now, full float64
	weighed, statuses := true, map[string]bool{}
	for _, folder := range folders {
		level, err := strconv.Atoi(strings.TrimSpace(fsutil.Optional(filepath.Join(folder, "capacity"))))
		if err != nil {
			continue
		}
		levels = append(levels, min(max(level, 0), Full))
		left, size, ok := stored(folder)
		now, full, weighed = now+left, full+size, weighed && ok
		statuses[strings.TrimSpace(fsutil.Optional(filepath.Join(folder, "status")))] = true
	}
	if len(levels) == 0 {
		return Reading{}, false
	}
	limited := statuses["Not charging"] && !statuses["Charging"]
	return Reading{percent(levels, now, full, weighed), statuses["Charging"] || statuses["Full"] || limited, limited}, true
}
