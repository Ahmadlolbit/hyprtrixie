package databases

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	secretMode = 0o600
	folderMode = 0o700
)

type Database struct {
	Name     string
	Title    string
	Image    string
	Port     string
	Data     string
	Variable string
	Command  []string
	Address  string
}

var All = []Database{
	{"postgres", "Postgres 17", "docker.io/library/postgres:17", "5432", "/var/lib/postgresql/data", "POSTGRES_PASSWORD", nil, "postgres://postgres:%s@localhost:5432/postgres"},
	{"mysql", "MySQL 8.4", "docker.io/library/mysql:8.4", "3306", "/var/lib/mysql", "MYSQL_ROOT_PASSWORD", nil, "mysql://root:%s@127.0.0.1:3306"},
	{"redis", "Redis 8", "docker.io/library/redis:8", "6379", "/data", "", []string{"redis-server", "--requirepass", "%s", "--appendonly", "yes"}, "redis://:%s@localhost:6379"},
}

var ErrNoEngine = errors.New("no container engine: tick Docker or Podman in the developer tools, then run the setup again")

func Find(name string) (Database, bool) {
	for _, database := range All {
		if database.Name == name {
			return database, true
		}
	}
	return Database{}, false
}

func (database Database) Container() string {
	return "hyprtrixie-" + database.Name
}

func folder() string {
	return filepath.Join(env.StateHome(), "hyprtrixie", "databases")
}

func (database Database) secret(suffix string) string {
	return filepath.Join(folder(), database.Name+suffix)
}

func (database Database) saved() (string, bool) {
	data, err := os.ReadFile(database.secret(""))
	password := strings.TrimSpace(string(data))
	return password, err == nil && password != ""
}

func (database Database) Password() (string, error) {
	if password, ok := database.saved(); ok {
		return password, nil
	}
	if err := os.MkdirAll(folder(), folderMode); err != nil {
		return "", err
	}
	password := rand.Text()
	return password, atomic.Install(database.secret(""), []byte(password+"\n"), secretMode)
}

func Engine() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return ErrNoEngine
	}
	return nil
}

func exists(kind, name string) bool {
	return run.Quiet(run.Minute, "docker", kind, "inspect", name) == 0
}

func (database Database) Running() bool {
	state, err := run.CaptureQuiet(run.Minute, "docker", "container", "inspect", "--format", "{{.State.Running}}", database.Container())
	return err == nil && strings.TrimSpace(state) == "true"
}

func (database Database) runArgs(password string) ([]string, error) {
	args := []string{"run", "--detach", "--name", database.Container(),
		"--publish", "127.0.0.1:" + database.Port + ":" + database.Port, "--volume", database.Container() + ":" + database.Data}
	if database.Variable != "" {
		if err := atomic.Install(database.secret(".env"), []byte(database.Variable+"="+password+"\n"), secretMode); err != nil {
			return nil, err
		}
		args = append(args, "--env-file", database.secret(".env"))
	}
	args = append(args, database.Image)
	for _, word := range database.Command {
		args = append(args, strings.ReplaceAll(word, "%s", password))
	}
	return args, nil
}

func (database Database) Start(out io.Writer) (string, error) {
	if err := Engine(); err != nil {
		return "", err
	}
	password, err := database.Password()
	if err != nil {
		return "", err
	}
	address := fmt.Sprintf(database.Address, password)
	if exists("container", database.Container()) {
		return address, run.Logged(out, "docker", "start", database.Container())
	}
	args, err := database.runArgs(password)
	if err != nil {
		return "", err
	}
	return address, run.Logged(out, "docker", args...)
}

func (database Database) Stop(out io.Writer) error {
	if err := Engine(); err != nil {
		return err
	}
	if !exists("container", database.Container()) {
		return nil
	}
	return run.Logged(out, "docker", "rm", "--force", database.Container())
}

func (database Database) Remove(out io.Writer) error {
	if err := database.Stop(out); err != nil {
		return err
	}
	if exists("volume", database.Container()) {
		if err := run.Logged(out, "docker", "volume", "rm", database.Container()); err != nil {
			return err
		}
	}
	for _, suffix := range []string{"", ".env"} {
		if err := os.Remove(database.secret(suffix)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func List() string {
	var text strings.Builder
	for _, database := range All {
		state := "stopped"
		if database.Running() {
			state = "running"
		}
		fmt.Fprintf(&text, "%-9s %-12s %s", database.Name, database.Title, state)
		if password, ok := database.saved(); ok && state == "running" {
			fmt.Fprintf(&text, "  %s", fmt.Sprintf(database.Address, password))
		}
		text.WriteString("\n")
	}
	return text.String()
}
