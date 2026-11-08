package ledger

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	Package    = "package"
	Wants      = "wants"
	File       = "file"
	Service    = "service"
	Made       = "made"
	Replaced   = "replaced"
	folderMode = 0o755
	recordMode = 0o644
)

type Record map[string][]string

var (
	active Record
	noting string
)

func Folder() string {
	return filepath.Join(env.StateDir(), "modules")
}

func Path(module string) string {
	return filepath.Join(Folder(), module+".record")
}

func startPath(module string) string {
	return filepath.Join(Folder(), module+".before")
}

func Begin(module string, before map[string]bool) {
	active, noting = Record{}, module
	if before == nil || fsutil.Exists(startPath(module)) {
		return
	}
	names := slices.Sorted(maps.Keys(before))
	if os.MkdirAll(Folder(), folderMode) == nil {
		atomic.Install(startPath(module), []byte(strings.Join(names, "\n")+"\n"), recordMode)
	}
}

func started(module string) map[string]bool {
	names, err := fsutil.Lines(startPath(module))
	if err != nil || len(names) == 0 {
		return nil
	}
	before := map[string]bool{}
	for _, name := range names {
		before[name] = true
	}
	return before
}

func Unfinished(module string, installed map[string]bool) []string {
	before := started(module)
	var names []string
	for name := range installed {
		if before != nil && !before[name] {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func forgetStart(module string) error {
	if err := os.Remove(startPath(module)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func Note(kind string, values ...string) {
	if active == nil {
		return
	}
	active[kind] = append(active[kind], values...)
	if record, added := Load(noting).with(Record{kind: values}); added {
		save(noting, record)
	}
}

func Load(module string) Record {
	record := Record{}
	for _, line := range strings.Split(fsutil.Optional(Path(module)), "\n") {
		if kind, value, ok := strings.Cut(line, " "); ok && value != "" {
			record[kind] = append(record[kind], value)
		}
	}
	return record
}

func Known(module string) bool {
	return fsutil.IsFile(Path(module))
}

func (record Record) Values(kind string) []string {
	values := slices.Clone(record[kind])
	slices.Sort(values)
	return slices.Compact(values)
}

func (record Record) with(more Record) (Record, bool) {
	added := false
	for kind, values := range more {
		for _, value := range values {
			if !slices.Contains(record[kind], value) {
				record[kind] = append(record[kind], value)
				added = true
			}
		}
	}
	return record, added
}

func save(module string, record Record) error {
	if err := os.MkdirAll(Folder(), folderMode); err != nil {
		return err
	}
	return atomic.Install(Path(module), []byte(record.text()), recordMode)
}

func (record Record) text() string {
	var lines []string
	for kind, values := range record {
		for _, value := range values {
			lines = append(lines, kind+" "+value)
		}
	}
	slices.Sort(lines)
	lines = slices.Compact(lines)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func End(module string, before, after map[string]bool) error {
	if active == nil {
		return nil
	}
	done := active
	active = nil
	if older := started(module); older != nil {
		before = older
	}
	for name := range after {
		if before != nil && !before[name] {
			done[Package] = append(done[Package], name)
		}
	}
	record, _ := Load(module).with(done)
	if err := save(module, record); err != nil || after == nil {
		return err
	}
	return forgetStart(module)
}

func Forget(module, kind, value string) error {
	record := Load(module)
	before := len(record[kind])
	record[kind] = slices.DeleteFunc(record[kind], func(found string) bool { return found == value })
	if len(record[kind]) == before {
		return nil
	}
	return atomic.Install(Path(module), []byte(record.text()), recordMode)
}

func Recorded(kind, value string) bool {
	return Elsewhere("", kind)[value]
}

func WantedElsewhere(module string) map[string]bool {
	return Elsewhere(module, Wants)
}

func Elsewhere(module, kind string) map[string]bool {
	found := map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(Folder(), "*.record"))
	for _, file := range files {
		other := strings.TrimSuffix(filepath.Base(file), ".record")
		if other == module {
			continue
		}
		for _, value := range Load(other).Values(kind) {
			found[value] = true
		}
	}
	return found
}

func Drop(module string) error {
	if err := os.Remove(Path(module)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return forgetStart(module)
}
