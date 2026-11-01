package setup

import (
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fetch"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/options"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
)

const (
	oldName        = "omarchy"
	oldState       = ".local/state/" + oldName + "/"
	oldConfig      = ".config/" + oldName + "/"
	themeState     = ".local/state/hyprtrixie/current"
	userThemes     = ".config/hyprtrixie/themes/"
	userBackdrops  = ".config/hyprtrixie/backgrounds/"
	userLook       = ".config/hyprtrixie/look.conf"
	themesSource   = "hyprtrixie-themes"
	themeSetter    = "hyprtrixie-theme-set"
	followTool     = "hyprtrixie-follow-theme"
	followPath     = followTool + ".path"
	followService  = followTool + ".service"
	consoleService = "hyprtrixie-console.service"
	consoleSetup   = "/etc/default/console-setup"
	unitFolder     = "/etc/systemd/system/"
	splashScript   = "/usr/share/plymouth/themes/hyprtrixie/"
	grubFont       = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
	btopTheme      = `color_theme = "hyprtrixie"`
	btopSetting    = "color_theme = "
)

var (
	themeFiles  = []string{"colors.toml", "icons.theme", "preview.png"}
	themeTools  = []string{"hyprtrixie-look", "hyprtrixie-theme-install", "hyprtrixie-weather", "hyprtrixie-background", themeSetter}
	lookOptions = [][2]string{
		{"ACCENT", "ACCENT_COLOR"}, {"GLASS", "GLASS"}, {"CORNERS", "ROUNDED_CORNERS"}, {"AUTO", "AUTO_THEME"}, {"LIGHT", "LIGHT_THEME"},
		{"DARK", "DARK_THEME"}, {"LIGHT_AT", "LIGHT_AT"}, {"DARK_AT", "DARK_AT"}, {"ROTATE", "WALLPAPER_MINUTES"}, {"WORKSPACES", "WALLPAPER_PER_WORKSPACE"},
		{"WEATHER", "WEATHER"}, {"WEATHER_UNITS", "WEATHER_UNITS"}, {"QUIET", "QUIET_HOURS"}, {"QUIET_FROM", "QUIET_FROM"}, {"QUIET_TO", "QUIET_TO"},
	}
)

type themeRun struct {
	*Context
	lookChanged bool
	bootChanged bool
	grubUpdate  followUp
}

func oldThemeRoot() string {
	return env.Or("HYPRTRIXIE_OLD_THEME_ROOT", "/usr/share/"+oldName)
}

func lookConf() string {
	return env.Or("HYPRTRIXIE_LOOK_CONF", "/etc/hyprtrixie/look.conf")
}

func themeApplied() string {
	return filepath.Join(env.StateDir(), "theme")
}

func grubThemeDir() string {
	return env.Or("HYPRTRIXIE_GRUB_THEME_DIR", "/boot/grub/themes/hyprtrixie")
}

func grubSettings() string {
	return env.Or("HYPRTRIXIE_GRUB_SETTINGS", "/etc/default/grub.d/hyprtrixie.cfg")
}

func followFile(name string) string {
	return filepath.Join(env.ShareDir(), "follow", name)
}

func followProgram() string {
	return filepath.Join(env.BinDir(), followTool)
}

func installPlain(log *runlog.Log, source, target string) error {
	return run.Logged(log, "install", "-m", "0644", source, target)
}

func themeCopyFiles(log *runlog.Log, source, part string) error {
	for _, name := range themeFiles {
		if fsutil.IsFile(filepath.Join(source, name)) {
			if err := installPlain(log, filepath.Join(source, name), filepath.Join(part, name)); err != nil {
				return err
			}
		}
	}
	backgrounds, _ := filepath.Glob(filepath.Join(source, "backgrounds", "*"))
	for _, file := range backgrounds {
		if fsutil.IsFile(file) && !fsutil.IsLink(file) {
			if err := installPlain(log, file, filepath.Join(part, "backgrounds", filepath.Base(file))); err != nil {
				return err
			}
		}
	}
	return nil
}

