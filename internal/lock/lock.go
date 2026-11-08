package lock

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

const (
	fileMode   = 0o600
	lockFields = 6
)

var heldLocks = "/proc/locks"

func Path() string {
	return env.Or("HYPRTRIXIE_LOCK", "/run/hyprtrixie.lock")
}

func open(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(fileMode); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func Take() (*os.File, error) {
	path := Path()
	file, err := open(path)
	if err != nil {
		return nil, errors.New("cannot create " + path)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("another Hyprtrixie run is still going")
	}
	return file, nil
}

func identity(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	device := uint64(stat.Dev)
	major := (device&0x00000000000fff00)>>8 | (device&0xfffff00000000000)>>32
	minor := device&0x00000000000000ff | (device&0x00000ffffff00000)>>12
	return fmt.Sprintf("%02x:%02x:%d", major, minor, stat.Ino)
}

func written(locks, id string) bool {
	for _, line := range strings.Split(locks, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= lockFields && fields[1] == "FLOCK" && fields[3] == "WRITE" && fields[5] == id {
			return true
		}
	}
	return false
}

func Busy() bool {
	info, err := os.Stat(Path())
	if err != nil {
		return false
	}
	locks, err := os.ReadFile(heldLocks)
	return err == nil && written(string(locks), identity(info))
}
