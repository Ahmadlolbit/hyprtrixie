package privilege

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	sudo     = "sudo"
	notFound = 127
)

var (
	ErrWrongPassword = errors.New("that password didn't work")
	ErrNoSudo        = errors.New("this account can't use sudo")

	isRoot = func() bool { return os.Geteuid() == 0 }
)

func Ready() bool {
	return isRoot() || run.Quiet(run.Quick, sudo, "-n", "true") == 0
}

func Unlock(password string) error {
	var said strings.Builder
	err := run.Supply(run.Minute, &said, password+"\n", "env", "LC_ALL=C", sudo, "-S", "-p", "", "-v")
	switch {
	case err == nil:
		return nil
	case strings.Contains(said.String(), "may not run sudo"), run.Code(err) == notFound:
		return ErrNoSudo
	}
	return ErrWrongPassword
}

func Spawn(environment []string, name string, args ...string) (*run.Child, error) {
	if isRoot() {
		return run.Spawn(environment, name, args...)
	}
	words := append([]string{"-n", "env"}, environment...)
	return run.Spawn(nil, sudo, append(append(words, name), args...)...)
}

func Save(text, target string) error {
	words := []string{"install", "-D", "-m", "0644", "/dev/stdin", target}
	if !isRoot() {
		words = append([]string{sudo, "-n"}, words...)
	}
	if run.Supply(run.Minute, io.Discard, text, words[0], words[1:]...) != nil {
		return errors.New("cannot write " + target)
	}
	return nil
}
