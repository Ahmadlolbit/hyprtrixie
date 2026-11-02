package options

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	Choice = "choice"
	YesNo  = "bool"
	Number = "number"
	Text   = "text"
	Multi  = "multi"

	themes   = "catppuccin catppuccin-latte ethereal everforest flexoki-light gruvbox hackerman kanagawa last-horizon lupine matte-black miasma nord northern-sea osaka-jade pitch-black retro-82 ristretto rose-pine solitude tokyo-night white"
	devTools = "docker@developer podman@developer mise@developer python@developer rust@developer go@developer kubernetes@developer neovim@developer vscode@developer vscodium@developer zed@developer toolbox@developer"
	lightAt  = "05:00 06:00 07:00 08:00 09:00"
)

type Facts struct {
	Value  func(string) string
	Nvidia bool
}

type Option struct {
	Key     string
	Label   string
	Kind    string
	initial string
	values  string
	Section string
	Help    string
	when    func(Facts) bool
}

var profiles = []string{"minimal", "everyday", "developer"}

var table = []Option{
	{"PROFILE", "Profile", Choice, "developer", "developer everyday minimal", "profile", "Presets for apps and tools: minimal, everyday, or developer (everything)", nil},
	{"FAILURE_MODE", "When a step fails", Choice, "ask", "ask halt continue", "run", "Ask what to do, stop the run, or note the failure and carry on", nil},
	{"DEBIAN_MIRROR", "Debian mirror", Text, "https://deb.debian.org/debian", "", "system", "Where Debian packages download from; the default picks a nearby server", nil},
	{"KERNEL", "Kernel", Choice, "backports", "backports stable", "system", "Backports has a newer kernel for recent laptops; stable is Debian's own", nil},
	{"GRAPHICS", "NVIDIA driver", Choice, "proprietary", "proprietary nouveau", "system", "NVIDIA's own driver is faster; nouveau is open source and slower", nvidiaPresent},
	{"SWAPFILE_GB", "Swapfile size (GiB)", Number, "8", "", "storage", "Disk space used as extra memory when RAM runs out; 0 turns it off", nil},
	{"ZRAM_PERCENT", "zram size (% of memory)", Number, "50", "", "storage", "Compressed memory that works as fast swap; 0 turns it off", nil},
	{"SWAPPINESS", "Swappiness", Number, "180", "", "storage", "How readily memory moves to swap; a high value suits zram", nil},
	{"SNAPSHOTS", "System snapshots", YesNo, "yes", "", "storage", "Snapshots before changes, so you can roll back from the boot menu", nil},
	{"SNAPSHOT_DAILY", "Daily snapshots to keep", Number, "7", "", "storage", "How many daily snapshots to keep", snapshotsOn},
	{"SNAPSHOT_WEEKLY", "Weekly snapshots to keep", Number, "4", "", "storage", "How many weekly snapshots to keep", snapshotsOn},
	{"SNAPSHOT_MONTHLY", "Monthly snapshots to keep", Number, "1", "", "storage", "How many monthly snapshots to keep", snapshotsOn},
	{"GREETER", "Login screen runs on", Choice, "x11", "x11 wayland", "system", "X11 is the most reliable for the login screen; Wayland is newer", nil},
	{"SHELL_PROMPT", "Prompt", Choice, "starship", "starship powerlevel10k", "shell", "How the terminal prompt looks: Starship or Powerlevel10k", nil},
	{"APPS", "Applications", Multi, "", "", "apps", "Apps to install, by category; untick any you don't want", nil},
	{"TEAMS_SOURCE", "Install Teams from", Choice, "apt", "apt snap", "apps", "Teams for Linux from its apt repository or from the Snap Store", teamsWanted},
	{"DEV_TOOLS", "Developer tools", Multi, "", devTools, "dev", "Languages, containers and editors to set up for programming", nil},
	{"CONTAINER_ENGINE", "Docker command uses", Choice, "docker", "docker podman", "dev", "When both are installed, which engine answers the docker command", bothEngines},
	{"DOCKER_PORTS", "Docker publishes ports on", Choice, "localhost", "localhost all", "dev", "Container ports reach only this computer, or every network", dockerWanted},
	{"DISPLAY_SCALE", "Display scale", Choice, "auto", "auto 1 1.25 1.5 1.75 2", "input", "Size of everything on screen; auto picks it from the screen's density", nil},
	{"NATURAL_SCROLL", "Natural scrolling", YesNo, "no", "", "input", "Content follows your fingers on the touchpad, like on a phone", nil},
	{"TAP_TO_CLICK", "Tap to click", YesNo, "yes", "", "input", "Tap the touchpad to click instead of pressing it down", nil},
	{"ANIMATIONS", "Animations", YesNo, "yes", "", "look", "Windows and workspaces animate as they open, close and move", nil},
	{"WINDOW_STYLE", "Window style", Choice, "sharp", "sharp soft", "look", "Sharp corners with thin gaps, or soft rounded corners", nil},
	{"SCREENSAVER_MINUTES", "Screensaver after (minutes)", Number, "3", "", "system", "An animated logo on every screen; a key or mouse move ends it; 0 = never", nil},
	{"TIPS_TERMINAL", "Tips in new terminals", YesNo, "yes", "", "look", "A short tip under the prompt in every new terminal", nil},
	{"TIPS_WELCOME", "Tips on the welcome screen", YesNo, "yes", "", "look", "A tip at the bottom of the welcome screen", nil},
	{"TIPS_NOTIFY", "Tip notifications", YesNo, "yes", "", "look", "Now and then a tip as a notification, at most once a day", nil},
	{"CARE_NOTIFY", "Firmware and disk warnings", YesNo, "yes", "", "system", "A notification when new firmware is ready, or when the disk is nearly full", nil},
	{"IDLE_LOCK_MINUTES", "Lock after (minutes)", Number, "5", "", "system", "Lock the screen when you're away; it turns off 30 s later; 0 = never", nil},
	{"IDLE_SUSPEND_MINUTES", "Suspend after (minutes)", Number, "30", "", "system", "Put the laptop to sleep when you're away; 0 = never", nil},
	{"BAR_PRESET", "Bar layout", Choice, "full", "full floating minimal vertical", "look", "Full width, floating with rounded corners, minimal, or down the left side", nil},
	{"BAR_POSITION", "Bar position", Choice, "top", "top bottom", "look", "The bar at the top or the bottom of the screen", barHorizontal},
	{"CLOCK_FORMAT", "Clock", Choice, "24h", "24h 12h", "look", "24-hour or 12-hour time in the bar and on the lock screen", nil},
	{"THEME", "Theme", Choice, "catppuccin", themes, "look", "Colours, wallpaper and icons for every app; Super+Ctrl+T changes it later", nil},
	{"ACCENT_COLOR", "Accent colour", Choice, "theme", "theme red orange yellow green cyan blue magenta", "look", "The theme's own accent, or another of its colours, for borders and highlights", nil},
	{"GLASS", "Glass mode", YesNo, "no", "", "look", "See-through terminals and panels, with the wallpaper blurred behind them", nil},
	{"ROUNDED_CORNERS", "Rounded screen corners", YesNo, "no", "", "look", "Round the corners of the screen, like a phone's", nil},
	{"AUTO_THEME", "Automatic light and dark", Choice, "off", "off sun times", "look", "A light theme by day and a dark one at night: by the sun or at set times", nil},
	{"LIGHT_THEME", "Light theme", Choice, "catppuccin-latte", themes, "look", "The theme for the day", autoThemeOn},
	{"DARK_THEME", "Dark theme", Choice, "catppuccin", themes, "look", "The theme for the night", autoThemeOn},
	{"LIGHT_AT", "Light from", Choice, "07:00", lightAt, "look", "When the light theme starts", autoThemeTimes},
	{"DARK_AT", "Dark from", Choice, "19:00", "17:00 18:00 19:00 20:00 21:00 22:00", "look", "When the dark theme starts", autoThemeTimes},
	{"WALLPAPER_MINUTES", "Change wallpaper every (minutes)", Number, "0", "", "look", "Move to the theme's next wallpaper every so many minutes; 0 keeps one", nil},
	{"WALLPAPER_PER_WORKSPACE", "A wallpaper per workspace", YesNo, "no", "", "look", "Each workspace shows its own wallpaper from the theme", nil},
	{"WEATHER", "Weather in the bar", YesNo, "no", "", "look", "From Open-Meteo, no account; it sends your time zone's main city there", nil},
	{"WEATHER_UNITS", "Weather units", Choice, "celsius", "celsius fahrenheit", "look", "Degrees Celsius and km/h, or Fahrenheit and mph", weatherOn},
	{"QUIET_HOURS", "Quiet hours", YesNo, "no", "", "look", "Do not disturb every night, for the hours below", nil},
	{"QUIET_FROM", "Quiet from", Choice, "22:00", "20:00 21:00 22:00 23:00 00:00", "look", "When notifications go quiet", quietOn},
	{"QUIET_TO", "Quiet until", Choice, "07:00", lightAt, "look", "When notifications come back", quietOn},
	{"BOOT_SPLASH", "Boot splash", YesNo, "yes", "", "look", "The logo and the disk password prompt in your theme's colours while it boots", nil},
	{"BOOT_MENU_THEME", "Boot menu in the theme", YesNo, "yes", "", "look", "The boot menu, where the snapshots are, in your theme's colours", nil},
	{"CONSOLE_THEME", "Console colours and font", YesNo, "no", "", "look", "The text consoles (Ctrl+Alt+F3) in your theme's colours, with a larger font", nil},
	{"FIREWALL", "Firewall", YesNo, "yes", "", "security", "Blocks connections from other computers; your own connections still work", nil},
	{"AUTO_UPDATES", "Automatic security updates", YesNo, "yes", "", "security", "Debian's security fixes install on their own in the background", nil},
	{"SSH_POLICY", "SSH", Choice, "client", "client leave", "security", "Client: the SSH server doesn't start at boot. Leave: keep it as it is", nil},
	{"NM_HANDOVER", "NetworkManager for everything", YesNo, "yes", "", "security", "Wi-Fi and wired connections move to NetworkManager, so the bar manages them", nil},
}

