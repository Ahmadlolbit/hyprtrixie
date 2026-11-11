package apt

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

var orphans = []string{"autoremove", "-o", "APT::AutoRemove::SuggestsImportant=false"}

func simulated(log *runlog.Log, args ...string) (map[string]bool, error) {
	output, err := run.CaptureTo(run.Slow, log, "apt-get", append([]string{"-s"}, args...)...)
	taken := map[string]bool{}
	for _, line := range run.Lines(output) {
		if rest, found := strings.CutPrefix(line, "Remv "); found {
			name, _, _ := strings.Cut(rest, " ")
			taken[name] = true
		}
	}
	return taken, err
}

func mark(log *runlog.Log, how string, names []string) error {
	if len(names) == 0 {
		return nil
	}
	return run.LoggedWithin(run.Minute, log, "apt-mark", append([]string{how}, names...)...)
}

func putBack(log *runlog.Log, manual []string, cause error) error {
	if err := mark(log, "manual", manual); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func freed(names []string, before, after map[string]bool) ([]string, []string) {
	var gone, kept []string
	for name := range after {
		if !before[name] || slices.Contains(names, name) {
			gone = append(gone, name)
		}
	}
	for _, name := range names {
		if !after[name] {
			kept = append(kept, name)
		}
	}
	slices.Sort(gone)
	return gone, kept
}

func beyond(taken map[string]bool, gone []string) []string {
	var extra []string
	for name := range taken {
		if !slices.Contains(gone, name) {
			extra = append(extra, name)
		}
	}
	slices.Sort(extra)
	return extra
}

func checkedRemoval(log *runlog.Log, gone []string) error {
	taken, err := simulated(log, append([]string{"remove"}, gone...)...)
	if err != nil {
		return err
	}
	if extra := beyond(taken, gone); len(extra) > 0 {
		return fmt.Errorf("removing %s would also remove %s, which it didn't bring, so nothing was removed", strings.Join(gone, " "), strings.Join(extra, " "))
	}
	return get(log, append([]string{"remove"}, gone...)...)
}

func Remove(log *runlog.Log, names ...string) ([]string, error) {
	before, err := simulated(log, orphans...)
	if err != nil {
		return nil, err
	}
	output, err := run.CaptureTo(run.Minute, log, "apt-mark", append([]string{"showmanual"}, names...)...)
	if err != nil {
		return nil, err
	}
	manual := run.Lines(output)
	if err := mark(log, "auto", names); err != nil {
		return nil, err
	}
	after, err := simulated(log, orphans...)
	if err != nil {
		return nil, putBack(log, manual, err)
	}
	gone, kept := freed(names, before, after)
	if len(gone) == 0 {
		return kept, nil
	}
	if err := checkedRemoval(log, gone); err != nil {
		return nil, putBack(log, manual, err)
	}
	return kept, nil
}
