package setup

import (
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const backupHelper = "restic-backup"

func backupSteps(context *Context) {
	context.Run.Step("Install the restic-backup helper", func(log *runlog.Log) error {
		if err := apt.Install(log, "restic"); err != nil {
			return err
		}
		return context.Deploy.UserProgram(filepath.Join(env.ToolsDir(), backupHelper), "bin/"+backupHelper)
	})
}

func backupChecks(context *Context) {
	context.Run.Check("restic-backup is in ~/bin", func(*runlog.Log) error {
		return holds(fsutil.Runnable(context.Person.Path("bin/" + backupHelper)))
	})
}
