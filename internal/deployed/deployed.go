package deployed

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	hexDigits = "0123456789abcdef"
	starts    = "hyprtrixie "
)

type Change struct {
	Path    string
	Missing bool
}

func LastCommit() string {
	logs, _ := filepath.Glob(filepath.Join(env.LogDir(), "run-*.log"))
	slices.Reverse(logs)
	for _, path := range logs {
		data, _ := os.ReadFile(path)
		if commit, ok := lastStart(string(data)); ok {
			return commit
		}
	}
	return ""
}

func lastStart(text string) (string, bool) {
	for at := strings.LastIndex(text, starts); at >= 0; at = strings.LastIndex(text[:at], starts) {
		if commit, ok := startedFrom(text[at+len(starts):]); ok {
			return commit, true
		}
	}
	return "", false
}

func startedFrom(text string) (string, bool) {
	version, rest, _ := strings.Cut(text, " ")
	rest, opened := strings.CutPrefix(rest, "(")
	commit, rest, closed := strings.Cut(rest, ")")
	return commit, opened && closed && !strings.Contains(version, "\n") && strings.HasPrefix(rest, " from ") && strings.Trim(commit, hexDigits) == ""
}

func Commit(root string) string {
	output, _ := run.CaptureQuiet(run.Quick, "git", "-C", root, "rev-parse", "--short", "HEAD")
	return output
}

func Matches(commit, last string) bool {
	return strings.HasPrefix(commit, last) || strings.HasPrefix(last, commit)
}

func Changes(limit time.Duration, messages *os.File) ([]Change, error) {
	data, err := os.ReadFile(filepath.Join(env.StateDir(), "files.db"))
	if err != nil {
		return nil, nil
	}
	lines := strings.Split(string(data), "\n")
	lines = lines[:len(lines)-1]
	current, err := hashes(limit, messages, lines)
	var found []Change
	for _, line := range lines {
		path, hash, _ := strings.Cut(line, "\t")
		have, known := current[path]
		if !fsutil.Exists(path) {
			found = append(found, Change{path, true})
		} else if known && have != hash {
			found = append(found, Change{path, false})
		}
	}
	return found, err
}

func hashes(limit time.Duration, messages *os.File, lines []string) (map[string]string, error) {
	found := map[string]string{}
	var regular []string
	for _, line := range lines {
		path, _, _ := strings.Cut(line, "\t")
		if fsutil.IsFile(path) {
			regular = append(regular, path)
		} else {
			found[path] = ""
		}
	}
	if len(regular) == 0 {
		return found, nil
	}
	output, err := run.CaptureTo(limit, messages, "sha256sum", append([]string{"-z", "--"}, regular...)...)
	entries := strings.Split(output, "\x00")
	for _, entry := range entries[:len(entries)-1] {
		if hash, path, ok := strings.Cut(entry, "  "); ok {
			found[path] = hash
		}
	}
	return found, err
}

func Waiting(limit time.Duration, home string) ([]string, error) {
	output, err := run.CaptureQuiet(limit, "find", "/etc", "/usr/share/hyprtrixie", filepath.Join(home, ".config"), "-name", "*.hyprtrixie-new")
	return run.Lines(output), err
}
