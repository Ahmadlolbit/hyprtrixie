package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
	nightLight   = "hyprsunset"
	warmest      = 1000
	coolest      = 6500
	dailyHour    = 6
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
	return run.Within(run.Slow, run.Sibling("hyprtrixie-theme-set"), wanted)
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

func nightNow() (wanted, known bool) {
	switch settings.Get("night") {
	case "on":
		return true, true
	case "times":
		at, ok := moment()
		if !ok {
			return false, false
		}
		return look.Inside(minuteOfDay(at), minuteSetting("night_from"), minuteSetting("night_to")), true
	}
	return false, false
}

func nightTick() {
	wanted, known := nightNow()
	if !known || wanted == (run.Within(run.Quick, "pgrep", "-x", nightLight) == 0) {
		return
	}
	if wanted {
		run.Quiet(run.Quick, "setsid", "-f", nightLight, "-t", settings.Get("night_warmth"))
		return
	}
	run.Within(run.Quick, "pkill", "-x", nightLight)
}

func nightOn() int {
	nightTick()
	return 0
}

func nightOff() int {
	run.Within(run.Quick, "pkill", "-x", nightLight)
	return 0
}

func night(args []string) int {
	value := strings.ToLower(arg(args, 0))
	if value == "toggle" {
		value = "on"
		if settings.Get("night") != "off" {
			value = "off"
		}
	}
	switch value {
	case "":
		return show("night")
	case "on", "times":
		return putThen("night", value, nightOn)()
	case "off":
		return putThen("night", value, nightOff)()
	}
	fmt.Fprintln(os.Stderr, "night takes on, off, times or toggle")
	return 2
}

func nightHours(args []string) int {
	if len(args) == 0 {
		fmt.Println(settings.Get("night_from"), settings.Get("night_to"))
		return 0
	}
	if !validTimes(args) {
		fmt.Fprintln(os.Stderr, "night-hours takes two times like 20:00 07:00: when it starts, then ends")
		return 2
	}
	if !put("night_from", args[0]) || !put("night_to", args[1]) {
		return 1
	}
	return nightOn()
}

func nightWarmth(args []string) int {
	value := arg(args, 0)
	if value == "" {
		return show("night_warmth")
	}
	kelvin, err := strconv.Atoi(value)
	if err != nil || kelvin < warmest || kelvin > coolest {
		fmt.Fprintf(os.Stderr, "night-warmth takes a colour temperature from %d to %d\n", warmest, coolest)
		return 2
	}
	if !put("night_warmth", value) {
		return 1
	}
	if run.Within(run.Quick, "pkill", "-x", nightLight) == 0 {
		run.Quiet(3*time.Second, "pidwait", "-x", nightLight)
	}
	return nightOn()
}

func themeAfter(current string) string {
	output, err := run.CaptureQuiet(run.Slow, run.Sibling("hyprtrixie-theme-set"), "--previews")
	if err != nil {
		return ""
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if name, _, found := strings.Cut(line, "\t"); found {
			names = append(names, name)
		}
	}
	if len(names) < 2 {
		return ""
	}
	return names[(slices.Index(names, current)+1)%len(names)]
}

func dailyTick() {
	if settings.Get("daily") != "yes" || settings.Get("auto") != "off" {
		return
	}
	at, ok := moment()
	if !ok || at.Hour() < dailyHour {
		return
	}
	today, last := at.Format(time.DateOnly), stored(lookState("daily"))
	if last == today {
		return
	}
	save(lookState("daily"), today)
	if next := themeAfter(currentTheme()); last != "" && next != "" {
		run.Within(run.Slow, run.Sibling("hyprtrixie-theme-set"), next)
	}
}

func tick([]string) int {
	autoTick()
	dailyTick()
	rotateTick()
	weatherTick()
	quietTick()
	nightTick()
	return 0
}
