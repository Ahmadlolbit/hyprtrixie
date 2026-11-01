package setup

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
)

const (
	minFreeGB  = 30
	newAccount = "Create a new account"
	adminGroup = "sudo"
)

var (
	accountName = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	adminRule   = regexp.MustCompile(`\(ALL( : ALL)?\) ALL`)
	errNoAnswer = errors.New("no answer")
)

func osRelease() string {
	return env.Or("HYPRTRIXIE_OS_RELEASE", "/etc/os-release")
}

func osValue(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if fields := strings.Split(line, "="); fields[0] == key && len(fields) > 1 {
			return strings.ReplaceAll(fields[1], `"`, "")
		}
	}
	return ""
}

func orUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func preflightOS(log *runlog.Log) error {
	text := fsutil.Optional(osRelease())
	id, codename := osValue(text, "ID"), osValue(text, "VERSION_CODENAME")
	if id == "debian" && codename == "trixie" {
		return nil
	}
	fmt.Fprintf(log, "This system is %s %s; hyprtrixie is made for Debian 13 (trixie).\n", orUnknown(id), orUnknown(codename))
	return holds(false)
}

func preflightArch(log *runlog.Log) error {
	output, _ := run.CaptureQuiet(run.Quick, "dpkg", "--print-architecture")
	arch := strings.TrimSpace(output)
	if arch == "amd64" {
		return nil
	}
	fmt.Fprintf(log, "This system is %s; hyprtrixie supports amd64 only.\n", arch)
	return holds(false)
}

func preflightSecureBoot(log *runlog.Log) error {
	state := detect.SecureBoot()
	fmt.Fprintf(log, "Secure Boot: %s\n", state)
	if state != "enabled" {
		return nil
	}
	fmt.Fprintln(log, "Turn Secure Boot off in the firmware settings: drivers built on this machine are not signed.")
	return holds(false)
}

func preflightNetwork(log *runlog.Log) error {
	return run.Logged(log, "curl", "-fsS", "--max-time", "20", "-o", "/dev/null", "https://deb.debian.org/debian/dists/trixie/InRelease")
}

func freeSpace() string {
	output, _ := run.CaptureQuiet(run.Quick, "df", "--output=avail", "-BG", "/")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	return strings.TrimFunc(lines[len(lines)-1], func(char rune) bool { return char < '0' || char > '9' })
}

func preflightDisk(log *runlog.Log) error {
	text := freeSpace()
	fmt.Fprintf(log, "%s GiB free on /\n", text)
	if free, _ := strconv.Atoi(text); free >= minFreeGB {
		return nil
	}
	fmt.Fprintf(log, "At least %d GiB of free space is needed.\n", minFreeGB)
	return holds(false)
}

func (context *Context) ask(ask func() (string, bool)) (string, error) {
	answer, ok := context.Run.Answer(ask)
	if !ok {
		return "", errNoAnswer
	}
	return answer, nil
}

func (context *Context) pickAccount() (string, error) {
	var names []string
	for _, person := range account.Humans() {
		names = append(names, person.Name)
	}
	if len(names) == 0 {
		return newAccount, nil
	}
	return context.ask(func() (string, bool) {
		return ui.Choose("Which account should this computer be set up for?", append(names, newAccount))
	})
}

func (context *Context) makeAccount(log *runlog.Log) (string, error) {
	name, err := context.ask(func() (string, bool) { return ui.Input("Name for the new account (lowercase, no spaces)") })
	if err != nil {
		return "", err
	}
	if !accountName.MatchString(name) {
		fmt.Fprintln(log, "Account names use lowercase letters, digits, - and _.")
		return "", holds(false)
	}
	return name, run.Logged(log, "adduser", "--disabled-password", "--comment", "", name)
}

func (context *Context) chooseAccount(log *runlog.Log) error {
	pick, err := context.pickAccount()
	if err == nil && pick == newAccount {
		pick, err = context.makeAccount(log)
	}
	if err != nil {
		return err
	}
	person, found := account.Lookup(pick)
	if !found {
		return errors.New("no account named " + pick)
	}
	context.Person = person
	if err := log.Attach(person.Home, person.Name); err != nil {
		log.Line("warn", "no copy of the log: "+err.Error())
	}
	return nil
}

func (context *Context) newPassword() (string, error) {
	for {
		first, err := context.ask(func() (string, bool) { return ui.Password("Choose a password for " + context.Person.Name) })
		if err != nil {
			return "", err
		}
		second, err := context.ask(func() (string, bool) { return ui.Password("Type it once more") })
		if err != nil {
			return "", err
		}
		if first != "" && first == second {
			return first, nil
		}
		context.Run.Warn("Those didn't match, try again")
	}
}

func (context *Context) setPassword(log *runlog.Log) error {
	password, err := context.newPassword()
	if err != nil {
		return err
	}
	return run.Supply(run.Minute, log, context.Person.Name+":"+password+"\n", "chpasswd")
}

func hasPassword(name string) bool {
	output, _ := run.CaptureQuiet(run.Quick, "passwd", "-S", name)
	fields := strings.Fields(output)
	return len(fields) > 1 && fields[1] == "P"
}

func (context *Context) ensurePassword(log *runlog.Log) error {
	if hasPassword(context.Person.Name) {
		return nil
	}
	if !context.CanAsk {
		fmt.Fprintf(log, "%[1]s has no password yet; set one with: passwd %[1]s\n", context.Person.Name)
		return holds(false)
	}
	return context.setPassword(log)
}

func (context *Context) ensureAccount(log *runlog.Log) error {
	if context.Person.Name != "" {
		return nil
	}
	if !context.CanAsk {
		fmt.Fprintln(log, "No admin account found. Run hyprtrixie with sudo from your own account.")
		return holds(false)
	}
	return context.chooseAccount(log)
}

func (context *Context) preflightUser() error {
	log := context.Run.Log
	if err := context.ensureAccount(log); err != nil {
		return err
	}
	if err := apt.Install(log, adminGroup); err != nil {
		return err
	}
	if err := context.Person.AddGroups(log, adminGroup); err != nil {
		return err
	}
	if err := context.ensurePassword(log); err != nil {
		return err
	}
	context.Run.Note("Admin account: " + context.Person.Name + " (" + context.Person.Home + ")")
	return nil
}

func preflightDetect(log *runlog.Log) error {
	fmt.Fprint(log, detect.Summary())
	return nil
}

func preflightSteps(context *Context) {
	context.Run.Step("Debian 13 (trixie)", preflightOS)
	context.Run.Step("64-bit PC (amd64)", preflightArch)
	context.Run.Step("Secure Boot is off", preflightSecureBoot)
	context.Run.Step("Internet connection", preflightNetwork)
	context.Run.Step("Enough free disk space", preflightDisk)
	context.Run.Interactive("Admin account", context.preflightUser)
	context.Run.Step("Detect hardware", preflightDetect)
}

func (context *Context) adminCheckTitle() string {
	if context.Person.Name == "" {
		return "The admin account can use sudo"
	}
	return context.Person.Name + " can use sudo"
}

func (context *Context) userIsAdmin(*runlog.Log) error {
	if context.Person.Name == "" {
		return holds(false)
	}
	output, err := run.CaptureQuiet(run.Quick, "sudo", "-l", "-U", context.Person.Name)
	return holds(err == nil && adminRule.MatchString(output))
}

func preflightChecks(context *Context) {
	context.Run.Check(context.adminCheckTitle(), context.userIsAdmin)
	context.Run.Check("Running on Debian 13 (trixie)", preflightOS)
}
