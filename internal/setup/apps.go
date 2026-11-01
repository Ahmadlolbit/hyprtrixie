package setup

import (
	"cmp"
	"errors"
	"fmt"
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
)

const heliumEntry = "helium.desktop"

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
	if !apt.Installed()["helium-bin"] {
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
		if err := run.Logged(log, "snap", "wait", "system", "seed.loaded"); err != nil {
			return err
		}
		if run.Quiet(run.Minute, "snap", "list", apps.TeamsApp) == 0 {
			return nil
		}
		return run.Logged(log, "snap", "install", apps.TeamsApp)
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

func (context *Context) defaultApp(log *runlog.Log, name, entry string, types []string) error {
	stamp := filepath.Join(env.StateDir(), "defaults", name)
	if fsutil.IsFile(stamp) || !fsutil.IsFile(filepath.Join(applicationsDir(), entry)) {
		return nil
	}
	if err := context.Person.Run(log, append([]string{"xdg-mime", "default", entry}, types...)...); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(stamp), programMode); err != nil {
		return err
	}
	return atomic.Write(stamp, []byte(entry+"\n"), plainMode)
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
	installed := apt.Installed()
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

func appSteps(context *Context) {
	for _, app := range context.wantedApps() {
		context.Run.Step("Install "+app.Name, func(log *runlog.Log) error { return context.appInstall(log, app) })
	}
	if context.Config.Enabled("APPS", "helium") {
		context.Run.Step("Make Helium the default browser", func(log *runlog.Log) error {
			return context.Person.Run(log, append([]string{"xdg-mime", "default", heliumEntry}, webTypes...)...)
		})
	}
	context.Run.Step("Choose the apps that open images, PDFs, videos and archives", context.defaultApps)
}

func appChecks(context *Context) {
	context.Run.Check("Every ticked app is installed", context.appsMissing)
	if context.Config.Enabled("APPS", "helium") {
		context.Run.Check("Helium is the default browser", context.heliumIsDefault)
	}
}
