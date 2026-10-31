package options

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	choice = "choice"
	yesNo  = "bool"
	number = "number"
	text   = "text"
	multi  = "multi"

	themes   = "catppuccin catppuccin-latte ethereal everforest flexoki-light gruvbox hackerman kanagawa last-horizon lupine matte-black miasma nord northern-sea osaka-jade pitch-black retro-82 ristretto rose-pine solitude tokyo-night white"
	devTools = "docker@developer podman@developer mise@developer python@developer rust@developer go@developer kubernetes@developer neovim@developer vscode@developer vscodium@developer zed@developer toolbox@developer"
	lightAt  = "05:00 06:00 07:00 08:00 09:00"
)

type Option struct {
	Key     string
	Label   string
	kind    string
	initial string
	values  string
}

var profiles = []string{"minimal", "everyday", "developer"}

var table = []Option{
	{"PROFILE", "Profile", choice, "developer", "developer everyday minimal"},
	{"FAILURE_MODE", "When a step fails", choice, "ask", "ask halt continue"},
	{"DEBIAN_MIRROR", "Debian mirror", text, "https://deb.debian.org/debian", ""},
	{"KERNEL", "Kernel", choice, "backports", "backports stable"},
	{"GRAPHICS", "NVIDIA driver", choice, "proprietary", "proprietary nouveau"},
	{"SWAPFILE_GB", "Swapfile size (GiB)", number, "8", ""},
	{"ZRAM_PERCENT", "zram size (% of memory)", number, "50", ""},
	{"SWAPPINESS", "Swappiness", number, "180", ""},
	{"SNAPSHOTS", "System snapshots", yesNo, "yes", ""},
	{"SNAPSHOT_DAILY", "Daily snapshots to keep", number, "7", ""},
	{"SNAPSHOT_WEEKLY", "Weekly snapshots to keep", number, "4", ""},
	{"SNAPSHOT_MONTHLY", "Monthly snapshots to keep", number, "1", ""},
	{"GREETER", "Login screen runs on", choice, "x11", "x11 wayland"},
	{"SHELL_PROMPT", "Prompt", choice, "starship", "starship powerlevel10k"},
	{"APPS", "Applications", multi, "", ""},
	{"TEAMS_SOURCE", "Install Teams from", choice, "apt", "apt snap"},
	{"DEV_TOOLS", "Developer tools", multi, "", devTools},
	{"CONTAINER_ENGINE", "Docker command uses", choice, "docker", "docker podman"},
	{"DOCKER_PORTS", "Docker publishes ports on", choice, "localhost", "localhost all"},
	{"DISPLAY_SCALE", "Display scale", choice, "auto", "auto 1 1.25 1.5 1.75 2"},
	{"NATURAL_SCROLL", "Natural scrolling", yesNo, "no", ""},
	{"TAP_TO_CLICK", "Tap to click", yesNo, "yes", ""},
	{"ANIMATIONS", "Animations", yesNo, "yes", ""},
	{"WINDOW_STYLE", "Window style", choice, "sharp", "sharp soft"},
	{"SCREENSAVER_MINUTES", "Screensaver after (minutes)", number, "3", ""},
	{"TIPS_TERMINAL", "Tips in new terminals", yesNo, "yes", ""},
	{"TIPS_WELCOME", "Tips on the welcome screen", yesNo, "yes", ""},
	{"TIPS_NOTIFY", "Tip notifications", yesNo, "yes", ""},
	{"CARE_NOTIFY", "Firmware and disk warnings", yesNo, "yes", ""},
	{"IDLE_LOCK_MINUTES", "Lock after (minutes)", number, "5", ""},
	{"IDLE_SUSPEND_MINUTES", "Suspend after (minutes)", number, "30", ""},
	{"BAR_PRESET", "Bar layout", choice, "full", "full floating minimal vertical"},
	{"BAR_POSITION", "Bar position", choice, "top", "top bottom"},
	{"CLOCK_FORMAT", "Clock", choice, "24h", "24h 12h"},
	{"THEME", "Theme", choice, "catppuccin", themes},
	{"ACCENT_COLOR", "Accent colour", choice, "theme", "theme red orange yellow green cyan blue magenta"},
	{"GLASS", "Glass mode", yesNo, "no", ""},
	{"ROUNDED_CORNERS", "Rounded screen corners", yesNo, "no", ""},
	{"AUTO_THEME", "Automatic light and dark", choice, "off", "off sun times"},
	{"LIGHT_THEME", "Light theme", choice, "catppuccin-latte", themes},
	{"DARK_THEME", "Dark theme", choice, "catppuccin", themes},
	{"LIGHT_AT", "Light from", choice, "07:00", lightAt},
	{"DARK_AT", "Dark from", choice, "19:00", "17:00 18:00 19:00 20:00 21:00 22:00"},
	{"WALLPAPER_MINUTES", "Change wallpaper every (minutes)", number, "0", ""},
	{"WALLPAPER_PER_WORKSPACE", "A wallpaper per workspace", yesNo, "no", ""},
	{"WEATHER", "Weather in the bar", yesNo, "no", ""},
	{"WEATHER_UNITS", "Weather units", choice, "celsius", "celsius fahrenheit"},
	{"QUIET_HOURS", "Quiet hours", yesNo, "no", ""},
	{"QUIET_FROM", "Quiet from", choice, "22:00", "20:00 21:00 22:00 23:00 00:00"},
	{"QUIET_TO", "Quiet until", choice, "07:00", lightAt},
	{"BOOT_SPLASH", "Boot splash", yesNo, "yes", ""},
	{"BOOT_MENU_THEME", "Boot menu in the theme", yesNo, "yes", ""},
	{"CONSOLE_THEME", "Console colours and font", yesNo, "no", ""},
	{"FIREWALL", "Firewall", yesNo, "yes", ""},
	{"AUTO_UPDATES", "Automatic security updates", yesNo, "yes", ""},
	{"SSH_POLICY", "SSH", choice, "client", "client leave"},
	{"NM_HANDOVER", "NetworkManager for everything", yesNo, "yes", ""},
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

func (option Option) items() []string {
	words := strings.Fields(option.values)
	for index, word := range words {
		words[index], _, _ = strings.Cut(word, "@")
	}
	return words
}

func (option Option) Valid(value string) bool {
	switch option.kind {
	case choice:
		return slices.Contains(option.items(), value)
	case yesNo:
		return value == "yes" || value == "no"
	case number:
		return value == "0" || value != "" && value[0] != '0' && strings.Trim(value, "0123456789") == ""
	case multi:
		items := option.items()
		return !slices.ContainsFunc(strings.Fields(value), func(word string) bool { return !slices.Contains(items, word) })
	}
	return true
}

func (option Option) DefaultFor(profile string) string {
	if option.kind != multi || !strings.Contains(option.values, "@") {
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