func themeCopy(log *runlog.Log, source, target string) error {
	if !fsutil.IsFile(filepath.Join(source, "colors.toml")) {
		return nil
	}
	part := target + ".part"
	if err := os.RemoveAll(part); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(part, "backgrounds"), programMode); err != nil {
		return err
	}
	if err := themeCopyFiles(log, source, part); err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(part, target)
}

func (theme *themeRun) themesFetch(log *runlog.Log, fetched string) error {
	folders, _ := filepath.Glob(filepath.Join(fetched, "themes", "*"))
	for _, folder := range folders {
		if fsutil.IsDir(folder) {
			if err := themeCopy(log, folder, filepath.Join(env.ThemeRoot(), "themes", filepath.Base(folder))); err != nil {
				return err
			}
		}
	}
	if err := run.Logged(log, "install", "-D", "-m", "0644", filepath.Join(fetched, "LICENSE"), filepath.Join(env.ThemeRoot(), "LICENSE.themes")); err != nil {
		return err
	}
	return run.Logged(log, "install", "-D", "-m", "0644", filepath.Join(fetched, "THIRD-PARTY-LICENSES"), filepath.Join(env.ThemeRoot(), "THIRD-PARTY-LICENSES"))
}

func (theme *themeRun) themesInstall(log *runlog.Log) error {
	if fetch.SourceCurrent(log, themesSource) && fsutil.IsFile(filepath.Join(env.ThemeRoot(), "THIRD-PARTY-LICENSES")) {
		return nil
	}
	fetched, err := fetch.GitCached(log, themesSource, "/themes/", "/LICENSE", "/THIRD-PARTY-LICENSES")
	if err != nil {
		return err
	}
	if err := theme.themesFetch(log, fetched); err != nil {
		return err
	}
	if err := os.RemoveAll(fetched); err != nil {
		return err
	}
	theme.lookChanged = true
	return fetch.SourceMark(log, themesSource)
}

func (theme *themeRun) folderMove(log *runlog.Log, from, to string) error {
	old, fresh := theme.Person.Path(from), theme.Person.Path(to)
	if !fsutil.Exists(old) && !fsutil.IsLink(old) {
		return nil
	}
	if err := theme.userFolder(log, filepath.Dir(to)); err != nil {
		return err
	}
	if !fsutil.Exists(fresh) {
		return theme.Person.Run(log, "mv", old, fresh)
	}
	if err := theme.Person.Run(log, "cp", "-aln", old+"/.", fresh+"/"); err != nil {
		return err
	}
	return theme.Person.Run(log, "rm", "-rf", old)
}

func replacedPrefix(text, prefix, replacement string) string {
	if rest, found := strings.CutPrefix(text, prefix); found {
		return replacement + rest
	}
	return text
}

func (theme *themeRun) wallpaperFollow(log *runlog.Log) error {
	link := theme.Person.Path(themeState + "/background")
	target, err := os.Readlink(link)
	if err != nil {
		return nil
	}
	home := theme.Person.Home + "/"
	target = replacedPrefix(target, home+oldConfig, home+".config/hyprtrixie/")
	target = replacedPrefix(target, home+oldState, home+".local/state/hyprtrixie/")
	backdrops := home + userBackdrops
	for old, fresh := range config.RenamedThemes() {
		target = replacedPrefix(target, backdrops+old+"/", backdrops+fresh+"/")
	}
	return theme.Person.Run(log, "ln", "-sfn", target, link)
}

func oldFolders() [][2]string {
	moves := [][2]string{
		{oldConfig + "themes", ".config/hyprtrixie/themes"},
		{oldConfig + "backgrounds", ".config/hyprtrixie/backgrounds"},
		{oldConfig + "themed", ".config/hyprtrixie/themed"},
		{oldState + "current", themeState},
	}
	renames := config.RenamedThemes()
	for _, old := range slices.Sorted(maps.Keys(renames)) {
		moves = append(moves, [2]string{userThemes + old, userThemes + renames[old]}, [2]string{userBackdrops + old, userBackdrops + renames[old]})
	}
	return moves
}

