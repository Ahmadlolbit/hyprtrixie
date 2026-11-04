package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/battery"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

func showLimit() int {
	limit := battery.Limit()
	if limit == "" {
		return 1
	}
	return say(limit + "\n")
}

func setLimit(value string) int {
	limit, ok := battery.Parse(value)
	if !ok {
		return fail("the limit is a number from 50 to 100, or off")
	}
	folders := battery.Batteries()
	if len(folders) == 0 {
		return fail("this battery has no charge limit")
	}
	for _, folder := range folders {
		for _, write := range battery.Writes(folder, limit) {
			if run.Supply(run.Quick, io.Discard, write.Value, "tee", write.File) != nil {
				return fail("couldn't set the limit in %s", write.File)
			}
		}
	}
	return keepLimit(folders, limit)
}

func keepLimit(folders []string, limit int) int {
	if limit == battery.Full {
		if remove(battery.Keep()) != 0 {
			return 1
		}
		return say("The battery charges to 100%.\n")
	}
	if err := os.MkdirAll(filepath.Dir(battery.Keep()), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := atomic.Write(battery.Keep(), []byte(battery.KeepLines(folders, limit)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return say(fmt.Sprintf("The battery stops charging at %d%%.\n", limit))
}

func batteryLimit() int {
	value := env.Arg(2)
	if value == "" {
		return showLimit()
	}
	return asRoot("battery-limit "+value, func() int { return setLimit(value) })
}
