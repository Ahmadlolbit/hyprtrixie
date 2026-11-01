package step

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
)

const (
	shownLines = 12
	allLines   = 100000
	showLog    = "Show log"
	ask        = "ask"
	retry      = "retry"
	skip       = "skip"
	halt       = "halt"
	kindStep   = "step"
	kindCheck  = "check"

	haltedCode      = 1
	interruptedCode = 130
	hangupWait      = 200 * time.Millisecond
)

var choices = []string{"Retry", "Skip", "Continue", "Halt", showLog}

type Work func(log *runlog.Log) error

type Mark struct {
	Module string
	Kind   string
	Title  string
}

type answer struct {
	text string
	ok   bool
}

type Runner struct {
	Log         *runlog.Log
	Module      string
	Mode        string
	Choose      func(header string, items []string) (string, bool)
	Steps       int
	Checks      int
	Failures    []Mark
	Skips       []Mark
	Skipped     bool
	Halted      bool
	Interrupted bool
	stop        chan struct{}
	signals     chan os.Signal
	failedAt    int
	skippedAt   int
}

func New(log *runlog.Log, module, mode string) *Runner {
	runner := &Runner{Log: log, Module: module, Mode: mode, Choose: ui.Choose, stop: make(chan struct{}), signals: make(chan os.Signal, 1)}
	cleanup.Notify(runner.signals)
	go func() {
		<-runner.signals
		run.Stop()
		close(runner.stop)
	}()
	return runner
}

func (runner *Runner) Begin(module string) {
	runner.Module = module
	runner.Skipped = false
	runner.failedAt = len(runner.Failures)
	runner.skippedAt = len(runner.Skips)
}

func (runner *Runner) Result() string {
	switch {
	case len(runner.Failures) > runner.failedAt:
		return "FAIL"
	case runner.Skipped:
		return "SKIP"
	case len(runner.Skips) > runner.skippedAt:
		return "PARTIAL"
	}
	return "PASS"
}

func (runner *Runner) ExitCode() int {
	runner.Ended()
	switch {
	case runner.Interrupted:
		return interruptedCode
	case runner.Halted:
		return haltedCode
	}
	return 0
}

func (runner *Runner) Step(title string, work Work) bool {
	return runner.attempt(kindStep, title, runner.logged(title, work))
}

func (runner *Runner) Check(title string, work Work) bool {
	return runner.attempt(kindCheck, title, runner.logged(title, work))
}

func (runner *Runner) Interactive(title string, work func() error) bool {
	return runner.attempt(kindStep, title, work)
}

func (runner *Runner) Note(text string) {
	ui.Note(text)
}

func (runner *Runner) Warn(text string) {
	ui.Warn(text)
}

func (runner *Runner) Answer(ask func() (string, bool)) (string, bool) {
	if runner.Ended() {
		return "", false
	}
	answered := make(chan answer, 1)
	go func() {
		text, ok := ask()
		answered <- answer{text, ok}
	}()
	select {
	case got := <-answered:
		if !got.ok {
			runner.noticeHangup()
		}
		return got.text, got.ok
	case <-runner.stop:
		runner.Interrupted = true
		return "", false
	}
}

func (runner *Runner) noticeHangup() {
	select {
	case <-runner.stop:
		runner.Interrupted = true
	case <-time.After(hangupWait):
	}
}

func (runner *Runner) Broke(kind, text string) {
	runner.Log.Line("error", text)
	ui.Fail(text)
	runner.Failures = append(runner.Failures, Mark{runner.Module, kind, text})
	runner.Halted = true
}

func (runner *Runner) SkipModule(reason string) {
	runner.Skipped = true
	ui.Skip(reason)
	runner.Log.Line("info", "module "+runner.Module+" skipped: "+reason)
}

func (runner *Runner) logged(title string, work Work) func() error {
	return func() error {
		ui.Busy(title)
		defer ui.Done()
		return work(runner.Log)
	}
}

func (runner *Runner) Ended() bool {
	select {
	case <-runner.stop:
		runner.Interrupted = true
	default:
	}
	return runner.Halted || runner.Interrupted
}

func (runner *Runner) attempt(kind, title string, perform func() error) bool {
	for !runner.Ended() {
		runner.Log.Line("info", kind+": "+title)
		offset := runner.Log.Size()
		err := perform()
		if runner.Ended() {
			return false
		}
		if err == nil {
			runner.passed(kind)
			ui.OK(title)
			return true
		}
		choice := runner.failed(kind, title, offset, err)
		if choice != retry {
			runner.giveUp(choice, Mark{runner.Module, kind, title})
			return false
		}
		runner.Log.Line("info", "retrying: "+title)
	}
	return false
}

func (runner *Runner) passed(kind string) {
	if kind == kindCheck {
		runner.Checks++
	} else {
		runner.Steps++
	}
}

func (runner *Runner) failed(kind, title string, offset int64, err error) string {
	run.Explain(runner.Log, err)
	runner.Log.Line("error", kind+" failed (exit "+strconv.Itoa(run.Code(err))+"): "+title)
	ui.Fail(title)
	for _, line := range runner.Log.Since(offset, shownLines) {
		ui.Note(line)
	}
	return runner.choice(title, offset)
}

func (runner *Runner) choice(title string, offset int64) string {
	for runner.Mode == ask {
		pick := runner.question("\"" + title + "\" failed. What now?")
		if pick != showLog {
			return strings.ToLower(pick)
		}
		ui.Page(strings.Join(runner.Log.Since(offset, allLines), "\n"))
	}
	return runner.Mode
}

func (runner *Runner) question(header string) string {
	pick, ok := runner.Answer(func() (string, bool) { return runner.Choose(header, choices) })
	if !ok {
		return halt
	}
	return pick
}

func (runner *Runner) giveUp(choice string, mark Mark) {
	if choice == skip {
		runner.Log.Line("warn", "skipped: "+mark.Title)
		runner.Skips = append(runner.Skips, mark)
		ui.Skip(mark.Title)
		return
	}
	runner.Failures = append(runner.Failures, mark)
	runner.Halted = choice == halt
}
