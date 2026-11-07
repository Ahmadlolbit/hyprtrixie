package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/look"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

const usage = `Usage: hyprtrixie-theme-set <theme>
       hyprtrixie-theme-set --list
       hyprtrixie-theme-set --previews
       hyprtrixie-theme-set --current
       hyprtrixie-theme-set --editors

Applies a theme to Hyprland, the bar, kitty, the launcher, notifications, the
lock screen, btop, Neovim, VS Code, VSCodium, Zed, Firefox, GTK and KDE
apps, and the wallpaper. Themes come from /usr/share/hyprtrixie/themes and
~/.config/hyprtrixie/themes. --editors gives the editors and Firefox's
profiles the current theme again, for one installed or made since.
`

const (
	denied     = " alacritty.toml fastfetch.jsonc firefox.css foot.ini ghostty.conf hyprlock.conf kde.colors kitty.conf mako.ini vscode.json "
	wallpapers = "--wallpapers-soon"
)

var (
	systemRoot = env.ThemeRoot()
	iconName   = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	settings   = look.Open()
)

func state() string {
	return env.ThemeState()
}

func userThemes() string {
	return filepath.Join(env.AppConfig(), "themes")
}

func themesIn(dir string) []string {
	var names []string
	for _, entry := range fsutil.Entries(dir) {
		if (entry.IsDir() || entry.Type()&os.ModeSymlink != 0) && theme.ValidName(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	return names
}

func themeNames() []string {
	names := append(themesIn(filepath.Join(systemRoot, "themes")), themesIn(userThemes())...)
	return slices.Compact(run.Sorted(names))
}

func previewOf(name string) string {
	own, shipped := filepath.Join(userThemes(), name), filepath.Join(systemRoot, "themes", name)
	for _, dir := range []string{own, shipped} {
		for _, kind := range []string{"png", "jpg", "jpeg", "webp"} {
			if file := filepath.Join(dir, "preview."+kind); fsutil.IsFile(file) {
				return file
			}
		}
	}
	for _, dir := range []string{own, shipped} {
		if found := fsutil.Images(filepath.Join(dir, "backgrounds")); len(found) > 0 {
			return found[0]
		}
	}
	return ""
}

func stageOwn(own, next string) error {
	entries, err := os.ReadDir(own)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := stageEntry(own, next, entry); err != nil {
			return err
		}
	}
	return nil
}

func stageEntry(own, next string, entry fs.DirEntry) error {
	base := entry.Name()
	source, target := filepath.Join(own, base), filepath.Join(next, base)
	switch {
	case strings.HasPrefix(base, "."):
		return nil
	case strings.HasSuffix(base, ".lua") || strings.Contains(denied, " "+base+" "):
		fmt.Fprintf(os.Stderr, "ignored %s from %s: your themes can't bring code or settings that run commands\n", base, own)
		return nil
	case entry.Type()&os.ModeSymlink != 0:
		return stageLink(source, target)
	}
	return fsutil.CopyEntry(source, target)
}

func stageLink(source, target string) error {
	if !fsutil.IsFile(source) {
		fmt.Fprintf(os.Stderr, "ignored %s: it links to something other than a file\n", source)
		return nil
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return atomic.Install(target, data, 0o644)
}

func stage(name, next string) error {
	if err := os.RemoveAll(next); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if err := os.MkdirAll(next, 0o777); err != nil {
		return err
	}
	if shipped := filepath.Join(systemRoot, "themes", name); fsutil.IsDir(shipped) {
		if err := fsutil.CopyTree(shipped, next); err != nil {
			return err
		}
	}
	own := filepath.Join(userThemes(), name)
	if !fsutil.IsDir(own) {
		return nil
	}
	return stageOwn(own, next)
}

func overrideAccent(colors theme.Colors) {
	if accent, ok := colors.Accent(settings.Get("accent")); ok {
		colors["theme_accent"], colors["accent"] = colors["accent"], accent
	}
}

func glassSettings(colors theme.Colors) {
	if settings.Get("glass") == "yes" {
		colors["glass"], colors["opacity"], colors["panel_alpha"] = "yes", "0.88", "cc"
		return
	}
	colors["glass"], colors["opacity"], colors["panel_alpha"] = "no", "1.0", "ee"
}

func styleSettings(colors theme.Colors) {
	colors["rounding"] = "0"
	if settings.Get("style") == "soft" {
		colors["rounding"] = "14"
	}
}

func chooseFont(colors theme.Colors) {
	colors["font"] = settings.Get("font")
	if !theme.ValidFont(colors["font"]) {
		colors["font"] = theme.DefaultFont
	}
}

func themeColors(dir string) (theme.Colors, error) {
	data, err := os.ReadFile(filepath.Join(dir, "colors.toml"))
	if err != nil {
		return nil, err
	}
	colors := theme.Parse(string(data))
	if colors["mode"]+colors["theme_type"] == "" && fsutil.IsFile(filepath.Join(dir, "light.mode")) {
		colors["mode"] = "light"
	}
	colors.Resolve()
	return colors, nil
}

func readColors(next string) (theme.Colors, error) {
	colors, err := themeColors(next)
	if err != nil {
		return nil, err
	}
	overrideAccent(colors)
	if err := atomic.Install(filepath.Join(next, "colors.toml"), []byte(colors.Canonical()), 0o644); err != nil {
		return nil, err
	}
	chooseFont(colors)
	glassSettings(colors)
	styleSettings(colors)
	return colors, nil
}

func templates(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var paths []string
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if !strings.HasPrefix(entry.Name(), ".") && strings.HasSuffix(entry.Name(), ".tpl") && fsutil.IsFile(path) {
			paths = append(paths, path)
		}
	}
	return paths
}

func renderOne(template, target string, replacer *strings.Replacer, scale float64) {
	data, err := os.ReadFile(template)
	if err == nil {
		err = atomic.Write(target, []byte(theme.Sized(replacer.Replace(string(data)), scale)), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func render(dir string, colors, apps theme.Colors) {
	replacers := map[bool]*strings.Replacer{false: colors.Replacer(), true: apps.Replacer()}
	scale := theme.TextScale(settings.Get("text"))
	found := append(templates(filepath.Join(env.AppConfig(), "themed")), templates(filepath.Join(systemRoot, "themed"))...)
	for _, template := range found {
		base := strings.TrimSuffix(filepath.Base(template), ".tpl")
		if target := filepath.Join(dir, base); !fsutil.Exists(target) {
			renderOne(template, target, replacers[slices.Contains(toolkitFiles, base)], scale)
		}
	}
}

func swap(next, target string) error {
	old := target + ".old"
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if err := os.Rename(target, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(next, target); err != nil {
		os.Rename(old, target)
		return err
	}
	return os.RemoveAll(old)
}

func symlink(target, link string) {
	err := os.MkdirAll(filepath.Dir(link), 0o777)
	if err == nil {
		err = atomic.Symlink(target, link)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func pickBackground(name, previous string) {
	link := filepath.Join(state(), "background")
	if previous == name && fsutil.Exists(link) {
		return
	}
	found := fsutil.Images(filepath.Join(state(), "theme", "backgrounds"), filepath.Join(env.AppConfig(), "backgrounds", name))
	if len(found) > 0 {
		symlink(found[0], link)
	}
}

func linkApps() {
	current := filepath.Join(state(), "theme")
	if file := filepath.Join(current, "walker.css"); fsutil.IsFile(file) {
		symlink(file, filepath.Join(env.ConfigHome(), "walker", "themes", "hyprtrixie", "style.css"))
	}
	if file := filepath.Join(current, "btop.theme"); fsutil.IsFile(file) {
		symlink(file, filepath.Join(env.ConfigHome(), "btop", "themes", "hyprtrixie.theme"))
	}
	if file := filepath.Join(current, "fastfetch.jsonc"); fsutil.IsFile(file) {
		linkUnlessYours(file, filepath.Join(env.ConfigHome(), "fastfetch", "config.jsonc"))
	}
	linkNeovim(current)
	followFirefox(current)
	linkToolkits(current)
}

func desktop(action string, args ...string) {
	args = append([]string{action, "org.gnome.desktop.interface"}, args...)
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		run.Within(run.Quick, "gsettings", args...)
		return
	}
	run.Quiet(run.Quick, "dbus-run-session", append([]string{"--", "gsettings"}, args...)...)
}

func desktopValue(key string) string {
	name, args := "gsettings", []string{"get", "org.gnome.desktop.interface", key}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		name, args = "dbus-run-session", append([]string{"--", "gsettings"}, args...)
	}
	output, _ := run.CaptureQuiet(run.Quick, name, args...)
	return strings.TrimSpace(output)
}

func ourScaling() bool {
	factor, err := strconv.ParseFloat(desktopValue("text-scaling-factor"), 64)
	return err == nil && slices.Contains([]float64{theme.TextScale("large"), theme.TextScale("larger")}, factor)
}

func iconTheme(mode string) string {
	icons, _, _ := strings.Cut(fsutil.Optional(filepath.Join(state(), "theme", "icons.theme")), "\n")
	if iconName.MatchString(icons) && fsutil.IsDir("/usr/share/icons/"+icons) {
		return icons
	}
	if mode == "dark" {
		return "Yaru-dark"
	}
	return "Yaru"
}

func desktopSettings(colors, apps theme.Colors) {
	if _, err := exec.LookPath("gsettings"); err != nil {
		return
	}
	mode := apps["mode"]
	if settings.Get("apps") == appsOff {
		releaseGTK(mode)
	} else {
		desktop("set", "color-scheme", "prefer-"+mode)
		desktop("set", "gtk-theme", gtkTheme(mode))
	}
	desktop("set", "monospace-font-name", colors["font"]+" 11")
	if ui := settings.Get("ui_font"); ui == "default" {
		desktop("reset", "font-name")
	} else {
		desktop("set", "font-name", ui+" 11")
	}
	if scale := theme.TextScale(settings.Get("text")); scale != 1 {
		desktop("set", "text-scaling-factor", strconv.FormatFloat(scale, 'f', -1, 64))
	} else if ourScaling() {
		desktop("reset", "text-scaling-factor")
	}
	desktop("set", "icon-theme", iconTheme(mode))
	desktop("set", "cursor-theme", "Bibata-Modern-Ice")
}

func rememberPick(name, mode string) {
	if os.Getenv("HYPRTRIXIE_LOOK_AUTO") != "" || settings.Get("auto") == "off" {
		return
	}
	key := "dark"
	if mode == "light" {
		key = "light"
	}
	if err := settings.Put(key, name); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func prepare(name, next string) (theme.Colors, theme.Colors, bool) {
	if err := stage(name, next); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, false
	}
	if !fsutil.IsFile(filepath.Join(next, "colors.toml")) {
		fmt.Fprintf(os.Stderr, "theme %s has no colors.toml\n", name)
		os.RemoveAll(next)
		return nil, nil, false
	}
	colors, err := readColors(next)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, false
	}
	apps := appColors(colors)
	render(next, colors, apps)
	if colors["glass"] != "yes" {
		os.Remove(filepath.Join(next, "glass.lua"))
	}
	return colors, apps, true
}

func apply(name string) bool {
	next := filepath.Join(filepath.Dir(state()), "next-theme")
	defer cleanup.OnSignal(func() { os.RemoveAll(next) })()
	previous, _ := fsutil.Text(filepath.Join(state(), "theme.name"))
	colors, apps, ok := prepare(name, next)
	if !ok {
		return false
	}
	if err := swap(next, filepath.Join(state(), "theme")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	if err := atomic.Write(filepath.Join(state(), "theme.name"), []byte(name+"\n"), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	pickBackground(name, previous)
	linkApps()
	desktopSettings(colors, apps)
	rememberPick(name, colors["mode"])
	return true
}

func detached(args ...string) {
	run.Quiet(run.Quick, "setsid", append([]string{"-f"}, args...)...)
}

func restartPaper() {
	if run.Within(run.Quick, "pkill", "-x", "hyprpaper") != 0 {
		return
	}
	run.Quiet(3*time.Second, "pidwait", "-x", "hyprpaper")
	detached("hyprtrixie-start", "hyprpaper")
	if self, err := os.Executable(); err == nil {
		detached(self, wallpapers)
	}
}

func retint() {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return
	}
	run.Quiet(run.Quick, "hyprctl", "reload")
	restartPaper()
	run.Quiet(run.Quick, "makoctl", "reload")
	tellKDEApps()
	look.ReloadShell()
	run.Within(run.Quick, "pkill", "-USR1", "-x", "kitty")
	run.Within(run.Quick, "pkill", "-USR2", "-x", "btop")
	reloadNeovim()
	if run.Within(run.Quick, "pkill", "-x", "walker") == 0 {
		detached("hyprtrixie-start", "walker")
	}
}

func lock() (*os.File, error) {
	if err := os.MkdirAll(state(), 0o777); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(filepath.Dir(state()), "theme.lock"), os.O_RDONLY|os.O_CREATE, 0o666)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			return file, err
		}
	}
}

func set(argument string) int {
	name := theme.Slug(argument)
	if !theme.ValidName(name) || (!fsutil.IsDir(filepath.Join(systemRoot, "themes", name)) && !fsutil.IsDir(filepath.Join(userThemes(), name))) {
		fmt.Fprintf(os.Stderr, "no theme named %s; see hyprtrixie-theme-set --list\n", argument)
		return 1
	}
	held, err := lock()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !apply(name) {
		return 1
	}
	held.Close()
	retint()
	return refreshEditors()
}

func list() {
	for _, name := range themeNames() {
		fmt.Println(theme.Pretty(name))
	}
}

func previews() {
	for _, name := range themeNames() {
		fmt.Printf("%s\t%s\t%s\n", name, theme.Pretty(name), previewOf(name))
	}
}

func current() int {
	file := filepath.Join(state(), "theme.name")
	if !fsutil.IsFile(file) {
		return 1
	}
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Print(theme.Pretty(string(data)))
	return 0
}

func main() {
	argument := env.Arg(1)
	switch argument {
	case "":
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	case "-h", "--help":
		fmt.Print(usage)
	case "--list":
		list()
	case "--previews":
		previews()
	case "--current":
		os.Exit(current())
	case "--editors":
		os.Exit(refreshEditors())
	case wallpapers:
		time.Sleep(time.Second)
		os.Exit(run.Report(run.Exec("hyprtrixie-background", "workspaces")))
	default:
		os.Exit(set(argument))
	}
}
