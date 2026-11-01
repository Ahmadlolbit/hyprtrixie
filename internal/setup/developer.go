package setup

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apps"
	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fetch"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

const (
	dockerUnit   = "docker.service"
	engineSwitch = "engine-switch"
	neovimName   = "NVIM v"
	toolboxDir   = "/opt/jetbrains-toolbox"
	toolboxTool  = "jetbrains-toolbox"
	zedDesktop   = "share/applications/dev.zed.Zed.desktop"
)

type toolStep struct {
	tools []string
	title string
	work  step.Work
}

func (context *Context) userFolder(log *runlog.Log, rel string) error {
	return context.Person.Run(log, "mkdir", "-p", context.Person.Path(rel))
}

func (context *Context) answers(args ...string) bool {
	return run.Quiet(run.Minute, "runuser", context.Person.RunuserArgs(args...)...) == 0
}

type dockerSetup struct {
	*Context
	restart followUp
}

func (docker *dockerSetup) install(log *runlog.Log) error {
	if err := apt.ExtrepoTry(log, "docker-ce"); err != nil {
		return err
	}
	if err := apt.InstallLists(log, "dev-docker"); err != nil {
		return err
	}
	changed, err := docker.Deploy.Changed("system/docker/daemon-"+docker.Config.Get("DOCKER_PORTS")+".json", "/etc/docker/daemon.json")
	docker.restart.need(changed)
	if err != nil {
		return err
	}
	err = docker.restart.do(func() error {
		if !service.Active(dockerUnit) {
			return nil
		}
		return service.Restart(log, dockerUnit)
	})
	if err != nil {
		return err
	}
	return docker.Person.AddGroups(log, "docker")
}

func (context *Context) dockerReady(*runlog.Log) error {
	if err := apt.Commands("docker"); err != nil {
		return err
	}
	return expect(context.Person.InGroup("docker"), "you are not in the docker group")
}

func (context *Context) podmanInstall(log *runlog.Log) error {
	if err := apt.InstallLists(log, "dev-podman"); err != nil {
		return err
	}
	entry := context.Person.Name + ":"
	if fileLineHasPrefix("/etc/subuid", entry) && fileLineHasPrefix("/etc/subgid", entry) {
		return nil
	}
	return run.Logged(log, "usermod", "--add-subuids", "100000-165535", "--add-subgids", "100000-165535", context.Person.Name)
}

func (context *Context) engineWanted() string {
	if context.Config.Enabled("DEV_TOOLS", "podman") && (!context.Config.Enabled("DEV_TOOLS", "docker") || context.Config.Get("CONTAINER_ENGINE") == "podman") {
		return "podman"
	}
	return "docker"
}

func engineInUse() string {
	output, _ := run.CaptureQuiet(run.Minute, filepath.Join(env.BinDir(), engineSwitch), "status")
	for _, line := range run.Lines(output) {
		if fields := strings.Fields(line); len(fields) > 1 && strings.HasPrefix(line, "engine:") {
			return fields[1]
		}
	}
	return ""
}

func (context *Context) engineInstall(log *runlog.Log) error {
	if err := context.Deploy.Tool(engineSwitch); err != nil {
		return err
	}
	engine := context.engineWanted()
	if engineInUse() == engine && fsutil.Exists(filepath.Join(env.BinDir(), "docker")) {
		return nil
	}
	return run.Logged(log, "env", "SUDO_USER="+context.Person.Name, filepath.Join(env.BinDir(), engineSwitch), engine)
}

func (context *Context) engineReady(*runlog.Log) error {
	engine := engineInUse()
	return expect(engine == context.engineWanted(), "it points at "+cmp.Or(engine, "nothing"))
}

func (context *Context) miseInstall(log *runlog.Log) error {
	if err := apt.ExtrepoTry(log, "mise"); err != nil {
		return err
	}
	if err := apt.InstallLists(log, "dev-mise"); err != nil {
		return err
	}
	return context.Person.Run(log, "mise", "use", "--global", "node@lts")
}

func (context *Context) rustInstall(log *runlog.Log) error {
	if err := apt.InstallLists(log, "dev-rust"); err != nil {
		return err
	}
	return context.Person.Run(log, "rustup", "default", "stable")
}

func (context *Context) toolchainsReady(*runlog.Log) error {
	if context.Config.Enabled("DEV_TOOLS", "mise") && !context.answers("mise", "exec", "--", "node", "--version") {
		return errors.New("mise has no Node.js")
	}
	if context.Config.Enabled("DEV_TOOLS", "python") {
		if err := apt.Commands("pyenv"); err != nil {
			return err
		}
	}
	if context.Config.Enabled("DEV_TOOLS", "rust") && !context.answers("rustc", "--version") {
		return errors.New("rustc doesn't run")
	}
	if context.Config.Enabled("DEV_TOOLS", "go") {
		return apt.Commands("go")
	}
	return nil
}

