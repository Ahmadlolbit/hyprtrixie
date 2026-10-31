package step

import (
	"os"
	"os/signal"
	"strconv"
	"strings"

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
)

var choices = []string{"Retry", "Skip", "Continue", "Halt", showLog}

type Work func(log *runlog.Log) error

type Mark struct {
	Kind  string
	Title string
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

func (runner *Runner) ExitCode() int {
	runner.ended()
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
	return runner.attempt(kindStep, title, func() error {
		signal.Stop(runner.signals)
		defer cleanup.Notify(runner.signals)
		return work()
	})
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

func (runner *Runner) ended() bool {
	select {
	case <-runner.stop:
		runner.Interrupted = true
	default:
	}
	return runner.Halted || runner.Interrupted
}

func (runner *Runner) attempt(kind, title string, perform func() error) bool {
	for !runner.ended() {
		runner.Log.Line("info", kind+": "+title)
		offset := runner.Log.Size()
		err := perform()
		if runner.ended() {
			return false
		}
		if err == nil {
			runner.passed(kind)
			ui.OK(title)
			return true
		}
		choice := runner.failed(kind, title, offset, err)
		if choice != retry {
			runner.giveUp(choice, Mark{kind, title})
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
	picked := make(chan string, 1)
	go func() {
		pick, ok := runner.Choose(header, choices)
		if !ok {
			pick = halt
		}
		picked <- pick
	}()
	select {
	case pick := <-picked:
		return pick
	case <-runner.stop:
		runner.Interrupted = true
		return halt
	}
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
