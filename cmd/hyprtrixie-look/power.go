package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

var powerNames = []string{"performance", "balanced", "power-saver"}

func powerFollow(args []string) int {
	return onOff{"power", putThen("power", "yes", nothing), putThen("power", "no", nothing)}.run(args)
}

func powerProfiles(args []string) int {
	if len(args) == 0 {
		fmt.Println(settings.Get("power_ac"), settings.Get("power_battery"))
		return 0
	}
	if len(args) != 2 || !slices.Contains(powerNames, args[0]) || !slices.Contains(powerNames, args[1]) {
		fmt.Fprintln(os.Stderr, "power-profiles takes two of performance, balanced and power-saver: on the charger, then on battery")
		return 2
	}
	if !put("power_ac", args[0]) || !put("power_battery", args[1]) {
		return 1
	}
	return 0
}

func powerApply(args []string) int {
	if settings.Get("power") != "yes" {
		return 0
	}
	key := "power_ac"
	if arg(args, 0) == "battery" {
		key = "power_battery"
	}
	return run.Within(run.Quick, "powerprofilesctl", "set", settings.Get(key))
}
