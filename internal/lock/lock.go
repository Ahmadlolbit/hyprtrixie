package lock

import (
	"errors"
	"os"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

func Path() string {
	return env.Or("HYPRTRIXIE_LOCK", "/run/hyprtrixie.lock")
}

func Busy() bool {
	file, err := os.Open(Path())
	if err != nil {
		return false
	}
	defer file.Close()
	return errors.Is(syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB), syscall.EWOULDBLOCK)
}