func ticked(facts Facts, key, item string) bool {
	return slices.Contains(strings.Fields(facts.Value(key)), item)
}

func nvidiaPresent(facts Facts) bool {
	return facts.Nvidia
}

func snapshotsOn(facts Facts) bool {
	return facts.Value("SNAPSHOTS") == "yes"
}

func teamsWanted(facts Facts) bool {
	return ticked(facts, "APPS", "teams")
}

func dockerWanted(facts Facts) bool {
	return ticked(facts, "DEV_TOOLS", "docker")
}

func bothEngines(facts Facts) bool {
	return dockerWanted(facts) && ticked(facts, "DEV_TOOLS", "podman")
}

func barHorizontal(facts Facts) bool {
	return facts.Value("BAR_PRESET") != "vertical"
}

func autoThemeOn(facts Facts) bool {
	return facts.Value("AUTO_THEME") != "off"
}

func autoThemeTimes(facts Facts) bool {
	return facts.Value("AUTO_THEME") == "times"
}

func weatherOn(facts Facts) bool {
	return facts.Value("WEATHER") == "yes"
}

func quietOn(facts Facts) bool {
	return facts.Value("QUIET_HOURS") == "yes"
}

func Themes() []string {
	return strings.Fields(themes)
}

func All() []Option {
	all := slices.Clone(table)
	for index := range all {
		if all[index].Key == "APPS" {
			all[index].values = appValues(Catalogue())
		}
	}
	return all
}