func kubernetesInstall(log *runlog.Log) error {
	for _, repository := range []string{"kubernetes", "helm"} {
		if err := apt.ExtrepoTry(log, repository); err != nil {
			return err
		}
	}
	if err := apt.InstallLists(log, "dev-kubernetes"); err != nil {
		return err
	}
	if err := apt.InstallRelease(log, "k9s", "k9s"); err != nil {
		return err
	}
	kind := filepath.Join(env.BinDir(), "kind")
	if fetch.ReleaseCurrent(log, "kind") && fsutil.Runnable(kind) {
		return nil
	}
	file, err := fetch.FetchRelease(log, "kind")
	if err != nil {
		return err
	}
	if err := run.Logged(log, "install", "-D", "-m", "0755", file, kind); err != nil {
		return err
	}
	return fetch.ReleaseMark(log, "kind")
}

func neovimUnpack(log *runlog.Log, target string) error {
	dir, err := unpackedRelease(log, "neovim")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return run.Logged(log, "mv", filepath.Join(dir, "nvim-linux-x86_64"), target)
}

func (context *Context) neovimInstall(log *runlog.Log) error {
	release, err := fetch.GetRelease(log, "neovim")
	if err != nil {
		return err
	}
	dir := "/opt/nvim-" + strings.TrimPrefix(release.Version, "v")
	if !fsutil.Runnable(filepath.Join(dir, "bin/nvim")) {
		if err := neovimUnpack(log, dir); err != nil {
			return err
		}
	}
	if err := atomic.Symlink(filepath.Join(dir, "bin/nvim"), filepath.Join(env.BinDir(), "nvim")); err != nil {
		return err
	}
	if err := context.Deploy.File(filepath.Join(dir, "share/applications/nvim.desktop"), filepath.Join(apps.WebFolder, "nvim.desktop")); err != nil {
		return err
	}
	return apt.InstallLists(log, "dev-neovim")
}

func (context *Context) lazyvimSetup(log *runlog.Log) error {
	config := context.Person.Path(".config/nvim")
	if fsutil.Exists(config) {
		return nil
	}
	cached, err := fetch.GitCached(log, "lazyvim-starter")
	if err != nil {
		return err
	}
	if err := context.userFolder(log, ".config"); err != nil {
		return err
	}
	if err := run.Logged(log, "cp", "-a", cached, config); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(config, ".git")); err != nil {
		return err
	}
	return give(log, context.Person.Name, config)
}

func (context *Context) lazyvimPlugins(log *runlog.Log) error {
	return context.Person.Run(log, filepath.Join(env.BinDir(), "nvim"), "--headless", "+Lazy! sync", "+qa")
}

func recentVersion(version string) bool {
	if version != "" && version[0] >= '1' && version[0] <= '9' {
		return true
	}
	minor, found := strings.CutPrefix(version, "0.")
	if !found || len(minor) < 2 || minor[1] < '0' || minor[1] > '9' {
		return false
	}
	return minor[0] == '1' && minor[1] != '0' || minor[0] >= '2' && minor[0] <= '9'
}

func recentNeovim(line string) bool {
	_, version, found := strings.Cut(line, neovimName)
	for found {
		if recentVersion(version) {
			return true
		}
		_, version, found = strings.Cut(version, neovimName)
	}
	return false
}

func neovimReady(*runlog.Log) error {
	output, _ := run.CaptureQuiet(run.Minute, filepath.Join(env.BinDir(), "nvim"), "--version")
	first, _, _ := strings.Cut(output, "\n")
	return expect(recentNeovim(first), "found "+cmp.Or(first, "no Neovim"))
}

func vendorEditor(repository, list string) step.Work {
	return func(log *runlog.Log) error {
		if err := apt.ExtrepoTry(log, repository); err != nil {
			return err
		}
		return apt.InstallLists(log, list)
	}
}

func (context *Context) zedUnpack(log *runlog.Log, app string) error {
	dir, err := unpackedRelease(log, "zed")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.RemoveAll(app); err != nil {
		return err
	}
	if err := context.userFolder(log, ".local"); err != nil {
		return err
	}
	if err := run.Logged(log, "mv", filepath.Join(dir, "zed.app"), app); err != nil {
		return err
	}
	if err := give(log, context.Person.Name, app); err != nil {
		return err
	}
	return fetch.ReleaseMark(log, "zed")
}

func (context *Context) zedEntry(log *runlog.Log, app string) error {
	if err := context.userFolder(log, ".local/share/applications"); err != nil {
		return err
	}
	text, err := context.Person.Read(log, filepath.Join(app, zedDesktop))
	if err != nil {
		return err
	}
	text = strings.ReplaceAll(text, "Exec=zed", "Exec="+app+"/bin/zed")
	text = strings.ReplaceAll(text, "Icon=zed", "Icon="+app+"/share/icons/hicolor/512x512/apps/zed.png")
	return context.Person.Write(log, context.Person.Path(".local/share/applications/dev.zed.Zed.desktop"), text+"\n")
}

