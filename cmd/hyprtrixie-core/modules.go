package main

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/modules"
)

func moduleTable() string {
	statuses := ""
	if file := modules.StatusFile(); fsutil.IsFile(file) {
		statuses = fsutil.Optional(file)
	}
	var table strings.Builder
	for _, module := range modules.All {
		status, when := modules.StatusOf(statuses, module.ID)
		fmt.Fprintf(&table, "%s  %-34s %-8s %s\n", module.ID, module.Title, cmp.Or(status, "never"), when)
	}
	return table.String()
}

func modulesCommand() int {
	return say(moduleTable())
}
