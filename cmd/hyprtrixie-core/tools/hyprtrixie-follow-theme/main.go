package followtheme

import (
	"cmp"
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/bootimage"
	"github.com/Ahmadlolbit/hyprtrixie/internal/burst"
	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/follow"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `Usage: hyprtrixie-follow-theme [--initramfs]

Paints the boot splash, the boot menu, the text consoles, the login screen and
Chromium-based browsers in the colours and wallpaper of the theme the desktop
user has picked.
--initramfs rebuilds the initramfs even when the colours didn't change.
Run as root; the installer and a theme switch run it for you.
`

const (
	initramfsLimit = 30 * time.Minute
	rounds         = 3
	settle         = 2 * time.Second
	colorsLimit    = 64 << 10
)

var (
	settingsFile = env.Or("HYPRTRIXIE_FOLLOW_SETTINGS", "/etc/hyprtrixie/follow-theme")
	templates    = env.Or("HYPRTRIXIE_FOLLOW_TEMPLATES", "/usr/share/hyprtrixie/follow")
	plymouthDir  = env.Or("HYPRTRIXIE_PLYMOUTH_DIR", "/usr/share/plymouth/themes/hyprtrixie")
	grubDir      = env.Or("HYPRTRIXIE_GRUB_THEME_DIR", "/boot/grub/themes/hyprtrixie")
	vtrgb        = env.Or("HYPRTRIXIE_VTRGB", "/etc/hyprtrixie/vtrgb")
	sddmDir      = env.Or("HYPRTRIXIE_SDDM_THEME_DIR", "/usr/share/sddm/themes/hyprtrixie")
	owedImage    = filepath.Join(env.StateDir(), "owed", "boot-image")
)

type theme struct {
	settings   string
	colors     string
	seen       string
	user       string
	home       string
	background string
	foreground string
	accent     string
	muted      string
}

func homeOf(user string) string {
	output, err := run.Capture(run.Quick, "getent", "passwd", user)
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(output, "\n")
	fields := strings.Split(line, ":")
	if len(fields) < 6 {
		return ""
	}
	return fields[5]
}

func refresh(dest, content string) (bool, error) {
	if current, err := os.ReadFile(dest); err == nil && string(current) == content {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return true, err
	}
	return true, atomic.Install(dest, []byte(content), 0o644)
}

func render(template, dest string, pairs ...string) (bool, error) {
	data, err := os.ReadFile(template)
	if err != nil {
		return false, err
	}
	return refresh(dest, follow.Fill(string(data), pairs...))
}

func splash(current theme, initramfs bool) bool {
	files, err := splashFiles(current)
	if err == nil {
		err = owe(files)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	if !initramfs && !fsutil.Exists(owedImage) {
		return true
	}
	rebuild := bootimage.Rebuild(false)
	if run.Within(initramfsLimit, rebuild[0], rebuild[1:]...) != 0 {
		return false
	}
	if err := os.Remove(owedImage); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, err)
	}
	return true
}

func splashFiles(current theme) ([][2]string, error) {
	data, err := os.ReadFile(filepath.Join(templates, "hyprtrixie.script"))
	if err != nil {
		return nil, err
	}
	script := follow.Fill(string(data), "BACKGROUND="+follow.Floats(current.background), "FOREGROUND="+follow.Floats(current.foreground), "ACCENT="+follow.Floats(current.accent))
	files := [][2]string{{filepath.Join(plymouthDir, "hyprtrixie.script"), script}}
	channels := follow.Channels(current.accent)
	paint := color.NRGBA{R: uint8(channels[0]), G: uint8(channels[1]), B: uint8(channels[2]), A: 255}
	for _, scale := range []int{4, 8} {
		picture, err := burst.PNG(scale, paint)
		if err != nil {
			return nil, err
		}
		files = append(files, [2]string{filepath.Join(plymouthDir, fmt.Sprintf("burst-%d.png", scale)), string(picture)})
	}
	return files, nil
}

func owe(files [][2]string) error {
	stale := slices.ContainsFunc(files, func(file [2]string) bool {
		current, err := os.ReadFile(file[0])
		return err != nil || string(current) != file[1]
	})
	if !stale {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(owedImage), 0o755); err != nil {
		return err
	}
	if err := atomic.Install(owedImage, nil, 0o644); err != nil {
		return err
	}
	for _, file := range files {
		if _, err := refresh(file[0], file[1]); err != nil {
			return err
		}
	}
	return nil
}

func menu(current theme) bool {
	_, err := render(filepath.Join(templates, "theme.txt"), filepath.Join(grubDir, "theme.txt"), "BACKGROUND="+current.background,
		"FOREGROUND="+current.foreground, "ACCENT="+current.accent, "MUTED="+current.muted)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return err == nil
}

func console(current theme) bool {
	changed, err := refresh(vtrgb, follow.Palette(current.colors, current.foreground))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	if changed {
		run.Quiet(run.Quick, "setvtrgb", vtrgb)
	}
	return true
}

func removeWallpapers() {
	entries, err := os.ReadDir(sddmDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "wallpaper.") {
			if err := os.Remove(filepath.Join(sddmDir, entry.Name())); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}

func asUser(limit time.Duration, user string, args ...string) int {
	return run.Within(limit, "runuser", append([]string{"-u", user, "--"}, args...)...)
}

func fetchWallpaper(temp, user, image string) bool {
	file, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	defer file.Close()
	if run.Into(run.Slow, file, "runuser", "-u", user, "--", "cat", image) != 0 {
		return false
	}
	if err := file.Sync(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	return true
}

func copyWallpaper(user, image, name string) bool {
	temp := filepath.Join(sddmDir, ".wallpaper-"+strconv.FormatUint(rand.Uint64(), 36))
	defer cleanup.OnSignal(func() { os.Remove(temp) })()
	defer os.Remove(temp)
	if !fetchWallpaper(temp, user, image) {
		return false
	}
	removeWallpapers()
	err := os.Chmod(temp, 0o644)
	if err == nil {
		err = os.Rename(temp, filepath.Join(sddmDir, name))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return err == nil
}

func wallpaper(current theme) (string, bool) {
	image, err := fsutil.RealPath(filepath.Join(current.home, ".local/state/hyprtrixie/current/background"))
	if err != nil {
		return "", true
	}
	extension, usable := follow.Extension(image)
	if !fsutil.IsFile(image) || !usable {
		return "", true
	}
	if asUser(run.Quick, current.user, "test", "-r", image) != 0 {
		return "", true
	}
	name := "wallpaper." + extension
	if asUser(run.Quick, current.user, "cmp", "-s", image, filepath.Join(sddmDir, name)) == 0 {
		return name, true
	}
	return name, copyWallpaper(current.user, image, name)
}

func loginScreen(current theme) bool {
	name, ok := wallpaper(current)
	if !ok {
		return false
	}
	text := fmt.Sprintf("[General]\nbackground=%s\nforeground=%s\naccent=%s\nfail=%s\nclock=%s\nwallpaper=%s\n", current.background,
		current.foreground, current.accent, cmp.Or(follow.Color(current.colors, "red"), "#f38ba8"), follow.Setting(current.settings, "clock"), name)
	if _, err := refresh(filepath.Join(sddmDir, "theme.conf.user"), text); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	return true
}

func load() (theme, bool) {
	data, _ := os.ReadFile(settingsFile)
	settings := string(data)
	user := follow.Setting(settings, "user")
	home := homeOf(user)
	file := home + "/.local/state/hyprtrixie/current/theme/colors.toml"
	colors, err := fsutil.ReadPlain(file, colorsLimit)
	if user == "" || err != nil {
		if user != "" && !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("No theme to follow yet (%s)\n", file)
		return theme{}, false
	}
	current := theme{settings: settings, colors: string(colors), user: user, home: home}
	current.background = cmp.Or(follow.Color(current.colors, "background"), "#1e1e2e")
	current.foreground = cmp.Or(follow.Color(current.colors, "foreground"), "#cdd6f4")
	current.accent = cmp.Or(follow.Color(current.colors, "accent"), "#89b4fa")
	current.muted = follow.Blend(current.foreground, current.background)
	link, _ := os.Readlink(filepath.Join(home, ".local/state/hyprtrixie/current/background"))
	current.seen = current.colors + "\x00" + link
	return current, true
}

func paint(initramfs bool) int {
	current, ok := load()
	code := 0
	for round := 0; ok && round < rounds; round++ {
		code = paintOnce(current, initramfs && round == 0)
		if now, found := load(); !found || now.seen == current.seen {
			break
		}
		time.Sleep(settle)
		current, ok = load()
	}
	return code
}

func paintOnce(current theme, initramfs bool) int {
	painted := true
	if follow.Setting(current.settings, "menu") == "yes" && fsutil.IsDir(grubDir) {
		painted = menu(current)
	}
	if follow.Setting(current.settings, "console") == "yes" {
		painted = console(current) && painted
	}
	if fsutil.IsDir(sddmDir) {
		painted = loginScreen(current) && painted
	}
	painted = paintBrowsers(current) && painted
	if follow.Setting(current.settings, "splash") == "yes" {
		painted = splash(current, initramfs) && painted
	}
	if !painted {
		return 1
	}
	return 0
}

func Main() {
	switch env.Arg(1) {
	case "":
		os.Exit(paint(false))
	case "--initramfs":
		os.Exit(paint(true))
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
