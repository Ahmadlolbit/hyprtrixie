package apt

import (
	"bytes"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

func listsUpdate(log *runlog.Log) (string, error) {
	var output bytes.Buffer
	err := get(io.MultiWriter(log, &output), "update")
	return output.String(), err
}

func failureLine(line string) bool {
	first, _, _ := strings.Cut(strings.TrimSpace(line), " ")
	return first == "E:" || first == "Error:"
}

func mentions(line, address string) bool {
	for _, word := range strings.Fields(line) {
		word = strings.TrimRight(strings.Trim(word, `'"`), "/")
		if word == address || strings.HasPrefix(word, address+"/") {
			return true
		}
	}
	return false
}

func vendorAddresses() map[string]string {
	found := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(SourcesDir(), "extrepo_*.sources"))
	for _, file := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "extrepo_"), ".sources")
		if text, err := fsutil.Text(file); err == nil && field(text, "URIs") != "" && ExtrepoEnabled(name) {
			found[name] = strings.TrimSuffix(field(text, "URIs"), "/")
		}
	}
	return found
}

func rejected(output string) []string {
	var names []string
	addresses := vendorAddresses()
	for _, line := range strings.Split(output, "\n") {
		for name, address := range addresses {
			if failureLine(line) && mentions(line, address) && !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)
	return names
}

func doubled(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Conflicting values set for option") {
			lines = append(lines, line)
		}
	}
	return rejected(strings.Join(lines, "\n"))
}

func refresh(log *runlog.Log) error {
	output, err := listsUpdate(log)
	if err == nil {
		return nil
	}
	if twice := doubled(output); len(twice) > 0 {
		for _, name := range twice {
			log.Line("warn", "the "+name+" repository is set up already in an apt source of your own, so extrepo's copy of it is turned off")
			run.LoggedWithin(extrepoLimit, log, "extrepo", "disable", name)
		}
		if output, err = listsUpdate(log); err == nil {
			return nil
		}
	}
	names := rejected(output)
	if len(names) == 0 {
		return err
	}
	for _, name := range names {
		log.Line("warn", "apt rejects the "+name+" repository, so extrepo fetches its settings again")
		run.LoggedWithin(extrepoLimit, log, "extrepo", "update", name)
	}
	if output, err = listsUpdate(log); err == nil {
		return nil
	}
	for _, name := range rejected(output) {
		log.Line("warn", "apt still rejects the "+name+" repository, so it's turned off and its apps keep the version they have; sudo extrepo enable "+name+" turns it on again")
		run.LoggedWithin(extrepoLimit, log, "extrepo", "disable", name)
	}
	_, err = listsUpdate(log)
	return err
}
