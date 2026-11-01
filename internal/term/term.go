package term

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"unsafe"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
)

const discardTyped = 0x540b

func attributes(file *os.File, request uintptr, state *syscall.Termios) error {
	_, _, failed := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), request, uintptr(unsafe.Pointer(state)))
	if failed != 0 {
		return failed
	}
	return nil
}

func IsTerminal(file *os.File) bool {
	var state syscall.Termios
	return attributes(file, syscall.TCGETS, &state) == nil
}

func Keep() func() {
	var saved syscall.Termios
	if attributes(os.Stdin, syscall.TCGETS, &saved) != nil {
		return func() {}
	}
	return func() { attributes(os.Stdin, syscall.TCSETS, &saved) }
}

func Raw() func() {
	restore, ok := adjust(func(state *syscall.Termios) {
		state.Lflag &^= syscall.ICANON | syscall.ECHO
		state.Cc[syscall.VMIN] = 1
		state.Cc[syscall.VTIME] = 0
	})
	if !ok {
		return func() {}
	}
	return restore
}

func noEcho() (func(), bool) {
	return adjust(func(state *syscall.Termios) { state.Lflag &^= syscall.ECHO })
}

func adjust(change func(*syscall.Termios)) (func(), bool) {
	var saved syscall.Termios
	if attributes(os.Stdin, syscall.TCGETS, &saved) != nil {
		return nil, false
	}
	allowQuit := cleanup.IgnoreQuit()
	restore := func() { attributes(os.Stdin, syscall.TCSETS, &saved) }
	stop := cleanup.OnSignal(restore)
	changed := saved
	change(&changed)
	attributes(os.Stdin, syscall.TCSETS, &changed)
	return func() {
		stop()
		restore()
		allowQuit()
	}, true
}

func ReadKey() {
	defer Raw()()
	key := make([]byte, 1)
	os.Stdin.Read(key)
}

func ReadSecret(prompt string) (string, error) {
	restore, terminal := noEcho()
	if terminal {
		defer restore()
		fmt.Fprint(os.Stderr, prompt)
	}
	line, err := readLine(os.Stdin)
	fmt.Fprintln(os.Stderr)
	return line, err
}

func ReadAnswer(prompt string) (string, error) {
	syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), discardTyped, syscall.TCIFLUSH)
	fmt.Print(prompt)
	return readLine(os.Stdin)
}

func readLine(input io.Reader) (string, error) {
	var line []byte
	key := make([]byte, 1)
	for {
		count, err := input.Read(key)
		if count == 1 && key[0] == '\n' {
			return string(line), nil
		}
		line = append(line, key[:count]...)
		if err != nil {
			return "", err
		}
	}
}
