package presets

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const sizeLimit = 64 * 1024

var (
	yours       = []string{"DEBIAN_MIRROR"}
	ErrTooLarge = errors.New("that preset is larger than 64 KB")
)

type Change struct {
	Key    string
	Before string
	After  string
}

func Export(current *config.Config) string {
	var kept strings.Builder
	for _, line := range strings.SplitAfter(current.Text(), "\n") {
		if key, _, _ := strings.Cut(line, "="); !slices.Contains(yours, key) {
			kept.WriteString(line)
		}
	}
	return kept.String()
}

func Remote(address string) bool {
	return strings.HasPrefix(address, "https://")
}

func Fetch(address, target string) error {
	err := run.LoggedWithin(time.Minute, os.Stderr, "curl", "-fsSL", "--proto", "=https", "--proto-redir", "=https",
		"--max-filesize", strconv.Itoa(sizeLimit), "--max-time", "60", "-o", target, address)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if info.Size() > sizeLimit {
		return ErrTooLarge
	}
	return nil
}

func Import(current *config.Config, path string) []Change {
	before := map[string]string{}
	for _, option := range current.Options() {
		before[option.Key] = current.Get(option.Key)
	}
	current.Merge(path)
	for _, key := range yours {
		current.Set(key, before[key])
	}
	var changes []Change
	for _, option := range current.Options() {
		if after := current.Get(option.Key); after != before[option.Key] {
			changes = append(changes, Change{option.Key, before[option.Key], after})
		}
	}
	return changes
}
