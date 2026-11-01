package setup

import (
	"errors"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

const (
	programMode = 0o755
)

func packages(lists ...string) step.Work {
	return func(log *runlog.Log) error { return apt.InstallLists(log, lists...) }
}

func commands(names ...string) step.Work {
	return func(*runlog.Log) error { return apt.Commands(names...) }
}

func quietly(what string, name string, args ...string) error {
	if run.Quiet(run.Minute, name, args...) != 0 {
		return errors.New(what)
	}
	return nil
}

func expect(ok bool, what string) error {
	if !ok {
		return errors.New(what)
	}
	return nil
}

func holds(ok bool) error {
	if !ok {
		return run.Reported("")
	}
	return nil
}

func fileThere(path string) error {
	return expect(fsutil.IsFile(path), path+" is missing")
}
