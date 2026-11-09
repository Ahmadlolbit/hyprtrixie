package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/dotfiles"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

func dotfilesCommand() int {
	if os.Geteuid() == 0 {
		return fail("run it as yourself, without sudo: hyprtrixie dotfiles %s", strings.Join(os.Args[2:], " "))
	}
	if err := dotfiles.Ready(); err != nil {
		return fail("%v", err)
	}
	repository := env.Arg(2)
	if repository == "" {
		return dotfiles.Changes()
	}
	if code := dotfiles.Init(repository); code != 0 {
		return code
	}
	written, err := dotfiles.SyncBack()
	if err != nil {
		return fail("%v", err)
	}
	if written {
		fmt.Printf("chezmoi will commit what you change through it. To send it to %s, run: chezmoi git push\n", repository)
	}
	return dotfiles.Apply()
}