func (theme *themeRun) oldFoldersMove(log *runlog.Log) error {
	for _, move := range oldFolders() {
		if err := theme.folderMove(log, move[0], move[1]); err != nil {
			return err
		}
	}
	if err := theme.wallpaperFollow(log); err != nil {
		return err
	}
	theme.Person.Run(io.Discard, "rmdir", theme.Person.Path(oldConfig), theme.Person.Path(oldState))
	return nil
}

func (theme *themeRun) oldIncludesUpdate(log *runlog.Log) error {
	for _, rel := range []string{".config/kitty/kitty.conf", ".config/mako/config", ".config/hypr/hyprlock.conf", ".config/hypr/hyprpaper.conf", userLook} {
		file := theme.Person.Path(rel)
		if !mentions(file, oldState) && !mentions(file, oldConfig) {
			continue
		}
		if err := theme.Person.Run(log, "sed", "-i", "--follow-symlinks", "-E", `s#\.(local/state|config)/`+oldName+`/#.\1/hyprtrixie/#g`, file); err != nil {
			return err
		}
	}
	for _, file := range []string{"/etc/xdg/hypr/hyprlock.conf", "/etc/xdg/hypr/hyprpaper.conf"} {
		if !mentions(file, oldState) {
			continue
		}
		if err := run.Logged(log, "sed", "-i", `s|\.local/state/`+oldName+`/|.local/state/hyprtrixie/|g`, file); err != nil {
			return err
		}
	}
	return nil
}