func (context *Context) zedInstall(log *runlog.Log) error {
	app := context.Person.Path(".local/zed.app")
	if !fetch.ReleaseCurrent(log, "zed") || !fsutil.Runnable(filepath.Join(app, "bin/zed")) {
		if err := context.zedUnpack(log, app); err != nil {
			return err
		}
	}
	if err := context.userFolder(log, ".local/bin"); err != nil {
		return err
	}
	if err := context.Person.Run(log, "ln", "-sfn", filepath.Join(app, "bin/zed"), context.Person.Path(".local/bin/zed")); err != nil {
		return err
	}
	return context.zedEntry(log, app)
}

func (context *Context) editorsReady(*runlog.Log) error {
	for _, editor := range [][2]string{{"vscode", "code"}, {"vscodium", "codium"}} {
		if context.Config.Enabled("DEV_TOOLS", editor[0]) {
			if err := apt.Commands(editor[1]); err != nil {
				return err
			}
		}
	}
	return expect(!context.Config.Enabled("DEV_TOOLS", "zed") || fsutil.Runnable(context.Person.Path(".local/zed.app/bin/zed")), "Zed is not installed")
}

func toolboxUnpack(log *runlog.Log) error {
	dir, err := unpackedRelease(log, "toolbox")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.RemoveAll(toolboxDir); err != nil {
		return err
	}
	found, _ := filepath.Glob(filepath.Join(dir, toolboxTool+"-*"))
	if err := run.Logged(log, "mv", append(found, toolboxDir)...); err != nil {
		return err
	}
	return fetch.ReleaseMark(log, "toolbox")
}

func (context *Context) toolboxInstall(log *runlog.Log) error {
	program := filepath.Join(toolboxDir, "bin", toolboxTool)
	if !fetch.ReleaseCurrent(log, "toolbox") || !fsutil.Runnable(program) {
		if err := toolboxUnpack(log); err != nil {
			return err
		}
	}
	if err := atomic.Symlink(program, filepath.Join(env.BinDir(), toolboxTool)); err != nil {
		return err
	}
	return context.Deploy.File("config/applications/jetbrains-toolbox.desktop", filepath.Join(apps.WebFolder, "jetbrains-toolbox.desktop"))
}

func (context *Context) toolSteps() []toolStep {
	docker := dockerSetup{Context: context, restart: followUp{name: "docker-restart"}}
	return []toolStep{
		{[]string{"docker"}, "Install Docker", docker.install},
		{[]string{"podman"}, "Install Podman", context.podmanInstall},
		{[]string{"docker", "podman"}, "Set up engine-switch", context.engineInstall},
		{[]string{"mise"}, "Install mise with Node.js LTS", context.miseInstall},
		{[]string{"python"}, "Install pyenv", packages("dev-python")},
		{[]string{"rust"}, "Install Rust (stable)", context.rustInstall},
		{[]string{"go"}, "Install Go", packages("dev-go")},
		{[]string{"kubernetes"}, "Install kubectl, helm, k9s and kind", kubernetesInstall},
		{[]string{"neovim"}, "Install Neovim", context.neovimInstall},
		{[]string{"neovim"}, "Set up LazyVim", context.lazyvimSetup},
		{[]string{"neovim"}, "Install LazyVim plugins", context.lazyvimPlugins},
		{[]string{"vscode"}, "Install Visual Studio Code", vendorEditor("vscode", "dev-vscode")},
		{[]string{"vscodium"}, "Install VSCodium", vendorEditor("vscodium", "dev-vscodium")},
		{[]string{"zed"}, "Install Zed", context.zedInstall},
		{[]string{"toolbox"}, "Install JetBrains Toolbox", context.toolboxInstall},
	}
}

func developerSteps(context *Context) {
	for _, entry := range context.toolSteps() {
		if slices.ContainsFunc(entry.tools, func(tool string) bool { return context.Config.Enabled("DEV_TOOLS", tool) }) {
			context.Run.Step(entry.title, entry.work)
		}
	}
}

func developerChecks(context *Context) {
	docker, podman := context.Config.Enabled("DEV_TOOLS", "docker"), context.Config.Enabled("DEV_TOOLS", "podman")
	if docker {
		context.Run.Check("Docker works and you are in the docker group", context.dockerReady)
	}
	context.Run.Check("Code editors are installed", context.editorsReady)
	if docker || podman {
		context.Run.Check("engine-switch points docker at "+context.engineWanted(), context.engineReady)
	}
	if context.Config.Enabled("DEV_TOOLS", "kubernetes") {
		context.Run.Check("kubectl, helm, k9s and kind are installed", commands("kubectl", "helm", "k9s", "kind"))
	}
	if context.Config.Enabled("DEV_TOOLS", "neovim") {
		context.Run.Check("Neovim 0.11 or newer is installed", neovimReady)
	}
	if podman {
		context.Run.Check("Podman is installed", commands("podman"))
	}
	if context.Config.Enabled("DEV_TOOLS", "toolbox") {
		context.Run.Check("JetBrains Toolbox is installed", commands(toolboxTool))
	}
	context.Run.Check("Language toolchains are ready", context.toolchainsReady)
}
