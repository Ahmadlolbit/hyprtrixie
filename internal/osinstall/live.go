package osinstall

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
)

const (
	liveHome   = "/home/hyprtrixie/"
	stateMode  = 0o600
	targetData = "var/lib/hyprtrixie"
)

func dropLines(path string, gone func(string) bool) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(data), "\n")
	var kept strings.Builder
	for _, line := range lines {
		if !gone(line) {
			kept.WriteString(line)
		}
	}
	if kept.Len() == len(data) {
		return nil
	}
	return atomic.Write(path, []byte(kept.String()), stateMode)
}

func liveFile(line string) bool {
	return strings.HasPrefix(line, liveHome)
}

func liveRecord(line string) bool {
	_, value, _ := strings.Cut(line, " ")
	return strings.HasPrefix(value, liveHome)
}

func forgetLiveUser() error {
	state := inTarget(targetData)
	if err := dropLines(filepath.Join(state, "files.db"), liveFile); err != nil {
		return err
	}
	records, err := filepath.Glob(filepath.Join(state, "modules", "*.record"))
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := dropLines(record, liveRecord); err != nil {
			return err
		}
	}
	return nil
}
