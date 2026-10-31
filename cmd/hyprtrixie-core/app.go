package main

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apps"
	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

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

func appCommand() int {
	if cmp.Or(env.Arg(2), "list") != "list" {
		return backToBash()
	}
	packages := apt.Installed()
	teamsSource := config.Read(config.Path()).Get("TEAMS_SOURCE")
	present := func(app apps.App) bool { return app.Present(packages, teamsSource) }
	catalogue := apps.All()
	if filter := env.Arg(3); filter != "" {
		return say(filteredApps(catalogue, present, filter))
	}
	return say(appsByCategory(catalogue, present))
}
