package term

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
)

func attributes(request uintptr, state *syscall.Termios) error {
	_, _, failed := syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), request, uintptr(unsafe.Pointer(state)))
	if failed != 0 {
		return failed
	}
	return nil
}

func Raw() func() {
	var saved syscall.Termios
	if attributes(syscall.TCGETS, &saved) != nil {
		return func() {}
	}
	allowQuit := cleanup.IgnoreQuit()
	restore := func() { attributes(syscall.TCSETS, &saved) }
	stop := cleanup.OnSignal(restore)
	raw := saved
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	attributes(syscall.TCSETS, &raw)
	return func() {
		stop()
		restore()
		allowQuit()
	}
}

func ReadKey() {
	defer Raw()()
	key := make([]byte, 1)
	os.Stdin.Read(key)
}
