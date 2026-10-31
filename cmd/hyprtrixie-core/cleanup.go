package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/snapshots"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
)

const (
	cleanGoal   = 85
	deleteLimit = 30 * time.Minute
)

func deleteUntilClean(numbers []string, use int) int {
	deleted := 0
	for _, number := range numbers {
		if run.Within(deleteLimit, "snapper", "--no-dbus", "-c", "root", "delete", "--sync", number) != 0 {
			return fail("snapshot %s could not be deleted", number)
		}
		deleted++
		now, ok := snapshots.DiskUse()
		if !ok {
			return fail("cannot tell how full / is")
		}
		if use = now; use < cleanGoal {
			break
		}
	}
	return say("Deleted " + strconv.Itoa(deleted) + " snapshots. / is " + strconv.Itoa(use) + "% full.\n")
}

func cleanUp(yes bool) int {
	if !installed("snapper") {
		return fail("snapper isn't installed, so there's nothing to clean up")
	}
	use, ok := snapshots.DiskUse()
	if !ok {
		return fail("cannot tell how full / is")
	}
	if use < cleanGoal {
		return say(fmt.Sprintf("/ is %d%% full, so there is nothing to clean up.\n", use))
	}
	all, err := snapshots.All(run.Minute, "--no-dbus")
	if err != nil {
		return fail("the snapshots can't be read")
	}
	numbers := snapshots.Disposable(all)
	if len(numbers) == 0 {
		return say(fmt.Sprintf("/ is %d%% full, but there are no old automatic snapshots to delete.\n", use))
	}
	say(fmt.Sprintf("/ is %d%% full. Up to %d old automatic snapshots can go, oldest first, until it is under %d%%.\n"+
		"The newest three and the ones with your own description stay.\n", use, len(numbers), cleanGoal))
	if !yes && !ui.Confirm("Delete them?") {
		return 0
	}
	return deleteUntilClean(numbers, use)
}

func cleanupCommand() int {
	yes := env.Arg(2) == "--yes"
	if env.Arg(2) != "" && !yes {
		return fail("usage: hyprtrixie cleanup [--yes]")
	}
	return asRoot("cleanup", func() int { return cleanUp(yes) })
}
