package setup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fetch"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ledger"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	nerdFont    = "jetbrainsmono-nerd"
	loginZsh    = "/usr/bin/zsh"
	fontPrefix  = "JetBrainsMonoNerdFont"
	shellStamp  = "shell"
	tmuxPlugins = ".config/tmux/plugins"
)

func fontDir() string {
	return env.Or("HYPRTRIXIE_FONT_DIR", "/usr/local/share/fonts/jetbrains-mono-nerd")
}

func zshShare() string {
	return filepath.Join(env.ShareDir(), "zsh")
}

func fontInPlace() bool {
	found, _ := filepath.Glob(filepath.Join(fontDir(), "*.ttf"))
	return len(found) > 0
}

func fontsIn(dir string) []string {
	var found []string
	filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && strings.HasPrefix(entry.Name(), fontPrefix) && strings.HasSuffix(entry.Name(), ".ttf") {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func nerdFontInstall(log *runlog.Log) error {
	if fetch.ReleaseCurrent(log, nerdFont) && fontInPlace() {
		return nil
	}
	file, err := fetch.FetchRelease(log, nerdFont)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "hyprtrixie")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := run.Logged(log, "tar", "-xJf", file, "-C", dir); err != nil {
		return err
	}
	if err := os.MkdirAll(fontDir(), programMode); err != nil {
		return err
	}
	args := append([]string{"-m", "0644", "-t", fontDir()}, fontsIn(dir)...)
	if err := run.Logged(log, "install", args...); err != nil {
		return err
	}
	if err := run.Logged(log, "fc-cache", "-f", fontDir()); err != nil {
		return err
	}
	return fetch.ReleaseMark(log, nerdFont)
}

func loginShell(name string) string {
	person, _ := account.Lookup(name)
	return person.Shell
}

func (context *Context) zshrcSetup(log *runlog.Log) error {
	zshrc := context.Person.Path(".zshrc")
	if fsutil.Exists(zshrc) {
		return files.EnsureUserLine(log, context.Person, zshrc, "source "+filepath.Join(zshShare(), "hyprtrixie.zsh"))
	}
	return context.Deploy.UserOnce("config/zsh/zshrc", ".zshrc")
}

func (context *Context) zshLoaded() bool {
	return mentions(context.Person.Path(".zshrc"), filepath.Join(zshShare(), "hyprtrixie.zsh"))
}

func (context *Context) shellChosen() string {
	return context.Person.Path(filepath.Join(yourDefaults, shellStamp))
}

func (context *Context) zshSetup(log *runlog.Log) error {
	if err := fetch.Git(log, "oh-my-zsh", filepath.Join(env.ShareDir(), "oh-my-zsh")); err != nil {
		return err
	}
	if err := context.Deploy.File("config/zsh/hyprtrixie.zsh", filepath.Join(zshShare(), "hyprtrixie.zsh")); err != nil {
		return err
	}
	setBefore := context.zshLoaded()
	if err := context.zshrcSetup(log); err != nil {
		return err
	}
	return context.loginShellOnce(log, setBefore)
}

func (context *Context) loginShellOnce(log *runlog.Log, setBefore bool) error {
	if fsutil.IsFile(context.shellChosen()) {
		return nil
	}
	shell := loginShell(context.Person.Name)
	if !setBefore && shell != loginZsh {
		if err := run.Logged(log, "chsh", "-s", loginZsh, context.Person.Name); err != nil {
			return err
		}
		shell = loginZsh
	}
	if err := context.userFolder(log, yourDefaults); err != nil {
		return err
	}
	return context.Person.Write(log, context.shellChosen(), shell+"\n")
}

func (context *Context) zshReady(*runlog.Log) error {
	if shell := loginShell(context.Person.Name); shell != loginZsh && !fsutil.IsFile(context.shellChosen()) {
		return errors.New("your login shell is " + shell)
	}
	return expect(context.zshLoaded(), ".zshrc doesn't load hyprtrixie.zsh")
}

func (context *Context) promptSetup(log *runlog.Log) error {
	prompt := filepath.Join(zshShare(), "prompt.zsh")
	if context.Config.Get("SHELL_PROMPT") == "powerlevel10k" {
		if err := fetch.Git(log, "powerlevel10k", filepath.Join(env.ShareDir(), "powerlevel10k")); err != nil {
			return err
		}
		return context.Deploy.File("config/zsh/prompt-powerlevel10k.zsh", prompt)
	}
	if err := context.Deploy.UserFile("config/starship.toml", ".config/starship.toml"); err != nil {
		return err
	}
	return context.Deploy.File("config/zsh/prompt-starship.zsh", prompt)
}

func (context *Context) promptReady(*runlog.Log) error {
	prompt := filepath.Join(zshShare(), "prompt.zsh")
	if context.Config.Get("SHELL_PROMPT") == "powerlevel10k" {
		if err := fileThere(filepath.Join(env.ShareDir(), "powerlevel10k/powerlevel10k.zsh-theme")); err != nil {
			return err
		}
		return expect(mentions(prompt, "powerlevel10k"), prompt+" doesn't load powerlevel10k")
	}
	if err := apt.Commands("starship"); err != nil {
		return err
	}
	return expect(mentions(prompt, "starship"), prompt+" doesn't load starship")
}

func (context *Context) tmuxSetup(log *runlog.Log) error {
	if err := context.Deploy.UserFile("config/tmux/tmux.conf", ".config/tmux/tmux.conf"); err != nil {
		return err
	}
	if context.pluginsYours(log) {
		return nil
	}
	plugins, err := context.pluginsFolder(log)
	if err != nil {
		return err
	}
	for _, plugin := range []string{"tpm", "tmux-resurrect", "tmux-continuum"} {
		if err := fetch.GitCopy(log, plugin, filepath.Join(plugins, plugin), context.Person); err != nil {
			return err
		}
	}
	return nil
}

func (context *Context) leftToRoot(dir string) bool {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	return ok && owner.Uid == 0 && context.Person.UID != "0"
}

func (context *Context) pluginsYours(log *runlog.Log) bool {
	dir := context.Person.Path(tmuxPlugins)
	if !fsutil.IsLink(dir) && (ours(dir, "") || context.leftToRoot(dir)) {
		return false
	}
	log.Line("info", dir+" is a plugins folder of your own, so the plugins in it stay as they are")
	return true
}

func (context *Context) pluginsFolder(log *runlog.Log) (string, error) {
	dir := context.Person.Path(tmuxPlugins)
	if err := context.userFolder(log, tmuxPlugins); err != nil {
		return "", err
	}
	ledger.Note(ledger.Made, dir)
	if context.leftToRoot(dir) {
		return dir, run.Logged(log, "chown", "-h", context.Person.Name+":", dir)
	}
	return dir, nil
}

func (context *Context) tmuxReady(*runlog.Log) error {
	if err := fileThere(context.Person.Path(".config/tmux/tmux.conf")); err != nil {
		return err
	}
	plugins := context.Person.Path(".config/tmux/plugins/tpm/tpm")
	return expect(fsutil.Runnable(plugins), plugins+" won't run")
}

func terminalSteps(context *Context) {
	prompt := context.Config.Get("SHELL_PROMPT")
	context.Run.Step("Install terminal packages", packages("terminal"))
	context.Run.Step("Install JetBrains Mono Nerd Font", nerdFontInstall)
	context.Run.Step("Set up zsh with Oh My Zsh", context.zshSetup)
	context.Run.Step("Set up the "+prompt+" prompt", context.promptSetup)
	context.Run.Step("Set up tmux", context.tmuxSetup)
	context.Run.Step("Set up kitty", func(*runlog.Log) error {
		return context.Deploy.UserFile("config/kitty/kitty.conf", ".config/kitty/kitty.conf")
	})
}

func terminalChecks(context *Context) {
	context.Run.Check("JetBrains Mono Nerd Font is installed", func(*runlog.Log) error {
		return holds(run.Quiet(run.Minute, "fc-list", "-q", "JetBrainsMono Nerd Font") == 0)
	})
	context.Run.Check("The "+context.Config.Get("SHELL_PROMPT")+" prompt is set up", context.promptReady)
	context.Run.Check("kitty, zsh and tmux are installed", commands("kitty", "zsh", "tmux"))
	context.Run.Check("tmux is set up with its plugins", context.tmuxReady)
	context.Run.Check("zsh is set up", context.zshReady)
}
