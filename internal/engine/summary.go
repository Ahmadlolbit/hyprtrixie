package engine

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/modules"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/setup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
)

const (
	minuteSeconds = 60
	hourSeconds   = 3600
	changesShown  = 30
)

func duration(seconds int) string {
	switch {
	case seconds >= hourSeconds:
		return fmt.Sprintf("%d h %d min", seconds/hourSeconds, seconds%hourSeconds/minuteSeconds)
	case seconds >= minuteSeconds:
		return fmt.Sprintf("%d min %d s", seconds/minuteSeconds, seconds%minuteSeconds)
	}
	return fmt.Sprintf("%d s", seconds)
}

func count(marks []step.Mark, kind string) int {
	found := 0
	for _, mark := range marks {
		if mark.Kind == kind {
			found++
		}
	}
	return found
}

func (work *session) newPackages() []string {
	var added []string
	for _, name := range packages() {
		if !slices.Contains(work.before, name) {
			added = append(added, name)
		}
	}
	return added
}

func (work *session) next(added []string) []string {
	kernel := slices.ContainsFunc(added, func(name string) bool { return strings.HasPrefix(name, "linux-image-") })
	if kernel {
		return []string{"Next: restart to start the new kernel"}
	}
	for _, id := range []string{"12", "13", "14"} {
		if _, ran := work.results[id]; ran {
			return []string{"Next: log out and back in to start the updated desktop"}
		}
	}
	return nil
}

func (work *session) summaryLines() []string {
	text := fsutil.Optional(work.log.Path())
	updated, kept := strings.Count(text, "] deployed "), strings.Count(text, "kept local edits in ")
	added := work.newPackages()
	runner := work.runner
	lines := []string{
		"Took " + duration(int(time.Since(work.started).Seconds())),
		fmt.Sprintf("Steps: %d passed, %d skipped, %d failed", runner.Steps, count(runner.Skips, "step"), count(runner.Failures, "step")),
		fmt.Sprintf("Checks: %d passed, %d failed", runner.Checks, count(runner.Failures, "check")),
		fmt.Sprintf("Packages installed: %d", len(added)),
		fmt.Sprintf("Files updated: %d, kept with your edits: %d", updated, kept),
	}
	lines = append(lines, work.next(added)...)
	if kept > 0 {
		lines = append(lines, "Files you edited kept your edits; the new versions are next to them as *.hyprtrixie-new")
	}
	if len(runner.Failures) > 0 {
		lines = append(lines, "Something failed: run sudo hyprtrixie collect --verbose and send the file it prints")
	}
	return lines
}

func (work *session) changes() {
	if work.options.Changes == "" {
		return
	}
	lines := run.Lines(strings.TrimSuffix(fsutil.Optional(work.options.Changes), "\n"))
	if len(lines) == 0 {
		return
	}
	ui.Rule("What changed in this update")
	for _, line := range lines[:min(len(lines), changesShown)] {
		ui.Print("  " + line + "\n")
	}
	if len(lines) > changesShown {
		ui.Print("  … the rest: hyprtrixie changes\n")
	}
}

func (work *session) paths() {
	ui.Print("\n  Log: " + work.log.Path() + "\n")
	if work.log.Copy() != "" {
		ui.Print("  Copy: " + work.log.Copy() + "\n")
	}
	if report := work.person.Path(setup.ReportName); work.person.Home != "" && fsutil.IsFile(report) {
		ui.Print("  Report: " + report + "\n")
	}
}

func (work *session) summary() {
	if len(work.results) == 0 {
		return
	}
	ui.Rule("Summary")
	for _, module := range modules.All {
		if result, ran := work.results[module.ID]; ran {
			ui.ModuleResult(module.ID, module.Title, result)
		}
	}
	if work.mode == installMode {
		ui.Print("\n" + strings.Join(work.summaryLines(), "\n") + "\n")
	}
	work.changes()
	work.paths()
}
