package engine

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/events"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/modules"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/setup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
)

const (
	lockMode     = 0o644
	askMode      = "ask"
	continueMode = "continue"
	installMode  = "install"
)

var (
	ErrHelp = errors.New("help")

	failureModes = []string{askMode, "halt", continueMode}
)

type Options struct {
	Config    string
	Only      string
	Skip      string
	Yes       bool
	OnFailure string
	Changes   string
	Events    bool
}

type session struct {
	mode     string
	options  Options
	log      *runlog.Log
	runner   *step.Runner
	person   account.Account
	settings *config.Config
	canAsk   bool
	before   []string
	started  time.Time
	results  map[string]string
}

func setConfig(options *Options, value string) error {
	if !fsutil.IsFile(value) {
		return errors.New("no such configuration file: " + value)
	}
	options.Config = value
	return nil
}

func setOnFailure(options *Options, value string) error {
	if !slices.Contains(failureModes, value) {
		return errors.New("--on-failure takes ask, halt or continue")
	}
	options.OnFailure = value
	return nil
}

func setOnly(options *Options, value string) error {
	options.Only = value
	return nil
}

func setSkip(options *Options, value string) error {
	options.Skip = value
	return nil
}

func setChanges(options *Options, value string) error {
	options.Changes = value
	return nil
}

var takesValue = map[string]func(*Options, string) error{
	"--config":     setConfig,
	"--only":       setOnly,
	"--skip":       setSkip,
	"--changes":    setChanges,
	"--on-failure": setOnFailure,
}

func valueAt(args []string, index int) string {
	if index < len(args) {
		return args[index]
	}
	return ""
}

func Parse(args []string) (Options, error) {
	var options Options
	for index := 0; index < len(args); index++ {
		switch set, valued := takesValue[args[index]]; {
		case valued:
			index++
			if err := set(&options, valueAt(args, index)); err != nil {
				return options, err
			}
		case args[index] == "--yes" || args[index] == "-y":
			options.Yes = true
		case args[index] == "--events":
			options.Events = true
		case args[index] == "-h" || args[index] == "--help":
			return options, ErrHelp
		default:
			return options, errors.New("unknown option: " + args[index] + " (see: hyprtrixie help)")
		}
	}
	return options, nil
}

func listed(list, id string) bool {
	return strings.Contains(","+list+",", ","+id+",")
}

func (options Options) wants(id string) bool {
	return (options.Only == "" || listed(options.Only, id)) && (options.Skip == "" || !listed(options.Skip, id))
}

func lockPath() string {
	return env.Or("HYPRTRIXIE_LOCK", "/run/hyprtrixie.lock")
}

func takeLock() (*os.File, error) {
	path := lockPath()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, lockMode)
	if err != nil {
		return nil, errors.New("cannot create " + path)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("another hyprtrixie run is still going")
	}
	return file, nil
}

func packages() []string {
	output, _ := run.CaptureQuiet(run.Slow, "dpkg-query", "-W", "-f=${Package}\n")
	return run.Lines(strings.TrimSuffix(output, "\n"))
}

func start(mode string, options Options, args []string) (*session, error) {
	log, err := runlog.Start(mode + " " + strings.Join(args, " "))
	if err != nil {
		return nil, errors.New("cannot write logs to " + env.LogDir())
	}
	work := &session{
		mode: mode, options: options, log: log, started: time.Now(), results: map[string]string{},
		canAsk: ui.CanAsk() && !options.Yes, before: packages(),
		runner:   step.New(log, "", askMode),
		settings: config.Read(config.Path(), options.Config),
	}
	for _, note := range work.settings.Notes {
		log.Line(note.Level, note.Text)
	}
	if options.Events {
		log.Mirror(ui.Mirror)
	}
	if person, found := account.Target(); found {
		work.person = person
		if err := log.Attach(person.Home, person.Name); err != nil {
			log.Line("warn", "no copy of the log: "+err.Error())
		}
	}
	return work, nil
}

func (work *session) applyOverrides() {
	if work.options.OnFailure != "" {
		work.settings.Set("FAILURE_MODE", work.options.OnFailure)
	}
	work.runner.Mode = work.settings.Get("FAILURE_MODE")
	if work.runner.Mode == askMode && !work.canAsk {
		work.runner.Mode = continueMode
	}
}

func (work *session) refreshPerson() {
	if person, found := account.Lookup(work.person.Name); work.person.Name != "" && found {
		work.person = person
	}
}

func (work *session) context() (*setup.Context, error) {
	work.refreshPerson()
	context := &setup.Context{Run: work.runner, Person: work.person, Config: work.settings, CanAsk: work.canAsk}
	if work.mode != installMode {
		return context, nil
	}
	deployer, err := files.New(work.log, work.person)
	context.Deploy = deployer
	return context, err
}

