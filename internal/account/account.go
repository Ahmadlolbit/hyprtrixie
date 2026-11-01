package account

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	passwdFields = 7
	firstHuman   = 1000
	firstSystem  = 60000
)

type Account struct {
	Name  string
	UID   string
	GID   string
	Home  string
	Shell string
}

func parse(line string) (Account, bool) {
	fields := strings.Split(line, ":")
	if len(fields) < passwdFields {
		return Account{}, false
	}
	return Account{Name: fields[0], UID: fields[2], GID: fields[3], Home: fields[5], Shell: fields[6]}, true
}

func (person Account) human() bool {
	uid, err := strconv.Atoi(person.UID)
	return err == nil && uid >= firstHuman && uid < firstSystem &&
		!strings.HasSuffix(person.Shell, "nologin") && !strings.HasSuffix(person.Shell, "false")
}

func humans() []Account {
	output, err := run.Capture(run.Quick, "getent", "passwd")
	if err != nil {
		return nil
	}
	var found []Account
	for _, line := range run.Lines(output) {
		if person, ok := parse(line); ok && person.human() {
			found = append(found, person)
		}
	}
	return found
}

func Lookup(name string) (Account, bool) {
	output, err := run.Capture(run.Quick, "getent", "passwd", name)
	if err != nil {
		return Account{}, false
	}
	line, _, _ := strings.Cut(output, "\n")
	return parse(line)
}

func Target() (Account, bool) {
	name := env.Or("HYPRTRIXIE_USER", os.Getenv("SUDO_USER"))
	if name != "" && name != "root" {
		return Lookup(name)
	}
	if people := humans(); len(people) == 1 {
		return people[0], true
	}
	return Account{}, false
}

func (person Account) IDs() (int, int, error) {
	uid, err := strconv.Atoi(person.UID)
	if err != nil {
		return 0, 0, err
	}
	gid, err := strconv.Atoi(person.GID)
	return uid, gid, err
}

func (person Account) Own(file *os.File) error {
	uid, gid, err := person.IDs()
	if err != nil {
		return err
	}
	return file.Chown(uid, gid)
}

func (person Account) Path(rel string) string {
	return filepath.Join("/", person.Home, rel)
}

func (person Account) InGroup(group string) bool {
	output, err := run.CaptureQuiet(run.Quick, "id", "-nG", person.Name)
	return err == nil && slices.Contains(strings.Fields(output), group)
}

func (person Account) AddGroups(out io.Writer, groups ...string) error {
	for _, group := range groups {
		if run.Quiet(run.Quick, "getent", "group", group) != 0 || person.InGroup(group) {
			continue
		}
		if err := run.Logged(out, "usermod", "-aG", group, person.Name); err != nil {
			return err
		}
	}
	return nil
}

func (person Account) Run(out io.Writer, args ...string) error {
	return run.Logged(out, "runuser", person.RunuserArgs(args...)...)
}

func (person Account) RunWithin(limit time.Duration, out io.Writer, args ...string) error {
	return run.LoggedWithin(limit, out, "runuser", person.RunuserArgs(args...)...)
}

func (person Account) Runtime() string {
	return "/run/user/" + person.UID
}

func (person Account) SessionRunning() bool {
	info, err := os.Stat(person.Runtime() + "/bus")
	return err == nil && info.Mode()&os.ModeSocket != 0
}

func (person Account) RunuserArgs(args ...string) []string {
	command := []string{
		"-u", person.Name, "--", "env", "-C", person.Home,
		"HOME=" + person.Home,
		"USER=" + person.Name,
		"LOGNAME=" + person.Name,
		"SHELL=" + person.Shell,
		"PATH=" + person.Home + "/.local/bin:/usr/local/bin:/usr/bin:/bin",
		"LANG=" + env.Or("LANG", "C.UTF-8"),
	}
	if fsutil.IsDir(person.Runtime()) {
		command = append(command, "XDG_RUNTIME_DIR="+person.Runtime(), "DBUS_SESSION_BUS_ADDRESS=unix:path="+person.Runtime()+"/bus")
	}
	return append(command, args...)
}
