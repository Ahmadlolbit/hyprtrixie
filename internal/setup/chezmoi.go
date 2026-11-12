package setup

import (
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/dotfiles"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const chezmoiLimit = 4 << 10

func (context *Context) chezmoiConfig() string {
	return context.Person.Path(filepath.Join(".config/chezmoi", dotfiles.ConfigName))
}

func (context *Context) chezmoiPushing() (string, bool) {
	uid, _, err := context.Person.IDs()
	if err != nil {
		return "", false
	}
	data, err := fsutil.ReadPlain(context.chezmoiConfig(), chezmoiLimit, uid)
	if err != nil {
		return "", false
	}
	return dotfiles.Unpushed(string(data))
}

func (context *Context) chezmoiNoPush(log *runlog.Log, text string) error {
	config := context.chezmoiConfig()
	part := config + ".hyprtrixie-part"
	err := context.Person.Write(log, part, text)
	for _, args := range [][]string{{"chmod", "600", part}, {"mv", "-fT", part, config}} {
		if err == nil {
			err = context.Person.Run(log, args...)
		}
	}
	if err != nil {
		run.Quiet(run.Quick, "runuser", context.Person.RunuserArgs("rm", "-f", part)...)
	}
	return err
}

func chezmoiSteps(context *Context) {
	if text, pushing := context.chezmoiPushing(); pushing {
		context.Run.Step("Stop chezmoi pushing your dotfiles on its own", func(log *runlog.Log) error { return context.chezmoiNoPush(log, text) })
	}
}