func Catalogue() string {
	path := filepath.Join(env.Manifests(), "apps")
	if !fsutil.IsFile(path) {
		return ""
	}
	return fsutil.Optional(path)
}

func appValues(text string) string {
	var values []string
	for _, line := range strings.Split(text, "\n") {
		if fields := strings.Fields(line); len(fields) >= 6 {
			values = append(values, fields[0]+"@"+fields[2])
		}
	}
	return strings.Join(values, " ")
}

func (option Option) Items() []string {
	words := strings.Fields(option.values)
	for index, word := range words {
		words[index], _, _ = strings.Cut(word, "@")
	}
	return words
}

func (option Option) Valid(value string) bool {
	switch option.Kind {
	case Choice:
		return slices.Contains(option.Items(), value)
	case YesNo:
		return value == "yes" || value == "no"
	case Number:
		return value == "0" || value != "" && value[0] != '0' && strings.Trim(value, "0123456789") == ""
	case Multi:
		items := option.Items()
		return !slices.ContainsFunc(strings.Fields(value), func(word string) bool { return !slices.Contains(items, word) })
	}
	return true
}

func (option Option) Preset() bool {
	return option.Kind == Multi && strings.Contains(option.values, "@")
}

func (option Option) Shows(facts Facts) bool {
	return option.when == nil || option.when(facts)
}

func (option Option) DefaultFor(profile string) string {
	if !option.Preset() {
		return option.initial
	}
	var chosen []string
	for _, word := range strings.Fields(option.values) {
		name, tag, tagged := strings.Cut(word, "@")
		if !tagged {
			tag = profiles[0]
		}
		if tag != "offered" && level(tag) <= level(profile) {
			chosen = append(chosen, name)
		}
	}
	return strings.Join(chosen, " ")
}

func level(profile string) int {
	if index := slices.Index(profiles, profile); index >= 0 {
		return index
	}
	return len(profiles) - 1
}
