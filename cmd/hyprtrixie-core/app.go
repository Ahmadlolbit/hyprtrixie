package main

import (
	"cmp"
	"fmt"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apps"
	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/engine"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

const appUsage = "usage: hyprtrixie app [list [installed|available] | install APP | remove APP]"

func filteredApps(catalogue []apps.App, present func(apps.App) bool, filter string) string {
	var list strings.Builder
	for _, app := range catalogue {
		if (filter == "installed" && present(app)) || (filter == "available" && !present(app)) {
			fmt.Fprintf(&list, "%s\t%s\n", app.ID, app.Name)
		}
	}
	return list.String()
}

func appsByCategory(catalogue []apps.App, present func(apps.App) bool) string {
	var list strings.Builder
	for _, category := range apps.Categories {
		group := apps.InCategory(catalogue, category[0])
		if len(group) == 0 {
			continue
		}
		list.WriteString(category[1] + "\n")
		for _, app := range group {
			mark := " "
			if present(app) {
				mark = "*"
			}
			fmt.Fprintf(&list, "  %s %-14s %s\n", mark, app.ID, app.Name)
		}
	}
	return list.String() + "\n* installed\n"
}

func appChange(action string, change func(apps.App) int) int {
	id := env.Arg(3)
	if id == "" {
		return fail("usage: sudo hyprtrixie app %s APP (see: hyprtrixie app list)", action)
	}
	app, found := apps.Find(id)
	if !found {
		return fail("no app named %s (see: hyprtrixie app list)", id)
	}
	if os.Getenv("HYPRTRIXIE_ENGINE") == "" {
		return throughScript()
	}
	return asRoot("app-"+action, func() int { return change(app) })
}

func appCommand() int {
	switch action := cmp.Or(env.Arg(2), "list"); action {
	case "install":
		return appChange(action, engine.AddApp)
	case "remove":
		return appChange(action, engine.RemoveApp)
	case "list":
		return appList()
	}
	return fail(appUsage)
}

func appList() int {
	packages, err := apt.Installed()
	if err != nil {
		return fail("the installed packages can't be read: %v", err)
	}
	teamsSource := config.Read(config.Path()).Get("TEAMS_SOURCE")
	present := func(app apps.App) bool { return app.Present(packages, teamsSource) }
	catalogue := apps.All()
	if filter := env.Arg(3); filter != "" {
		return say(filteredApps(catalogue, present, filter))
	}
	return say(appsByCategory(catalogue, present))
}