func (work *session) save() {
	if err := modules.SaveStatus(work.results); err != nil {
		work.log.Line("warn", "cannot save the status: "+err.Error())
	}
}

func (work *session) guarded(part func(setup.Module, *setup.Context), body setup.Module, context *setup.Context) {
	defer func() {
		if caught := recover(); caught != nil {
			work.log.Write(debug.Stack())
			work.runner.Broke("step", fmt.Sprintf("module %s stopped: %v", work.runner.Module, caught))
		}
	}()
	part(body, context)
}

func (work *session) one(module modules.Module, position, total int, part func(setup.Module, *setup.Context)) bool {
	work.runner.Begin(module.ID)
	ui.Rule(fmt.Sprintf("%s  %s  ·  %d of %d", module.ID, module.Title, position, total))
	work.log.Line("info", "module "+module.ID+" begin: "+module.Title)
	context, err := work.context()
	if err != nil {
		work.runner.Broke("step", "cannot read what's installed: "+err.Error())
		return false
	}
	work.guarded(part, setup.Modules[module.ID], context)
	work.person = context.Person
	if work.runner.Ended() {
		return false
	}
	result := work.runner.Result()
	work.results[module.ID] = result
	work.log.Line("info", "module "+module.ID+" end: "+result)
	work.save()
	return true
}

func (work *session) walk(part func(setup.Module, *setup.Context)) {
	var chosen []modules.Module
	for _, module := range modules.All {
		if work.options.wants(module.ID) {
			chosen = append(chosen, module)
		} else {
			work.log.Line("info", "module "+module.ID+" not selected")
		}
	}
	for index, module := range chosen {
		if !work.one(module, index+1, len(chosen), part) {
			return
		}
	}
}

func stepsAndChecks(body setup.Module, context *setup.Context) {
	body.Steps(context)
	body.Checks(context)
}

func checksOnly(body setup.Module, context *setup.Context) {
	body.Checks(context)
}

func (work *session) outcome() int {
	code := work.runner.ExitCode()
	if code == 0 && len(work.runner.Failures) > 0 {
		return 1
	}
	return code
}

func (work *session) showMachine() {
	for _, line := range run.Lines(strings.TrimSuffix(detect.Summary(), "\n")) {
		work.log.Line("info", line)
		if !work.options.Events {
			ui.Print(line + "\n")
		}
	}
}

func (work *session) confirmed() bool {
	if !work.canAsk || work.options.Events {
		return true
	}
	_, ok := work.runner.Answer(func() (string, bool) { return "", ui.Proceed("Start the setup now?") })
	return ok
}

func (work *session) install() int {
	if !work.options.Events {
		ui.Header("hyprtrixie "+env.Version, "Debian 13 with Hyprland, set up properly")
	}
	work.showMachine()
	if !work.confirmed() {
		return work.runner.ExitCode()
	}
	if err := work.settings.Save(config.Path()); err != nil {
		work.log.Line("warn", "cannot save the configuration: "+err.Error())
	}
	work.applyOverrides()
	work.walk(stepsAndChecks)
	return work.outcome()
}

func (work *session) verify() int {
	work.applyOverrides()
	work.walk(checksOnly)
	return work.outcome()
}

func (work *session) finish(code int) int {
	if work.runner.Interrupted {
		ui.Print("\n")
		work.log.Line("warn", "interrupted")
	} else if work.runner.Halted {
		work.log.Line("error", "run halted in module "+work.runner.Module)
	}
	work.save()
	if work.person.Home != "" {
		if err := setup.WriteReport(&setup.Context{Run: work.runner, Person: work.person}); err != nil {
			work.log.Line("warn", "could not write the report")
		}
	}
	work.summary()
	work.log.Line("info", "finished with exit code "+strconv.Itoa(code))
	return code
}

func entered(mode string, args []string, body func(*session) int) int {
	options, err := Parse(args)
	if err != nil {
		return run.Report(err)
	}
	lock, err := takeLock()
	if err != nil {
		return run.Report(err)
	}
	defer lock.Close()
	defer term.Keep()()
	cleanup.SurviveBrokenPipes()
	if options.Events {
		cleanup.OwnGroup()
		ui.Attach(events.Open(os.Stdin, os.Stdout, cleanup.StopGroup))
	}
	work, err := start(mode, options, args)
	if err != nil {
		return run.Report(err)
	}
	return work.finish(body(work))
}

func Install(args []string) int {
	return entered(installMode, args, (*session).install)
}

func Verify(args []string) int {
	return entered("verify", args, (*session).verify)
}
