package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/look"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

var moment = sync.OnceValues(look.Now)

const (
	noLocation   = "no location: set one with hyprtrixie-look location LATITUDE,LONGITUDE"
	weatherStale = 1800
	weatherLimit = time.Minute
	doNotDisturb = "do-not-disturb"
)

func lookState(name string) string {
	return stateHome("hyprtrixie", "look", name)
}

func quietStamp() string {
	return filepath.Join(env.Or("XDG_RUNTIME_DIR", stateHome("hyprtrixie", "look")), "hyprtrixie-quiet")
}

func weatherCache() string {
	return filepath.Join(env.CacheHome(), "hyprtrixie", "weather.json")
}

func remove(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, err)
	}
}

func stored(path string) string {
	value, _ := fsutil.Text(path)
	return value
}

func save(path, value string) {
	err := os.MkdirAll(stateHome("hyprtrixie", "look"), 0o777)
	if err == nil {
		err = atomic.Write(path, []byte(value+"\n"), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func showCoordinates([]string) int {
	value, ok := settings.Coordinates()
	if !ok {
		return 1
	}
	fmt.Println(value)
	return 0
}

func parsedCoordinates() (float64, float64, bool) {
	value, ok := settings.Coordinates()
	if !ok {
		return 0, 0, false
	}
	first, second, _ := strings.Cut(value, ",")
	latitude, err := strconv.ParseFloat(first, 64)
	if err != nil {
		return 0, 0, false
	}
	longitude, err := strconv.ParseFloat(second, 64)
	return latitude, longitude, err == nil
}

func dayOf(date string, at time.Time) (int, int, bool) {
	if date != "" {
		return look.DayOfDate(date)
	}
	day, offset := look.DayOf(at)
	return day, offset, true
}

func sunTimes(date string, at time.Time) (int, int, bool) {
	latitude, longitude, ok := parsedCoordinates()
	if !ok {
		return 0, 0, false
	}
	day, offset, ok := dayOf(date, at)
	if !ok {
		return 0, 0, false
	}
	rise, set := look.SunMinutes(latitude, longitude, day, offset)
	return rise, set, true
}

func sun(args []string) int {
	rise, set, ok := sunTimes(arg(args, 0), time.Now())
	if !ok {
		fmt.Fprintln(os.Stderr, noLocation)
		return 1
	}
	fmt.Printf("sunrise %s\nsunset %s\n", look.Clock(rise), look.Clock(set))
	return 0
}

func minuteSetting(key string) int {
	if value, ok := look.Minutes(settings.Get(key)); ok {
		return value
	}
	fallback, _ := look.Default(key)
	value, _ := look.Minutes(fallback)
	return value
}

func minuteOfDay(at time.Time) int {
	return at.Hour()*60 + at.Minute()
}

func period(mode string) (string, bool) {
	at, ok := moment()
	if !ok {
		return "", false
	}
	start, end, found := 0, 0, false
	if mode == "sun" {
		start, end, found = sunTimes("", at)
	}
	if !found {
		start, end = minuteSetting("light_at"), minuteSetting("dark_at")
	}
	if look.Inside(minuteOfDay(at), start, end) {
		return "day", true
	}
	return "night", true
}

func autoTick() int {
	mode := settings.Get("auto")
	if mode != "sun" && mode != "times" {
		return 0
	}
	now, ok := period(mode)
	if !ok || stored(lookState("period")) == now {
		return 0
	}
	save(lookState("period"), now)
	wanted := settings.Get("dark")
	if now == "day" {
		wanted = settings.Get("light")
	}
	if currentTheme() == wanted {
		return 0
	}
	if err := os.Setenv("HYPRTRIXIE_LOOK_AUTO", "1"); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return run.Within(run.Slow, run.Sibling("omarchy-theme-set"), wanted)
}

func retick() int {
	remove(lookState("period"))
	return autoTick()
}

func rotateTick() {
	every, ok := look.WholeNumber(settings.Get("rotate"))
	if !ok || every == 0 {
		return
	}
	at, ok := moment()
	if !ok {
		return
	}
	last, known := look.WholeNumber(stored(lookState("rotated")))
	if known && at.Unix()-last < every*60 {
		return
	}
	save(lookState("rotated"), strconv.FormatInt(at.Unix(), 10))
	if known {
		run.Capture(run.Slow, run.Sibling("hyprtrixie-background"), "next")
	}
}

func weatherFresh() bool {
	info, err := os.Stat(weatherCache())
	if err != nil || info.Size() == 0 {
		return false
	}
	at, ok := moment()
	return ok && at.Unix()-info.ModTime().Unix() < weatherStale
}

func weatherTick() {
	if settings.Get("weather") == "yes" && !weatherFresh() {
		run.Quiet(weatherLimit, run.Sibling("hyprtrixie-weather"), "update")
	}
}

func updateWeather() int {
	return run.Within(weatherLimit, run.Sibling("hyprtrixie-weather"), "update")
}

func dropWeather() int {
	remove(weatherCache())
	return 0
}

func weather(args []string) int {
	on, off := putThen("weather", "yes", updateWeather), putThen("weather", "no", dropWeather)
	return onOff{"weather", on, off}.run(args)
}

func weatherUnits(args []string) int {
	value := arg(args, 0)
	switch value {
	case "":
		return show("weather_units")
	case "celsius", "fahrenheit":
		return putThen("weather_units", value, func() int {
			if settings.Get("weather") != "yes" {
				return 0
			}
			return updateWeather()
		})()
	}
	fmt.Fprintln(os.Stderr, "weather-units takes celsius or fahrenheit")
	return 2
}

func quietNow() (string, bool) {
	at, ok := moment()
	if !ok {
		return "", false
	}
	if look.Inside(minuteOfDay(at), minuteSetting("quiet_from"), minuteSetting("quiet_to")) {
		return "quiet", true
	}
	return "loud", true
}

func quietTick() {
	if settings.Get("quiet") != "yes" {
		return
	}
	state, ok := quietNow()
	if !ok || stored(quietStamp()) == state {
		return
	}
	save(quietStamp(), state)
	if state == "quiet" {
		run.Quiet(run.Quick, "makoctl", "mode", "-a", doNotDisturb)
		return
	}
	run.Quiet(run.Quick, "makoctl", "mode", "-r", doNotDisturb)
}

func quietOn() int {
	remove(quietStamp())
	quietTick()
	return 0
}

func quietOff() int {
	if stored(quietStamp()) == "quiet" {
		run.Quiet(run.Quick, "makoctl", "mode", "-r", doNotDisturb)
	}
	remove(quietStamp())
	return 0
}

func quiet(args []string) int {
	on, off := putThen("quiet", "yes", quietOn), putThen("quiet", "no", quietOff)
	return onOff{"quiet", on, off}.run(args)
}

func quietHours(args []string) int {
	if len(args) == 0 {
		fmt.Println(settings.Get("quiet_from"), settings.Get("quiet_to"))
		return 0
	}
	if !validTimes(args) {
		fmt.Fprintln(os.Stderr, "quiet-hours takes two times like 22:00 07:00: when it starts, then ends")
		return 2
	}
	if !put("quiet_from", args[0]) || !put("quiet_to", args[1]) {
		return 1
	}
	return quietOn()
}

func tick([]string) int {
	autoTick()
	rotateTick()
	weatherTick()
	quietTick()
	return 0
}
