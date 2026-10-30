package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/look"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

const usage = `Usage: hyprtrixie-look
       hyprtrixie-look accent [theme|red|orange|yellow|green|cyan|blue|magenta|#rrggbb]
       hyprtrixie-look font [FAMILY|default]
       hyprtrixie-look ui-font [FAMILY|default]
       hyprtrixie-look glass [on|off|toggle]
       hyprtrixie-look corners [on|off|toggle]
       hyprtrixie-look auto [off|sun|times|toggle]
       hyprtrixie-look light|dark [THEME]
       hyprtrixie-look times [LIGHT_FROM DARK_FROM]
       hyprtrixie-look sun [DATE]
       hyprtrixie-look location [LATITUDE,LONGITUDE|zone]
       hyprtrixie-look rotate [MINUTES|off|toggle]
       hyprtrixie-look workspaces [on|off|toggle]
       hyprtrixie-look bar [full|floating|minimal|vertical]
       hyprtrixie-look video [FILE|off]
       hyprtrixie-look video-battery [pause|play]
       hyprtrixie-look weather [on|off|toggle]
       hyprtrixie-look weather-units [celsius|fahrenheit]
       hyprtrixie-look coordinates
       hyprtrixie-look quiet [on|off|toggle]
       hyprtrixie-look quiet-hours [FROM TO]
       hyprtrixie-look get SETTING

Shows and changes how the desktop looks. Your settings are kept in
~/.config/hyprtrixie/look.conf and win over the setup menu's in
/etc/hyprtrixie/look.conf. With no value, a command prints the setting.
An accent named after a colour uses that colour of each theme. font is
the terminal's, the bar's and the lock screen's; ui-font is the apps'.
Glass makes terminals and panels see-through, with blur behind them.
corners rounds the screen's corners.
auto switches to the light theme by day and the dark one at night, at
sunrise and sunset (sun) or at set times like 07:00 and 19:00 (times).
While it's on, a theme you pick becomes the light or the dark one.
sun prints the day's sunrise and sunset where your time zone's main city
is, or at the location you set. rotate changes the wallpaper every so
many minutes; toggle switches between off and every 30 minutes.
workspaces gives each workspace its own wallpaper. bar sets the bar's
layout in ~/.config/hyprtrixie/shell.json. video plays a video file as
the wallpaper, muted and looping; on battery it pauses, unless
video-battery is play. weather shows the weather in the bar from
Open-Meteo for your location (it sends that location there); coordinates
prints the location the sun and the weather use. quiet turns do not
disturb on for the quiet hours (22:00 to 07:00 unless you set others).
`

const (
	accents    = " theme red orange yellow green cyan blue magenta "
	readAccess = 4
)

var (
	settings   = look.Open()
	systemRoot = env.Or("OMARCHY_PATH", "/usr/share/omarchy")
)

func stateHome(parts ...string) string {
	return filepath.Join(append([]string{env.StateHome()}, parts...)...)
}

func currentTheme() string {
	name, _ := fsutil.Text(stateHome("omarchy", "current", "theme.name"))
	return name
}

