package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
)

const (
	Quick  = 10 * time.Second
	Minute = time.Minute
	Slow   = 5 * time.Minute

	grace       = 3 * time.Second
	executable  = 1
	bySignal    = 128
	notRunnable = 126
	notFound    = 127
)

var (
	ErrStopped = errors.New("stopped before it started")
	stopped    atomic.Bool
)

func Exec(name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	if err := syscall.Exec(path, append([]string{name}, args...), os.Environ()); err != nil {
		return &fs.PathError{Op: "fork/exec", Path: name, Err: err}
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
	command, finish := bounded(limit, os.Stderr, name, args)
	inherit(command)
	return Report(finish(command.Run()))
}

func Quiet(limit time.Duration, name string, args ...string) int {
	command, finish := bounded(limit, nil, name, args)
	return Code(finish(command.Run()))
}

func Feed(limit time.Duration, input *os.File, name string, args ...string) int {
	command, finish := bounded(limit, os.Stderr, name, args)
	command.Stdin = input
	command.Stderr = os.Stderr
	return Report(finish(command.Run()))
}

func Into(limit time.Duration, output *os.File, name string, args ...string) int {
	command, finish := bounded(limit, os.Stderr, name, args)
	command.Stdout = output
	command.Stderr = os.Stderr
	return Report(finish(command.Run()))
}

func Record(limit time.Duration, output *os.File, name string, args ...string) int {
	command, finish := bounded(limit, output, name, args)
	command.Stdout = output
	command.Stderr = output
	err := finish(command.Run())
	Explain(output, err)
	if caught := signalOf(err); caught != 0 && caught != syscall.SIGTERM {
		fmt.Fprintf(output, "%s: %v\n", name, caught)
	}
	return Code(err)
}

func Stop() {
	stopped.Store(true)
}

func Logged(output io.Writer, name string, args ...string) error {
	if stopped.Load() {
		return ErrStopped
	}
	command := exec.Command(name, args...)
	command.Stdout = output
	command.Stderr = output
	command.WaitDelay = grace
	err := done(command.Run())
	if caught := signalOf(err); caught != 0 {
		fmt.Fprintf(output, "%s: %v\n", name, caught)
	}
	return err
}

func Page(text string) {
	command := exec.Command("less")
	command.Stdin = strings.NewReader(text)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if command.Run() != nil {
		fmt.Println(text)
	}
}

func Output(name string, args ...string) (string, error) {
	return shown(exec.Command(name, args...), os.Stderr)
}

func Filter(input, name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	command.Stdin = strings.NewReader(input)
	return shown(command, os.Stderr)
}

func Capture(limit time.Duration, name string, args ...string) (string, error) {
	return CaptureTo(limit, os.Stderr, name, args...)
}

func CaptureTo(limit time.Duration, messages io.Writer, name string, args ...string) (string, error) {
	command, finish := bounded(limit, messages, name, args)
	output, err := shown(command, messages)
	return output, finish(err)
}

func CaptureQuiet(limit time.Duration, name string, args ...string) (string, error) {
	command, finish := bounded(limit, nil, name, args)
	output, err := collect(command)
	return output, finish(err)
}

func TimedOut(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

func shown(command *exec.Cmd, messages io.Writer) (string, error) {
	command.Stderr = messages
	output, err := collect(command)
	Explain(messages, err)
	return output, err
}

func done(err error) error {
	if errors.Is(err, exec.ErrWaitDelay) {
		return nil
	}
	return err
}

func collect(command *exec.Cmd) (string, error) {
	data, err := command.Output()
	return strings.TrimRight(string(data), "\n"), done(err)
}

func inherit(command *exec.Cmd) {
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
}

func bounded(limit time.Duration, messages io.Writer, name string, args []string) (*exec.Cmd, func(error) error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	command := exec.CommandContext(ctx, name, args...)
	command.Cancel = func() error {
		if messages != nil {
			fmt.Fprintf(messages, "%s took longer than %s\n", name, limit)
		}
		return command.Process.Signal(syscall.SIGTERM)
	}
	command.WaitDelay = grace
	return command, func(err error) error {
		defer cancel()
		if err != nil && ctx.Err() != nil {
			return fmt.Errorf("%w: %w", context.DeadlineExceeded, err)
		}
		return err
	}
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
	Explain(os.Stderr, err)
	return Code(err)
}

type Reported string

func (text Reported) Error() string {
	return string(text)
}

func Explain(output io.Writer, err error) {
	var exit *exec.ExitError
	var reported Reported
	if err != nil && !errors.As(err, &exit) && !errors.Is(err, context.DeadlineExceeded) && !errors.As(err, &reported) {
		fmt.Fprintln(output, err)
	}
}

func Code(err error) int {
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
	if !couldNotStart(err) {
		return 1
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return notFound
	}
	return notRunnable
}

func couldNotStart(err error) bool {
	var lookup *exec.Error
	var start *fs.PathError
	return errors.As(err, &lookup) || (errors.As(err, &start) && start.Op == "fork/exec")
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
	command, finish := bounded(Quick, nil, "sort", []string{"-z"})
	defer finish(nil)
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
