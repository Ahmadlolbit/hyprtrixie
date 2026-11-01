package setup

import (
	"fmt"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/modules"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

const (
	ReportName = "runbook-report.md"
	lastModule = "17"
	reportEnd  = `
## Next

- Reboot to start the desktop.
- ` + "`sudo hyprtrixie verify`" + ` runs every check again.
`
)

func moduleTable(text *strings.Builder) {
	saved := fsutil.Optional(modules.StatusFile())
	text.WriteString("## Modules\n\n| Module | Result | When |\n|---|---|---|\n")
	for _, module := range modules.All {
		result, when := modules.StatusOf(saved, module.ID)
		if result == "" {
			result = "not run"
		}
		fmt.Fprintf(text, "| %s %s | %s | %s |\n", module.ID, module.Title, result, when)
	}
}

func markList(text *strings.Builder, heading string, marks []step.Mark) {
	if len(marks) == 0 {
		return
	}
	text.WriteString("\n## " + heading + "\n\n")
	for _, mark := range marks {
		fmt.Fprintf(text, "- %s %s: %s\n", mark.Module, mark.Kind, mark.Title)
	}
}

func report(context *Context) string {
	var text strings.Builder
	text.WriteString("# hyprtrixie report\n\n")
	fmt.Fprintf(&text, "- Date: %s\n- hyprtrixie: %s\n- Configuration: %s\n- Log: %s\n\n",
		time.Now().Format(time.DateTime), env.Version, config.Path(), context.Run.Log.Path())
	text.WriteString("## Machine\n\n")
	for _, line := range strings.Split(strings.TrimSuffix(detect.Summary(), "\n"), "\n") {
		text.WriteString("    " + line + "\n")
	}
	text.WriteString("\n")
	moduleTable(&text)
	markList(&text, "Failed", context.Run.Failures)
	markList(&text, "Skipped", context.Run.Skips)
	text.WriteString(reportEnd)
	return text.String()
}

func WriteReport(context *Context) error {
	uid, gid, err := context.Person.IDs()
	if err != nil {
		return err
	}
	return atomic.InstallAs(context.Person.Path(ReportName), []byte(report(context)), plainMode, uid, gid)
}

func reportSteps(context *Context) {
	for _, module := range modules.All {
		if body, known := Modules[module.ID]; known && module.ID != lastModule {
			body.Checks(context)
		}
	}
	context.Run.Step("Write ~/"+ReportName, func(*runlog.Log) error { return WriteReport(context) })
}

func noChecks(*Context) {}
