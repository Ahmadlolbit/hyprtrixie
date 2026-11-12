package setup

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apps"
	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fetch"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ledger"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	heliumEntry   = "helium.desktop"
	snapLimit     = 20 * time.Minute
	downloadLimit = 30 * time.Minute
	yourDefaults  = ".local/state/hyprtrixie/defaults"
)

var (
	webTypes     = []string{"x-scheme-handler/http", "x-scheme-handler/https", "text/html", "application/xhtml+xml"}
	imageTypes   = []string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "image/tiff", "image/svg+xml", "image/avif", "image/heif"}
	videoTypes   = []string{"video/mp4", "video/x-matroska", "video/webm", "video/quicktime", "video/x-msvideo", "video/mpeg", "video/ogg", "audio/mpeg", "audio/ogg", "audio/flac", "audio/x-wav", "audio/mp4", "audio/aac", "audio/opus"}
	archiveTypes = []string{
		"application/zip", "application/x-tar", "application/gzip", "application/x-compressed-tar", "application/x-7z-compressed",
		"application/vnd.rar", "application/x-rar", "application/x-xz", "application/x-xz-compressed-tar", "application/zstd", "application/x-bzip2",
	}
)

func applicationsDir() string {
	return env.Or("HYPRTRIXIE_APPLICATIONS_DIR", "/usr/share/applications")
}

func (context *Context) wantedApps() []apps.App {
	return slices.DeleteFunc(apps.All(), func(app apps.App) bool { return !context.Config.Enabled("APPS", app.ID) })
}

func extrepoAppInstall(log *runlog.Log, app apps.App, repository string) error {
	ledger.Note(ledger.Wants, app.Packages...)
	if apt.Present(app.Packages...) == nil {
		return nil
	}
	if apt.ExtrepoTry(log, repository) == nil && apt.Candidate(app.Packages[0]) != "" {
		return apt.Install(log, app.Packages...)
	}
	if !fetch.HasRelease(app.ID) {
		return errors.New("no pinned package for " + app.ID)
	}
	fmt.Fprintf(log, "The %s repository is unavailable, installing the pinned package instead\n", repository)
	return apt.InstallRelease(log, app.ID, app.Packages[0])
}

func (context *Context) installBrowser(log *runlog.Log) error {
	for _, app := range apps.All() {
		if app.ID == "helium" {
			return context.appInstall(log, app)
		}
	}
	return log.Failed("no app named helium in the catalogue")
}

func (context *Context) webappInstall(log *runlog.Log, app apps.App) error {
	if installed, _ := apt.Installed(); !installed["helium-bin"] {
		if err := context.installBrowser(log); err != nil {
			return err
		}
	}
	if err := context.Deploy.File("config/applications/"+app.ID+"-web.svg", filepath.Join(env.ShareDir(), "icons", app.ID+"-web.svg")); err != nil {
		return err
	}
	return context.Deploy.File("config/applications/"+app.ID+"-web.desktop", filepath.Join(apps.WebFolder, app.ID+"-web.desktop"))
}

func (context *Context) teamsInstall(log *runlog.Log) error {
	if context.Config.Get("TEAMS_SOURCE") == "snap" {
		if err := apt.Install(log, "snapd"); err != nil {
			return err
		}
		if err := run.LoggedWithin(snapLimit, log, "snap", "wait", "system", "seed.loaded"); err != nil {
			return err
		}
		if run.Quiet(run.Minute, "snap", "list", apps.TeamsApp) == 0 {
			return nil
		}
		return run.LoggedWithin(downloadLimit, log, "snap", "install", apps.TeamsApp)
	}
	if err := apt.ExtrepoTry(log, apps.TeamsApp); err != nil {
		return err
	}
	return apt.Install(log, apps.TeamsApp)
}

func (context *Context) appInstall(log *runlog.Log, app apps.App) error {
	kind, name, _ := strings.Cut(app.Source, ":")
	switch kind {
	case "debian":
		return apt.Install(log, app.Packages...)
	case "extrepo":
		return extrepoAppInstall(log, app, name)
	case "release":
		return apt.InstallRelease(log, name, app.Packages[0])
	case "webapp":
		return context.webappInstall(log, app)
	case "teams":
		return context.teamsInstall(log)
	}
	return log.Failed("unknown source " + app.Source + " for " + app.ID)
}

func (context *Context) repositoryInUse(repository, skip string) bool {
	installed, err := apt.Installed()
	if err != nil {
		return true
	}
	for _, app := range apps.All() {
		if app.ID != skip && app.Source == "extrepo:"+repository && app.Present(installed, context.Config.Get("TEAMS_SOURCE")) {
			return true
		}
	}
	return false
}

func repositoryOf(app apps.App) string {
	kind, name, _ := strings.Cut(app.Source, ":")
	switch kind {
	case "teams":
		return apps.TeamsApp
	case "extrepo":
		return name
	}
	return ""
}

func removeSnap(log *runlog.Log, name string) error {
	if run.Quiet(run.Minute, "snap", "list", name) != 0 {
		return nil
	}
	return run.Logged(log, "snap", "remove", name)
}

func removePackages(log *runlog.Log, app apps.App) error {
	var present []string
	installed, err := apt.Installed()
	if err != nil {
		return err
	}
	for _, spec := range app.Packages {
		if name := apt.Bare(spec); installed[name] {
			present = append(present, name)
		}
	}
	if len(present) == 0 {
		return nil
	}
	kept, err := apt.Remove(log, present...)
	if err == nil && len(kept) > 0 {
		err = log.Failed(strings.Join(kept, " ") + " stays installed, because other installed packages depend on it or recommend it")
	}
	return err
}

