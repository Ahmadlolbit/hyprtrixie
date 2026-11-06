package lock

import (
	"errors"
	"os"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

const fileMode = 0o644

func Path() string {
	return env.Or("HYPRTRIXIE_LOCK", "/run/hyprtrixie.lock")
}

func Take() (*os.File, error) {
	path := Path()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return nil, errors.New("cannot create " + path)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("another Hyprtrixie run is still going")
	}
	return file, nil
}

func Busy() bool {
	file, err := os.Open(Path())
	if err != nil {
		return false
	}
	defer file.Close()
	return errors.Is(syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB), syscall.EWOULDBLOCK)
}
