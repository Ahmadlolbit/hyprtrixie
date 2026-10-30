package run

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
)

const (
	Quick = 10 * time.Second
	Slow  = 5 * time.Minute

	grace       = 2 * time.Second
	executable  = 1
	bySignal    = 128
	notRunnable = 126
	notFound    = 127
)

func Exec(name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	if err := syscall.Exec(path, append([]string{name}, args...), os.Environ()); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func Status(name string, args ...string) int {
	command := exec.Command(name, args...)
	inherit(command)
	defer cleanup.IgnoreQuit()()
	interrupted := make(chan os.Signal, 1)
	cleanup.NotifyInterrupt(interrupted)
	err := command.Run()
	signal.Stop(interrupted)
	if len(interrupted) > 0 && signalOf(err) == syscall.SIGINT {
		cleanup.Raise(syscall.SIGINT)
	}
	return Report(err)
}

func Within(limit time.Duration, name string, args ...string) int {
	command, cancel := bounded(limit, name, args)
	defer cancel()
	inherit(command)
	return Report(command.Run())
}

func Quiet(limit time.Duration, name string, args ...string) int {
	command, cancel := bounded(limit, name, args)
	defer cancel()
	return exitCode(command.Run())
}

func Feed(limit time.Duration, input *os.File, name string, args ...string) int {
	command, cancel := bounded(limit, name, args)
	defer cancel()
	command.Stdin = input
	command.Stderr = os.Stderr
	return Report(command.Run())
}

func Output(name string, args ...string) (string, error) {
	return shown(exec.Command(name, args...))
}

func Filter(input, name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	command.Stdin = strings.NewReader(input)
	return shown(command)
}

func Capture(limit time.Duration, name string, args ...string) (string, error) {
	command, cancel := bounded(limit, name, args)
	defer cancel()
	return shown(command)
}

func CaptureQuiet(limit time.Duration, name string, args ...string) (string, error) {
	command, cancel := bounded(limit, name, args)
	defer cancel()
	return collect(command)
}

func shown(command *exec.Cmd) (string, error) {
	command.Stderr = os.Stderr
	output, err := collect(command)
	explain(err)
	return output, err
}

func collect(command *exec.Cmd) (string, error) {
	data, err := command.Output()
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	return strings.TrimRight(string(data), "\n"), err
}

func inherit(command *exec.Cmd) {
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
}

func bounded(limit time.Duration, name string, args []string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	command := exec.CommandContext(ctx, name, args...)
	command.Cancel = func() error {
		if command.Stderr == os.Stderr {
			fmt.Fprintf(os.Stderr, "%s took longer than %s\n", name, limit)
		}
		return command.Process.Signal(syscall.SIGTERM)
	}
	command.WaitDelay = grace
	return command, cancel
}

func Sibling(name string) string {
	self, err := os.Executable()
	if err != nil {
		return name
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	path := filepath.Join(filepath.Dir(self), name)
	if syscall.Access(path, executable) != nil {
		return name
	}
	return path
}

func Report(err error) int {
	explain(err)
	return exitCode(err)
}

func explain(err error) {
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) && !errors.Is(err, context.DeadlineExceeded) {
		fmt.Fprintln(os.Stderr, err)
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if caught := signalOf(err); caught != 0 {
			return bySignal + int(caught)
		}
		return exit.ExitCode()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return bySignal + int(syscall.SIGTERM)
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return notFound
	}
	return notRunnable
}

func signalOf(err error) syscall.Signal {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return status.Signal()
		}
	}
	return 0
}

func Sorted(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	command, cancel := bounded(Quick, "sort", []string{"-z"})
	defer cancel()
	command.Stdin = strings.NewReader(strings.Join(lines, "\x00") + "\x00")
	output, err := command.Output()
	if err != nil {
		sorted := slices.Clone(lines)
		slices.Sort(sorted)
		return sorted
	}
	return strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
}

func Lines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
