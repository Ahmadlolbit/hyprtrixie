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
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	secretMode = 0o600
	configMode = 0o644
	folderMode = 0o700
	pullLimit  = 30 * time.Minute
	stopLimit  = 2 * time.Minute
)

type Database struct {
	Name       string
	Title      string
	Image      string
	Port       string
	Data       string
	Variable   string
	Config     string
	ConfigPath string
	Command    []string
	Address    string
}

var All = []Database{
	{"postgres", "Postgres 17", "docker.io/library/postgres:17", "5432", "/var/lib/postgresql/data", "POSTGRES_PASSWORD", "", "", nil, "postgres://postgres:%s@localhost:5432/postgres"},
	{"mysql", "MySQL 8.4", "docker.io/library/mysql:8.4", "3306", "/var/lib/mysql", "MYSQL_ROOT_PASSWORD", "", "", nil, "mysql://root:%s@127.0.0.1:3306"},
	{"redis", "Redis 8", "docker.io/library/redis:8", "6379", "/data", "", "requirepass %s\nappendonly yes\n", "/usr/local/etc/redis/redis.conf",
		[]string{"redis-server", "/usr/local/etc/redis/redis.conf"}, "redis://:%s@localhost:6379"},
}

var (
	ErrNoEngine = errors.New("no container engine: tick Docker or Podman in the developer tools, then run the setup again")
	ErrNoAnswer = errors.New("the container engine isn't answering: start it, or log in again if you were just added to the docker group")
)

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
	if exists("volume", database.Container()) {
		return "", fmt.Errorf("%s has data but its password is gone: delete the data with hyprtrixie db remove %s", database.Title, database.Name)
	}
	if err := os.MkdirAll(folder(), folderMode); err != nil {
		return "", err
	}
	file, err := os.OpenFile(database.secret(""), os.O_WRONLY|os.O_CREATE|os.O_EXCL, secretMode)
	if errors.Is(err, fs.ErrExist) {
		if password, ok := database.saved(); ok {
			return password, nil
		}
		return database.fresh()
	}
	if err != nil {
		return "", err
	}
	password := rand.Text()
	_, err = file.WriteString(password + "\n")
	if err == nil {
		err = file.Sync()
	}
	return password, errors.Join(err, file.Close())
}

func (database Database) fresh() (string, error) {
	password := rand.Text()
	if err := atomic.Install(database.secret(""), []byte(password+"\n"), secretMode); err != nil {
		return "", err
	}
	return password, nil
}

func Engine() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return ErrNoEngine
	}
	if _, err := run.CaptureQuiet(run.Minute, "docker", "info", "--format", "{{.ServerVersion}}"); err != nil {
		return ErrNoAnswer
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
	if database.Config != "" {
		if err := atomic.Install(database.secret(".conf"), []byte(fmt.Sprintf(database.Config, password)), configMode); err != nil {
			return nil, err
		}
		args = append(args, "--volume", database.secret(".conf")+":"+database.ConfigPath+":ro")
	}
	return append(append(args, database.Image), database.Command...), nil
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
		return address, run.LoggedWithin(stopLimit, out, "docker", "start", database.Container())
	}
	args, err := database.runArgs(password)
	if err != nil {
		return "", err
	}
	return address, run.LoggedWithin(pullLimit, out, "docker", args...)
}

func (database Database) Stop(out io.Writer) error {
	if err := Engine(); err != nil {
		return err
	}
	if !exists("container", database.Container()) {
		return nil
	}
	if err := run.LoggedWithin(stopLimit, out, "docker", "stop", "--time", "30", database.Container()); err != nil {
		return err
	}
	return run.LoggedWithin(run.Minute, out, "docker", "rm", database.Container())
}

func (database Database) Remove(out io.Writer) error {
	if err := database.Stop(out); err != nil {
		return err
	}
	if exists("volume", database.Container()) {
		if err := run.LoggedWithin(run.Minute, out, "docker", "volume", "rm", database.Container()); err != nil {
			return err
		}
	}
	for _, suffix := range []string{"", ".env", ".conf"} {
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
