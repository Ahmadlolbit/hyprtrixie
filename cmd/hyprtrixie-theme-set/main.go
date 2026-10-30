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

Applies a theme to Hyprland, the bar, kitty, the launcher, notifications, the
lock screen, btop, GTK and the wallpaper. Themes come from
/usr/share/omarchy/themes and ~/.config/omarchy/themes.
`

const (
	denied     = " alacritty.toml foot.ini ghostty.conf kitty.conf vscode.json "
	wallpapers = "--wallpapers-soon"
)

var (
	systemRoot = env.Or("OMARCHY_PATH", "/usr/share/omarchy")
	iconName   = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	settings   = look.Open()
)

func state() string {
	return filepath.Join(env.StateHome(), "omarchy", "current")
}

func userThemes() string {
	return filepath.Join(env.ConfigHome(), "omarchy", "themes")
}

func themesIn(dir string) []string {
	var names []string
	for _, entry := range fsutil.Entries(dir) {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
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

func stageFromGit(own, next string) error {
	entries, err := os.ReadDir(own)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		base := entry.Name()
		switch {
		case strings.HasPrefix(base, "."):
		case entry.Type()&os.ModeSymlink != 0 || strings.HasSuffix(base, ".lua") || strings.Contains(denied, " "+base+" "):
			fmt.Fprintf(os.Stderr, "ignored %s from %s: a theme from a git repo cannot ship code or terminal configs\n", base, own)
		default:
			if err := fsutil.CopyEntry(filepath.Join(own, base), filepath.Join(next, base)); err != nil {
				return err
			}
		}
	}
	return nil
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
	if fsutil.IsLink(own) || !fsutil.IsDir(filepath.Join(own, ".git")) {
		return fsutil.CopyTree(own, next)
	}
	return stageFromGit(own, next)
}

func overrideAccent(colors theme.Colors, file string) {
	original := colors["accent"]
	accent, ok := colors.Accent(settings.Get("accent"))
	if !ok {
		return
	}
	colors["accent"] = accent
	data, err := os.ReadFile(file)
	if err == nil {
		err = atomic.Install(file, []byte(theme.WithAccent(string(data), accent, original)), 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func glassSettings(colors theme.Colors) {
	if settings.Get("glass") == "yes" {
		colors["glass"], colors["opacity"], colors["panel_alpha"] = "yes", "0.88", "cc"
		return
	}
	colors["glass"], colors["opacity"], colors["panel_alpha"] = "no", "1.0", "ee"
}

func chooseFont(colors theme.Colors) {
	colors["font"] = settings.Get("font")
	if !theme.ValidFont(colors["font"]) {
		colors["font"] = theme.DefaultFont
	}
}

func readColors(next string) (theme.Colors, error) {
	file := filepath.Join(next, "colors.toml")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	colors := theme.Parse(string(data))
	if colors["mode"]+colors["theme_type"] == "" && fsutil.IsFile(filepath.Join(next, "light.mode")) {
		colors["mode"] = "light"
	}
	colors.Resolve()
	overrideAccent(colors, file)
	chooseFont(colors)
	glassSettings(colors)
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

func renderOne(template, target string, replacer *strings.Replacer) {
	data, err := os.ReadFile(template)
	if err == nil {
		err = atomic.Write(target, []byte(replacer.Replace(string(data))), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func render(dir string, colors theme.Colors) {
	replacer := colors.Replacer()
	found := append(templates(filepath.Join(env.ConfigHome(), "omarchy", "themed")), templates(filepath.Join(systemRoot, "default", "themed"))...)
	for _, template := range found {
		target := filepath.Join(dir, strings.TrimSuffix(filepath.Base(template), ".tpl"))
		if !fsutil.Exists(target) {
			renderOne(template, target, replacer)
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
	found := fsutil.Images(filepath.Join(state(), "theme", "backgrounds"), filepath.Join(env.ConfigHome(), "omarchy", "backgrounds", name))
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
}

func desktop(action string, args ...string) {
	args = append([]string{action, "org.gnome.desktop.interface"}, args...)
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		run.Within(run.Quick, "gsettings", args...)
		return
	}
	run.Quiet(run.Quick, "dbus-run-session", append([]string{"--", "gsettings"}, args...)...)
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

func desktopSettings(colors theme.Colors) {
	if _, err := exec.LookPath("gsettings"); err != nil {
		return
	}
	mode := colors["mode"]
	desktop("set", "color-scheme", "prefer-"+mode)
	if mode == "light" {
		desktop("set", "gtk-theme", "Adwaita")
	} else {
		desktop("set", "gtk-theme", "Adwaita-dark")
	}
	desktop("set", "monospace-font-name", colors["font"]+" 11")
	if ui := settings.Get("ui_font"); ui == "default" {
		desktop("reset", "font-name")
	} else {
		desktop("set", "font-name", ui+" 11")
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

func prepare(name, next string) (theme.Colors, bool) {
	if err := stage(name, next); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, false
	}
	if !fsutil.IsFile(filepath.Join(next, "colors.toml")) {
		fmt.Fprintf(os.Stderr, "theme %s has no colors.toml\n", name)
		os.RemoveAll(next)
		return nil, false
	}
	colors, err := readColors(next)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, false
	}
	render(next, colors)
	if colors["glass"] != "yes" {
		os.Remove(filepath.Join(next, "glass.lua"))
	}
	return colors, true
}

func apply(name string) bool {
	next := filepath.Join(state(), "next-theme")
	defer cleanup.OnSignal(func() { os.RemoveAll(next) })()
	previous, _ := fsutil.Text(filepath.Join(state(), "theme.name"))
	colors, ok := prepare(name, next)
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
	desktopSettings(colors)
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
	look.ReloadShell()
	run.Within(run.Quick, "pkill", "-USR1", "-x", "kitty")
	run.Within(run.Quick, "pkill", "-USR2", "-x", "btop")
	if run.Within(run.Quick, "pkill", "-x", "walker") == 0 {
		detached("hyprtrixie-start", "walker")
	}
}

func lock() (*os.File, error) {
	if err := os.MkdirAll(state(), 0o777); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(state(), ".lock"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
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
	if name == "" || strings.HasPrefix(name, ".") || strings.Contains(name, "/") ||
		(!fsutil.IsDir(filepath.Join(systemRoot, "themes", name)) && !fsutil.IsDir(filepath.Join(userThemes(), name))) {
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
	return 0
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
	case wallpapers:
		time.Sleep(time.Second)
		os.Exit(run.Report(run.Exec("hyprtrixie-background", "workspaces")))
	default:
		os.Exit(set(argument))
	}
}