func put(key, value string) bool {
	if err := settings.Put(key, value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	return true
}

func show(key string) int {
	fmt.Println(settings.Get(key))
	return 0
}

func arg(args []string, index int) string {
	if index < len(args) {
		return args[index]
	}
	return ""
}

func reapply() int {
	name := currentTheme()
	if name == "" {
		return 0
	}
	return run.Within(run.Slow, run.Sibling("hyprtrixie-theme-set"), name)
}

func accent(args []string) int {
	value := strings.ToLower(arg(args, 0))
	if value == "" {
		return show("accent")
	}
	if !strings.Contains(accents, " "+value+" ") && !theme.IsHex(value) {
		fmt.Fprintf(os.Stderr, "not an accent: %s (theme, a colour name or #rrggbb)\n", arg(args, 0))
		return 2
	}
	return putThen("accent", value, reapply)()
}

func fontInstalled(name string) bool {
	if _, err := exec.LookPath("fc-list"); err != nil {
		return true
	}
	family := ":family=" + strings.ReplaceAll(name, "-", `\-`)
	output, _ := run.CaptureQuiet(run.Slow, "fc-list", "--format", `%{family[0]}\n`, family)
	return output != ""
}

func font(key, value string) int {
	if value == "" {
		return show(key)
	}
	if value == "default" {
		value, _ = look.Default(key)
	} else if !theme.ValidFont(value) || !fontInstalled(value) {
		fmt.Fprintf(os.Stderr, "no installed font family named %s (see: fc-list : family)\n", value)
		return 2
	}
	if !put(key, value) {
		return 1
	}
	return reapply()
}

type onOff struct {
	key     string
	on, off func() int
}

func (item onOff) run(args []string) int {
	switch strings.ToLower(arg(args, 0)) {
	case "":
		return show(item.key)
	case "on", "yes":
		return item.on()
	case "off", "no":
		return item.off()
	case "toggle":
		if settings.Get(item.key) == "yes" {
			return item.off()
		}
		return item.on()
	}
	fmt.Fprintln(os.Stderr, item.key+" takes on, off or toggle")
	return 2
}

func putThen(key, value string, then func() int) func() int {
	return func() int {
		if !put(key, value) {
			return 1
		}
		return then()
	}
}

func nothing() int {
	return 0
}

func reload() int {
	look.ReloadShell()
	return 0
}

func glass(args []string) int {
	return onOff{"glass", putThen("glass", "yes", reapply), putThen("glass", "no", reapply)}.run(args)
}

func corners(args []string) int {
	return onOff{"corners", putThen("corners", "yes", reload), putThen("corners", "no", reload)}.run(args)
}

func background(args ...string) int {
	return run.Within(run.Slow, run.Sibling("hyprtrixie-background"), args...)
}

func flatWallpaper() int {
	shown, err := os.Readlink(stateHome("omarchy", "current", "background"))
	if err != nil || shown == "" {
		return 0
	}
	return background(shown)
}

func workspaces(args []string) int {
	on := putThen("workspaces", "yes", func() int { return background("workspaces") })
	return onOff{"workspaces", on, putThen("workspaces", "no", flatWallpaper)}.run(args)
}

func video(args []string) int {
	value := arg(args, 0)
	if value == "" {
		return show("video")
	}
	if value != "off" {
		file, ok := videoFile(value)
		if !ok {
			fmt.Fprintf(os.Stderr, "not a video file this can play: %s (mp4, webm, mkv, mov or m4v)\n", value)
			return 2
		}
		value = file
	}
	return putThen("video", value, reload)()
}

func videoFile(path string) (string, bool) {
	file, err := fsutil.RealPath(path)
	if err != nil || !fsutil.IsFile(file) || syscall.Access(file, readAccess) != nil {
		return "", false
	}
	switch strings.ToLower(filepath.Ext(file)) {
	case ".mp4", ".webm", ".mkv", ".mov", ".m4v":
		return file, true
	}
	return "", false
}

func videoBattery(args []string) int {
	value := arg(args, 0)
	switch value {
	case "":
		return show("video_battery")
	case "pause", "play":
		return putThen("video_battery", value, reload)()
	}
	fmt.Fprintln(os.Stderr, "video-battery takes pause or play")
	return 2
}

func auto(args []string) int {
	value := strings.ToLower(arg(args, 0))
	switch value {
	case "":
		return show("auto")
	case "off":
		return putThen("auto", "off", nothing)()
	case "sun", "times":
		return putThen("auto", value, retick)()
	case "toggle":
		if settings.Get("auto") == "off" {
			return auto([]string{"sun"})
		}
		return auto([]string{"off"})
	}
	fmt.Fprintln(os.Stderr, "auto takes off, sun, times or toggle")
	return 2
}

func themeChoice(key, value string) int {
	if value == "" {
		return show(key)
	}
	name := theme.Slug(value)
	if !theme.ValidName(name) || !themeExists(name) {
		fmt.Fprintf(os.Stderr, "no theme named %s; see hyprtrixie-theme-set --list\n", value)
		return 2
	}
	return putThen(key, name, retick)()
}

func themeExists(name string) bool {
	return fsutil.IsDir(filepath.Join(systemRoot, "themes", name)) || fsutil.IsDir(filepath.Join(env.ConfigHome(), "omarchy", "themes", name))
}

func validTimes(args []string) bool {
	if len(args) != 2 {
		return false
	}
	_, first := look.Minutes(args[0])
	_, second := look.Minutes(args[1])
	return first && second
}

func times(args []string) int {
	if len(args) == 0 {
		fmt.Println(settings.Get("light_at"), settings.Get("dark_at"))
		return 0
	}
	if !validTimes(args) {
		fmt.Fprintln(os.Stderr, "times takes two times like 07:00 19:00: when light starts, then dark")
		return 2
	}
	if !put("light_at", args[0]) || !put("dark_at", args[1]) {
		return 1
	}
	return retick()
}

func location(args []string) int {
	value := arg(args, 0)
	if value == "" {
		return show("location")
	}
	if !look.ValidLocation(value) {
		fmt.Fprintln(os.Stderr, "a location is LATITUDE,LONGITUDE in degrees, like 33.31,44.36, or zone")
		return 2
	}
	return putThen("location", value, nothing)()
}

func remember(args []string) int {
	name := arg(args, 0)
	if name == "" {
		return 2
	}
	key := "dark"
	if arg(args, 1) == "light" {
		key = "light"
	}
	return putThen(key, name, nothing)()
}

func rotate(args []string) int {
	every := strings.ToLower(arg(args, 0))
	switch every {
	case "":
		return show("rotate")
	case "off":
		every = "0"
	case "toggle":
		every = "30"
		if settings.Get("rotate") != "0" {
			every = "0"
		}
	}
	minutes, ok := look.WholeNumber(every)
	if !ok || minutes > 1440 {
		fmt.Fprintln(os.Stderr, "rotate takes minutes from 0 to 1440, off or toggle")
		return 2
	}
	remove(lookState("rotated"))
	return putThen("rotate", fmt.Sprint(minutes), func() int {
		rotateTick()
		return 0
	})()
}

func getSetting(args []string) int {
	key := arg(args, 0)
	if _, known := look.Default(key); !known {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	return show(key)
}

func all() int {
	for _, key := range look.Keys {
		fmt.Printf("%s=%s\n", key, settings.Get(key))
	}
	return 0
}

func commands() map[string]func([]string) int {
	return map[string]func([]string) int{
		"get": getSetting, "accent": accent, "glass": glass, "corners": corners, "sun": sun, "auto": auto, "times": times,
		"tick": tick, "remember": remember, "rotate": rotate, "workspaces": workspaces, "bar": barLayout, "video": video,
		"video-battery": videoBattery, "weather": weather, "weather-units": weatherUnits, "coordinates": showCoordinates,
		"quiet": quiet, "quiet-hours": quietHours, "location": location,
		"font":    func(args []string) int { return font("font", arg(args, 0)) },
		"ui-font": func(args []string) int { return font("ui_font", arg(args, 0)) },
		"light":   func(args []string) int { return themeChoice("light", arg(args, 0)) },
		"dark":    func(args []string) int { return themeChoice("dark", arg(args, 0)) },
	}
}

func main() {
	name := env.Arg(1)
	if name == "" {
		os.Exit(all())
	}
	if name == "-h" || name == "--help" {
		fmt.Print(usage)
		return
	}
	command, known := commands()[name]
	if !known {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	os.Exit(command(os.Args[2:]))
}