func (theme *themeRun) nameReplace(log *runlog.Log, file string, asPerson bool) error {
	lines, err := fsutil.Lines(file)
	if err != nil {
		return err
	}
	renames := config.RenamedThemes()
	for _, old := range slices.Sorted(maps.Keys(renames)) {
		if !slices.Contains(lines, old) && !slices.Contains(lines, "light="+old) && !slices.Contains(lines, "dark="+old) {
			continue
		}
		args := []string{"-i", "--follow-symlinks", "-E", `s/^((light|dark)=)?` + old + `$/\1` + renames[old] + `/`, file}
		if asPerson {
			err = theme.Person.Run(log, append([]string{"sed"}, args...)...)
		} else {
			err = run.Logged(log, "sed", args...)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (theme *themeRun) renamedFollow(log *runlog.Log) error {
	if err := theme.nameReplace(log, themeApplied(), false); err != nil {
		return err
	}
	if err := theme.nameReplace(log, lookConf(), false); err != nil {
		return err
	}
	if err := theme.nameReplace(log, theme.Person.Path(themeState+"/theme.name"), true); err != nil {
		return err
	}
	return theme.nameReplace(log, theme.Person.Path(userLook), true)
}

func (theme *themeRun) migrate(log *runlog.Log) error {
	if !fsutil.IsDir(oldThemeRoot()) {
		return nil
	}
	theme.lookChanged = true
	if err := theme.oldFoldersMove(log); err != nil {
		return err
	}
	if err := theme.renamedFollow(log); err != nil {
		return err
	}
	return theme.oldIncludesUpdate(log)
}

func missingThemes() []string {
	return slices.DeleteFunc(options.Themes(), func(name string) bool {
		return fsutil.IsFile(filepath.Join(env.ThemeRoot(), "themes", name, "colors.toml"))
	})
}

func (theme *themeRun) moveFinished() bool {
	return !fsutil.IsDir(oldThemeRoot()) || (!fsutil.IsDir(theme.Person.Path(oldState+"current")) && len(missingThemes()) == 0)
}

func (theme *themeRun) oldThemesRemove(log *runlog.Log) error {
	if !fetch.SourceCurrent(log, themesSource) {
		return nil
	}
	if err := os.RemoveAll(oldThemeRoot()); err != nil {
		return err
	}
	if err := removeFiles(filepath.Join(env.StateDir(), "sources", oldName), filepath.Join(env.StateDir(), "sources", "community-themes")); err != nil {
		return err
	}
	return theme.Deploy.Forget(oldThemeRoot()+"/", filepath.Join(env.BinDir(), oldName+"-theme-"))
}

func (theme *themeRun) switcherInstall(*runlog.Log) error {
	if err := theme.Deploy.Tree("themed", filepath.Join(env.ThemeRoot(), "themed")); err != nil {
		return err
	}
	for _, tool := range themeTools {
		if err := theme.Deploy.Tool(tool); err != nil {
			return err
		}
	}
	return removeFiles(filepath.Join(env.BinDir(), "hyprtrixie-theme-picker"), filepath.Join(env.BinDir(), oldName+"-theme-set"), filepath.Join(env.BinDir(), oldName+"-theme-install"))
}

func (theme *themeRun) btopFirstConfig(log *runlog.Log, conf string) error {
	if err := theme.userFolder(log, ".config/btop"); err != nil {
		return err
	}
	uid, gid, err := theme.Person.IDs()
	if err != nil {
		return err
	}
	return atomic.InstallAs(conf, []byte(btopTheme+"\n"), plainMode, uid, gid)
}

func (theme *themeRun) btopSetup(log *runlog.Log) error {
	conf := theme.Person.Path(".config/btop/btop.conf")
	if !fsutil.IsFile(conf) {
		return theme.btopFirstConfig(log, conf)
	}
	lines, err := fsutil.Lines(conf)
	if err != nil {
		return err
	}
	found := false
	for index, line := range lines {
		if strings.HasPrefix(line, btopSetting) {
			lines[index] = btopTheme
			found = true
		}
	}
	if !found {
		lines = append(lines, btopTheme)
	}
	return atomic.Write(conf, []byte(strings.Join(lines, "\n")+"\n"), plainMode)
}

func (theme *themeRun) lookInstall(*runlog.Log) error {
	var pairs []string
	for _, option := range lookOptions {
		pairs = append(pairs, option[0]+"="+theme.Config.Get(option[1]))
	}
	changed, err := theme.Deploy.Changed("system/look.conf", lookConf(), pairs...)
	if err != nil || !changed {
		return err
	}
	theme.lookChanged = true
	return removeFiles(theme.Person.Path(".local/state/hyprtrixie/look/period"))
}

func (theme *themeRun) nameInUse() string {
	name, _ := fsutil.Text(theme.Person.Path(themeState + "/theme.name"))
	return name
}

func (theme *themeRun) themeInUse() bool {
	name := theme.nameInUse()
	return name != "" && (fsutil.IsDir(filepath.Join(env.ThemeRoot(), "themes", name)) || fsutil.IsDir(theme.Person.Path(userThemes+name)))
}

func (theme *themeRun) themeSet(log *runlog.Log, name string) error {
	return theme.Person.Run(log, "env", "HYPRTRIXIE_THEME_ROOT="+env.ThemeRoot(), "HYPRTRIXIE_LOOK_SYSTEM="+lookConf(), "HYPRTRIXIE_LOOK_AUTO=1",
		filepath.Join(env.BinDir(), themeSetter), name)
}

func (theme *themeRun) applyChosen(log *runlog.Log, chosen string) error {
	applied, _ := fsutil.Text(themeApplied())
	if theme.themeInUse() && applied == chosen {
		log.Line("info", "keeping the theme in use; "+chosen+" was applied before")
		if theme.lookChanged {
			return theme.themeSet(log, theme.nameInUse())
		}
		return nil
	}
	if err := theme.themeSet(log, chosen); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(themeApplied()), programMode); err != nil {
		return err
	}
	return atomic.Write(themeApplied(), []byte(chosen+"\n"), plainMode)
}

func (theme *themeRun) apply(log *runlog.Log) error {
	if !theme.moveFinished() {
		return errors.New("the old themes haven't finished moving")
	}
	if err := theme.applyChosen(log, theme.Config.Get("THEME")); err != nil {
		return err
	}
	mako := theme.Person.Path(".config/mako/config")
	if !fsutil.IsFile(mako) {
		return nil
	}
	return files.EnsureLine(mako, "include=~/"+themeState+"/theme/mako.ini")
}

func (theme *themeRun) followInstall(*runlog.Log) error {
	if err := theme.Deploy.Tool(followTool); err != nil {
		return err
	}
	if err := theme.Deploy.File("system/plymouth/hyprtrixie.script", followFile("hyprtrixie.script")); err != nil {
		return err
	}
	if err := theme.Deploy.File("system/grub/theme.txt", followFile("theme.txt")); err != nil {
		return err
	}
	settings := theme.Config
	return theme.Deploy.File("system/follow-theme.conf", "/etc/hyprtrixie/follow-theme", "USER="+theme.Person.Name, "SPLASH="+settings.Get("BOOT_SPLASH"),
		"MENU="+settings.Get("BOOT_MENU_THEME"), "CONSOLE="+settings.Get("CONSOLE_THEME"), "CLOCK="+settings.Get("CLOCK_FORMAT"))
}

func (theme *themeRun) splashFile(source, target string) error {
	changed, err := theme.Deploy.Changed("system/"+source, target)
	theme.bootChanged = theme.bootChanged || changed
	return err
}

func (theme *themeRun) bootSplashInstall(log *runlog.Log) error {
	if theme.Config.Get("BOOT_SPLASH") != "yes" {
		return nil
	}
	if err := apt.InstallLists(log, "boot-splash"); err != nil {
		return err
	}
	if err := theme.splashFile("plymouth/hyprtrixie.plymouth", splashScript+"hyprtrixie.plymouth"); err != nil {
		return err
	}
	if err := theme.splashFile("plymouth/plymouthd.conf", "/etc/plymouth/plymouthd.conf"); err != nil {
		return err
	}
	return theme.splashFile("initramfs/splash.conf", "/etc/initramfs-tools/conf.d/hyprtrixie-splash")
}

func (theme *themeRun) grubThemeInstall(log *runlog.Log) error {
	if theme.Config.Get("BOOT_MENU_THEME") != "yes" {
		return nil
	}
	if err := apt.Install(log, "fonts-dejavu-core"); err != nil {
		return err
	}
	if err := os.MkdirAll(grubThemeDir(), programMode); err != nil {
		return err
	}
	for _, size := range []string{"16", "20", "28"} {
		font := filepath.Join(grubThemeDir(), "dejavu-"+size+".pf2")
		if fsutil.IsFile(font) {
			continue
		}
		if err := run.Logged(log, "grub-mkfont", "-s", size, "-o", font, grubFont); err != nil {
			return err
		}
	}
	return nil
}

func (theme *themeRun) grubConfig() ([]byte, error) {
	var parts []string
	if theme.Config.Get("BOOT_SPLASH") == "yes" {
		parts = append(parts, "splash.cfg")
	}
	if theme.Config.Get("BOOT_MENU_THEME") == "yes" {
		parts = append(parts, "theme.cfg")
	}
	var text []byte
	for _, part := range append(parts, "hidden.cfg") {
		data, err := os.ReadFile(filepath.Join(env.Root(), "system/grub", part))
		if err != nil {
			return nil, err
		}
		text = append(text, data...)
	}
	return text, nil
}

func (theme *themeRun) grubSetup(log *runlog.Log) error {
	text, err := theme.grubConfig()
	if err != nil {
		return err
	}
	changed, err := theme.Deploy.Content(text, grubSettings())
	if err != nil {
		return err
	}
	theme.grubUpdate.need(changed)
	return theme.grubUpdate.do(func() error {
		if apt.Commands("update-grub") != nil {
			return nil
		}
		return run.Logged(log, "update-grub")
	})
}

func (theme *themeRun) consoleInstall(log *runlog.Log) error {
	if theme.Config.Get("CONSOLE_THEME") != "yes" {
		return service.Disable(log, consoleService)
	}
	if err := apt.Install(log, "kbd", "console-setup"); err != nil {
		return err
	}
	if err := files.ConfSet(consoleSetup, "FONTFACE", `"Terminus"`); err != nil {
		return err
	}
	if err := files.ConfSet(consoleSetup, "FONTSIZE", `"16x32"`); err != nil {
		return err
	}
	if err := theme.Deploy.File("system/systemd/"+consoleService, unitFolder+consoleService); err != nil {
		return err
	}
	if err := service.Reload(log); err != nil {
		return err
	}
	return service.Enable(log, consoleService)
}

func (theme *themeRun) followRun(log *runlog.Log) error {
	if theme.bootChanged {
		return run.Logged(log, followProgram(), "--initramfs")
	}
	return run.Logged(log, followProgram())
}

func (theme *themeRun) followWatch(log *runlog.Log) error {
	if err := theme.Deploy.File("system/systemd/"+followService, unitFolder+followService); err != nil {
		return err
	}
	if err := theme.Deploy.File("system/systemd/"+followPath, unitFolder+followPath, "HOME="+theme.Person.Home); err != nil {
		return err
	}
	if err := service.Reload(log); err != nil {
		return err
	}
	if err := service.Enable(log, followPath); err != nil {
		return err
	}
	return service.Restart(log, followPath)
}

func (theme *themeRun) ready(*runlog.Log) error {
	if err := holds(theme.themeInUse()); err != nil {
		return err
	}
	return fileThere(theme.Person.Path(themeState + "/theme/colors.toml"))
}

func themeSteps(context *Context) {
	theme := &themeRun{Context: context}
	context.Run.Step("Install theme packages", packages("themes"))
	context.Run.Step("Move your themes to their new folders", theme.migrate)
	context.Run.Step("Install the themes", theme.themesInstall)
	context.Run.Step("Install the theme switcher", theme.switcherInstall)
	context.Run.Step("Point btop at the theme", theme.btopSetup)
	context.Run.Step("Write the look settings", theme.lookInstall)
	if context.Run.Step("Apply the "+context.Config.Get("THEME")+" theme", theme.apply) {
		context.Run.Step("Remove the old themes", theme.oldThemesRemove)
	}
	context.Run.Step("Install the theme follower", theme.followInstall)
	context.Run.Step("Show a boot splash in the theme's colours", theme.bootSplashInstall)
	context.Run.Step("Theme the boot menu", theme.grubThemeInstall)
	context.Run.Step("Set up the boot menu", theme.grubSetup)
	context.Run.Step("Theme the text consoles", theme.consoleInstall)
	context.Run.Step("Paint the boot and login screens in the theme's colours", theme.followRun)
	context.Run.Step("Follow theme switches on the boot and login screens", theme.followWatch)
}

func themeChecks(context *Context) {
	theme := &themeRun{Context: context}
	if context.Config.Get("BOOT_MENU_THEME") == "yes" {
		context.Run.Check("The boot menu is in the theme's colours", func(*runlog.Log) error { return fileThere(filepath.Join(grubThemeDir(), "theme.txt")) })
	}
	context.Run.Check("Theme switches reach the boot screens", func(*runlog.Log) error {
		return expect(service.Enabled(followPath), followPath+" isn't enabled")
	})
	context.Run.Check("The look settings are written", func(*runlog.Log) error { return fileThere(lookConf()) })
	if context.Config.Get("BOOT_SPLASH") == "yes" {
		context.Run.Check("The boot splash is in the theme's colours", func(*runlog.Log) error { return fileThere(splashScript + "hyprtrixie.script") })
	}
	context.Run.Check("A theme is applied", theme.ready)
	context.Run.Check("All themes are installed", func(*runlog.Log) error {
		missing := missingThemes()
		return expect(len(missing) == 0, "these themes are missing: "+strings.Join(missing, " "))
	})
}
