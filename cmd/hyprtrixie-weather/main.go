package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/weather"
)

const usage = `Usage: hyprtrixie-weather [update]

Prints the weather saved in ~/.cache/hyprtrixie/weather.json; update
fetches it again from Open-Meteo (no account) for the look settings'
location: your time zone's main city, or hyprtrixie-look location.
hyprtrixie-look weather on fetches it every 30 minutes for the bar.
`

func cache() string {
	return filepath.Join(env.CacheHome(), "hyprtrixie", "weather.json")
}

func look(args ...string) string {
	output, _ := run.Capture(run.Quick, run.Sibling("hyprtrixie-look"), args...)
	return output
}

func update() int {
	coordinates := look("coordinates")
	if coordinates == "" {
		fmt.Fprintln(os.Stderr, "no location: set one with hyprtrixie-look location LATITUDE,LONGITUDE")
		return 1
	}
	url := weather.URL(coordinates, look("get", "weather_units"))
	if err := os.MkdirAll(filepath.Dir(cache()), 0o777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data, err := weather.Fetch(url)
	if err == nil {
		data, err = weather.Stamp(data, time.Now().Unix())
	}
	if err == nil {
		err = atomic.Write(cache(), data, 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not fetch the weather")
		return 1
	}
	return 0
}

func show() int {
	if info, err := os.Stat(cache()); err != nil || info.Size() == 0 {
		fmt.Fprintln(os.Stderr, "no weather yet: hyprtrixie-weather update")
		return 1
	}
	data, err := os.ReadFile(cache())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	summary, err := weather.Summary(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 5
	}
	fmt.Println(summary)
	return 0
}

func main() {
	switch env.Arg(1) {
	case "":
		os.Exit(show())
	case "update":
		os.Exit(update())
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