func (context *Context) webAppOn(browser apps.App) (apps.App, bool) {
	for _, app := range apps.All() {
		if browser.ID == "helium" && app.Source == "webapp" && context.Config.Enabled("APPS", app.ID) {
			return app, true
		}
	}
	return apps.App{}, false
}

func (context *Context) appRemove(log *runlog.Log, app apps.App) error {
	if webApp, needed := context.webAppOn(app); needed {
		return log.Failed(webApp.Name + "'s web app needs Helium, so Helium stays installed")
	}
	switch app.Source {
	case "webapp":
		return context.Deploy.Remove(filepath.Join(apps.WebFolder, app.ID+"-web.desktop"), filepath.Join(env.ShareDir(), "icons", app.ID+"-web.svg"))
	case "teams":
		if err := removeSnap(log, apps.TeamsApp); err != nil {
			return err
		}
	}
	if err := removePackages(log, app); err != nil {
		return err
	}
	repository := repositoryOf(app)
	if repository == "" || !apt.ExtrepoEnabled(repository) || context.repositoryInUse(repository, app.ID) {
		return nil
	}
	return apt.ExtrepoDisable(log, repository)
}

func AddApp(context *Context, app apps.App) {
	context.Run.Step("Install "+app.Name, func(log *runlog.Log) error { return context.appInstall(log, app) })
	context.Run.Step("Choose the apps that open images, PDFs, videos and archives", context.defaultApps)
}

func RemoveApp(context *Context, app apps.App) {
	context.Run.Step("Remove "+app.Name, func(log *runlog.Log) error { return context.appRemove(log, app) })
}

func (context *Context) defaultApp(log *runlog.Log, name, entry string, types []string) error {
	stamp := context.Person.Path(filepath.Join(yourDefaults, name))
	if fsutil.IsFile(stamp) || !fsutil.IsFile(filepath.Join(applicationsDir(), entry)) {
		return nil
	}
	shared := filepath.Join(env.StateDir(), "defaults", name)
	if !fsutil.IsFile(shared) {
		if err := context.Person.Run(log, append([]string{"xdg-mime", "default", entry}, types...)...); err != nil {
			return err
		}
	}
	if err := context.userFolder(log, yourDefaults); err != nil {
		return err
	}
	if err := context.Person.Write(log, stamp, entry+"\n"); err != nil {
		return err
	}
	return removeFiles(shared)
}

func (context *Context) defaultApps(log *runlog.Log) error {
	viewer := "okularApplication_pdf.desktop"
	if context.Config.Enabled("APPS", "evince") {
		viewer = "org.gnome.Evince.desktop"
	}
	for _, choice := range []struct {
		name, entry string
		types       []string
	}{
		{"images", "imv.desktop", imageTypes},
		{"pdf", viewer, []string{"application/pdf"}},
		{"videos", "vlc.desktop", videoTypes},
		{"archives", "org.kde.ark.desktop", archiveTypes},
	} {
		if err := context.defaultApp(log, choice.name, choice.entry, choice.types); err != nil {
			return err
		}
	}
	return nil
}

func (context *Context) appsMissing(*runlog.Log) error {
	installed, err := apt.Installed()
	if err != nil {
		return err
	}
	source := context.Config.Get("TEAMS_SOURCE")
	var missing []string
	for _, app := range context.wantedApps() {
		if !app.Present(installed, source) {
			missing = append(missing, app.ID)
		}
	}
	return expect(len(missing) == 0, "not installed: "+strings.Join(missing, " "))
}

func (context *Context) heliumIsDefault(log *runlog.Log) error {
	output, err := run.CaptureTo(run.Minute, log, "runuser", context.Person.RunuserArgs("xdg-mime", "query", "default", "x-scheme-handler/https")...)
	return expect(err == nil && output == heliumEntry, "the default is "+cmp.Or(output, "not set"))
}

func (context *Context) waitingForRepositories() []apps.App {
	source := context.Config.Get("TEAMS_SOURCE")
	return slices.DeleteFunc(context.wantedApps(), func(app apps.App) bool {
		name := repositoryOf(app)
		return name == "" || app.Source == "teams" && source == "snap" || apt.ExtrepoEnabled(name)
	})
}

func (context *Context) repositoriesAhead() []string {
	waiting := context.waitingForRepositories()
	if len(waiting) == 0 {
		return nil
	}
	installed, err := apt.Installed()
	if err != nil {
		return nil
	}
	var names []string
	for _, app := range waiting {
		if name := repositoryOf(app); !slices.Contains(names, name) && !app.Present(installed, context.Config.Get("TEAMS_SOURCE")) {
			names = append(names, name)
		}
	}
	return names
}

func appSteps(context *Context) {
	ahead := context.repositoriesAhead()
	for _, app := range context.wantedApps() {
		context.Run.Step("Install "+app.Name, func(log *runlog.Log) error {
			apt.ExtrepoAhead(log, ahead...)
			ahead = nil
			return context.appInstall(log, app)
		})
	}
	if context.Config.Enabled("APPS", "helium") {
		context.Run.Step("Make Helium the default browser", func(log *runlog.Log) error {
			return context.Person.Run(log, append([]string{"xdg-mime", "default", heliumEntry}, webTypes...)...)
		})
	}
	context.Run.Step("Choose the apps that open images, PDFs, videos and archives", context.defaultApps)
	chezmoiSteps(context)
	extrasSteps(context)
}

func appChecks(context *Context) {
	context.Run.Check("Every ticked app is installed", context.appsMissing)
	if context.Config.Enabled("APPS", "helium") {
		context.Run.Check("Helium is the default browser", context.heliumIsDefault)
	}
	extrasChecks(context)
}
