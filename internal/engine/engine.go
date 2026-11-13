package engine

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/apps"
	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/deployed"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/events"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/firewall"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ledger"
	"github.com/Ahmadlolbit/hyprtrixie/internal/lock"
	"github.com/Ahmadlolbit/hyprtrixie/internal/modules"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/setup"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
	"github.com/Ahmadlolbit/hyprtrixie/internal/words"
)

const (
	askMode      = "ask"
	continueMode = "continue"
	installMode  = "install"
	verifyMode   = "verify"
	accountMode  = "account"
	appMode      = "app-"
	appsModule   = "10"
	noModule     = "none"
	configLimit  = 64 << 10
	keepYours    = "Keep yours, leaving the configuration alone"
	useRuns      = "Use the run's, overwriting your choices"
	mergeBoth    = "Merge the two: the run's changes, and yours where you both changed a setting"
)

var (
	ErrHelp      = errors.New("help")
	AfterTesting = func() {}

	failureModes  = []string{askMode, "halt", continueMode}
	editedChoices = []string{keepYours, useRuns, mergeBoth}
)

type Options struct {
	Config    string
	Only      string
	Skip      string
	Yes       bool
	OnFailure string
	Changes   string
	Events    bool
	Testing   bool
}

type session struct {
	mode      string
	options   Options
	log       *runlog.Log
	runner    *step.Runner
	person    account.Account
	settings  *config.Config
	canAsk    bool
	before    []string
	started   time.Time
	results   map[string]string
	installed map[string]bool
	machine   string
	hardware  detect.Hardware
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

func moduleNumbers(flag, value string) error {
	for _, id := range strings.Split(value, ",") {
		if id == "" {
			return errors.New(flag + " takes module numbers, such as 04,05")
		}
		if !slices.ContainsFunc(modules.All, func(module modules.Module) bool { return module.ID == id }) {
			return errors.New("no module " + id + ": " + flag + " takes module numbers, such as 04,05 (see: hyprtrixie modules)")
		}
	}
	return nil
}

func setOnly(options *Options, value string) error {
	options.Only = value
	return moduleNumbers("--only", value)
}

func setSkip(options *Options, value string) error {
	options.Skip = value
	return moduleNumbers("--skip", value)
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
		case args[index] == "--testing":
			options.Testing = true
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

func packages() []string {
	output, _ := run.CaptureQuiet(run.Slow, "dpkg-query", "-W", "-f=${Package}\n")
	return run.Lines(strings.TrimSuffix(output, "\n"))
}

func callers() []int {
	allowed := []int{0}
	if uid, err := strconv.Atoi(os.Getenv("SUDO_UID")); err == nil {
		allowed = append(allowed, uid)
	}
	return allowed
}

func settingsFor(options Options) (*config.Config, error) {
	settings := config.Read(config.Path())
	if options.Config == "" {
		return settings, nil
	}
	text, err := fsutil.ReadPlain(options.Config, configLimit, callers()...)
	if err != nil {
		return nil, err
	}
	settings.MergeText(options.Config, string(text))
	return settings, nil
}

func start(mode string, options Options, args []string) (*session, error) {
	settings, err := settingsFor(options)
	if err != nil {
		return nil, err
	}
	log, err := runlog.Start(mode + " " + strings.Join(args, " "))
	if err != nil {
		return nil, errors.New("cannot write logs to " + env.LogDir())
	}
	work := &session{
		mode: mode, options: options, log: log, started: time.Now(), results: map[string]string{},
		canAsk: ui.CanAsk() && !options.Yes, before: packages(),
		runner:   step.New(log, "", askMode),
		settings: settings,
	}
	for _, note := range work.settings.Notes {
		log.Line(note.Level, note.Text)
	}
	if options.Events {
		log.Mirror(ui.Mirror)
	}
	if options.Testing {
		run.Trace(log)
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

func (work *session) changesFiles() bool {
	return work.mode == installMode || strings.HasPrefix(work.mode, appMode)
}

func (work *session) context() (*setup.Context, error) {
	work.refreshPerson()
	context := &setup.Context{Run: work.runner, Person: work.person, Config: work.settings, CanAsk: work.canAsk, Machine: work.machine, Hardware: work.readHardware}
	if !work.changesFiles() {
		return context, nil
	}
	deployer, err := files.New(work.log, work.person)
	context.Deploy = deployer
	return context, err
}

func (work *session) kept() map[string]string {
	if work.mode != verifyMode {
		return work.results
	}
	statuses := fsutil.Optional(modules.StatusFile())
	kept := map[string]string{}
	for id, result := range work.results {
		if status, _ := modules.StatusOf(statuses, id); status != "" && status != modules.Undone {
			kept[id] = result
		}
	}
	return kept
}

func (work *session) save() {
	if err := modules.SaveStatus(work.kept()); err != nil {
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

type recording struct {
	on     bool
	before map[string]bool
}

func (work *session) packagesNow(id string) map[string]bool {
	installed, err := apt.Installed()
	if err != nil {
		work.log.Line("warn", "cannot list the installed packages, so module "+id+" records none: "+err.Error())
		return nil
	}
	return installed
}

func (work *session) recordStart(id string) recording {
	if !work.changesFiles() {
		return recording{}
	}
	before := work.installed
	if before == nil {
		before = work.packagesNow(id)
	}
	ledger.Begin(id, before)
	return recording{true, before}
}

func (work *session) recordEnd(id string, record recording) {
	if !record.on {
		return
	}
	var after map[string]bool
	if record.before != nil {
		after = work.packagesNow(id)
	}
	work.installed = after
	if err := ledger.End(id, record.before, after); err != nil {
		work.log.Line("warn", "cannot record what module "+id+" did: "+err.Error())
	}
}

func (work *session) ended(id string) {
	if work.runner.Halted && !work.runner.Interrupted {
		work.results[id] = work.runner.Result()
		work.log.Line("info", "module "+id+" end: "+work.results[id])
	}
}

func (work *session) one(module modules.Module, position, total int, part func(setup.Module, *setup.Context)) bool {
	work.runner.Begin(module.ID)
	ui.Rule(fmt.Sprintf(words.T("%s  %s  ·  %d of %d"), module.ID, words.T(module.Title), position, total))
	work.log.Line("info", "module "+module.ID+" begin: "+module.Title)
	context, err := work.context()
	if err != nil {
		work.runner.Broke("step", "cannot read what's installed: "+err.Error())
		work.ended(module.ID)
		return false
	}
	record := work.recordStart(module.ID)
	work.guarded(part, setup.Modules[module.ID], context)
	work.recordEnd(module.ID, record)
	work.person = context.Person
	if work.runner.Ended() {
		work.ended(module.ID)
		return false
	}
	result := work.runner.Result()
	work.results[module.ID] = result
	work.log.Line("info", "module "+module.ID+" end: "+result)
	work.save()
	return true
}

func (work *session) undone(id string) bool {
	if listed(work.options.Only, id) {
		return false
	}
	status, _ := modules.StatusOf(fsutil.Optional(modules.StatusFile()), id)
	return status == modules.Undone
}

func (work *session) walk(part func(setup.Module, *setup.Context)) {
	var chosen []modules.Module
	for _, module := range modules.All {
		switch {
		case !work.options.wants(module.ID):
			work.log.Line("info", "module "+module.ID+" not selected")
		case work.undone(module.ID):
			work.log.Line("info", "module "+module.ID+" was undone; run it again with --only "+module.ID)
		default:
			chosen = append(chosen, module)
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

func (work *session) survey() {
	if work.machine == "" {
		work.hardware, work.machine = detect.Survey()
	}
}

func (work *session) readHardware() detect.Hardware {
	work.survey()
	return work.hardware
}

func (work *session) showMachine() {
	work.survey()
	for _, line := range run.Lines(strings.TrimSuffix(work.machine, "\n")) {
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

func (work *session) editedChoice(path string) (string, bool) {
	if !work.canAsk {
		return mergeBoth, true
	}
	return work.runner.Answer(func() (string, bool) {
		return work.runner.Choose(path+" changed after this run read it. Which configuration should it keep?", editedChoices)
	})
}

func (work *session) saveSettings() bool {
	path, settings := config.Path(), work.settings
	if settings.Edited(path) {
		choice, ok := work.editedChoice(path)
		if !ok {
			return false
		}
		work.log.Line("info", "the configuration changed after this run read it: "+choice)
		if choice == keepYours {
			return true
		}
		if choice == mergeBoth {
			settings = settings.Merged(path)
		}
	}
	if err := settings.Save(path); err != nil {
		work.log.Line("warn", "cannot save the configuration: "+err.Error())
	}
	return true
}

func (work *session) install() int {
	if !work.options.Events {
		ui.Header("hyprtrixie "+env.Version, "Debian 13 with Hyprland, set up properly")
	}
	work.showMachine()
	if !work.confirmed() || !work.saveSettings() {
		return work.runner.ExitCode()
	}
	work.applyOverrides()
	work.walk(stepsAndChecks)
	work.noteFinished()
	return work.outcome()
}

func (work *session) noteFinished() {
	if work.runner.ExitCode() != 0 || work.options.Only != "" || work.options.Skip != "" {
		return
	}
	if err := deployed.NoteFinished(env.Root()); err != nil {
		work.log.Line("warn", "cannot note that the setup finished: "+err.Error())
	}
}

func (work *session) setUpAccount() int {
	work.runner.Begin(accountMode)
	context, err := work.context()
	if err != nil {
		work.runner.Broke("step", err.Error())
		return work.outcome()
	}
	setup.AccountSteps(context)
	work.person = context.Person
	if work.outcome() == 0 && work.person.Name != "" {
		ui.Print("\nLog in as " + work.person.Name + " and run: " + context.CloneFor() + "/hyprtrixie\n")
	}
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
		work.survey()
		if err := setup.WriteReport(&setup.Context{Run: work.runner, Person: work.person, Machine: work.machine}); err != nil {
			work.log.Line("warn", "could not write the report")
		}
	}
	work.summary()
	work.log.Line("info", "finished with exit code "+strconv.Itoa(code))
	return code
}

func entered(mode string, options Options, args []string, body func(*session) int) int {
	held, err := lock.Take()
	if err != nil {
		return run.Report(err)
	}
	defer held.Close()
	defer term.Keep()()
	defer run.Remember()()
	cleanup.SurviveBrokenPipes()
	if options.Events {
		cleanup.OwnGroup()
		ui.Attach(events.Open(os.Stdin, os.Stdout, cleanup.StopGroup))
	}
	work, err := start(mode, options, args)
	if err != nil {
		return run.Report(err)
	}
	code := work.finish(body(work))
	if options.Testing {
		AfterTesting()
	}
	return code
}

func parsed(mode string, args []string, body func(*session) int) int {
	options, err := Parse(args)
	if err != nil {
		return run.Report(err)
	}
	return entered(mode, options, args, body)
}

func Install(args []string) int {
	return parsed(installMode, args, (*session).install)
}

func Account(args []string) int {
	return parsed(accountMode, args, (*session).setUpAccount)
}

func Verify(args []string) int {
	return parsed(verifyMode, args, (*session).verify)
}

func (work *session) changeApp(app apps.App, change func(*config.Config), steps func(*setup.Context, apps.App)) int {
	work.runner.Begin(noModule)
	change(work.settings)
	if !work.saveSettings() {
		return work.outcome()
	}
	return work.appSteps(app, steps)
}

func (work *session) appSteps(app apps.App, steps func(*setup.Context, apps.App)) int {
	work.applyOverrides()
	context, err := work.context()
	if err != nil {
		work.runner.Broke("step", "cannot read what's installed: "+err.Error())
		return work.outcome()
	}
	record := work.recordStart(appsModule)
	steps(context, app)
	work.recordEnd(appsModule, record)
	work.forgetRemoved(record.before)
	return work.outcome()
}

func (work *session) removeApp(app apps.App) int {
	work.runner.Begin(noModule)
	if work.appSteps(app, setup.RemoveApp) == 0 {
		work.settings.Untick("APPS", app.ID)
		work.saveSettings()
	}
	return work.outcome()
}

func (work *session) forgetRemoved(before map[string]bool) {
	if before == nil || work.installed == nil {
		return
	}
	for name := range before {
		if work.installed[name] {
			continue
		}
		for _, kind := range []string{ledger.Package, ledger.Wants} {
			if err := ledger.Forget(appsModule, kind, name); err != nil {
				work.log.Line("warn", "cannot record that "+name+" was removed: "+err.Error())
			}
		}
	}
}

func (work *session) followed(app apps.App, code int) int {
	if code != 0 || !firewall.Opens(app.ID) {
		return code
	}
	if err := firewall.Refresh(work.log); err != nil {
		work.log.Line("warn", "the firewall can't follow "+app.Name+": "+err.Error())
		ui.Warn("The firewall can't follow " + app.Name + ": " + err.Error())
	}
	return code
}

func AddApp(app apps.App) int {
	tick := func(settings *config.Config) { settings.Tick("APPS", app.ID) }
	return entered(appMode+"install", Options{}, []string{app.ID}, func(work *session) int {
		return work.followed(app, work.changeApp(app, tick, setup.AddApp))
	})
}

func RemoveApp(app apps.App) int {
	return entered(appMode+"remove", Options{}, []string{app.ID}, func(work *session) int {
		return work.followed(app, work.removeApp(app))
	})
}
