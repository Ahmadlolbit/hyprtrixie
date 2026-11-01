package runlog

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	fileMode = 0o644
	dirMode  = 0o755
	runStamp = "20060102-150405"
	canWrite = 2
)

type Log struct {
	file        string
	copy        string
	fileWarning sync.Once
	copyWarning sync.Once
}

func Joined() *Log {
	return &Log{file: os.Getenv("HYPRTRIXIE_LOG_FILE"), copy: os.Getenv("HYPRTRIXIE_USER_LOG")}
}

func Start(what string) (*Log, error) {
	dir := env.LogDir()
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, dirMode); err != nil {
		return nil, err
	}
	name := "run-" + time.Now().Format(runStamp) + ".log"
	log := &Log{file: filepath.Join(dir, name)}
	if err := add(log.file, os.O_CREATE, nil); err != nil {
		return nil, err
	}
	if err := atomic.Symlink(name, filepath.Join(dir, "latest.log")); err != nil {
		log.Line("warn", "cannot point latest.log at this run: "+err.Error())
	}
	log.Line("info", "hyprtrixie "+env.Version+" started ("+what+")")
	return log, nil
}

func (log *Log) Attach(home, owner string) error {
	dir := filepath.Join(home, "runbook-logs")
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a folder", dir)
	}
	earlier, err := os.ReadFile(log.file)
	if err != nil {
		return err
	}
	target := filepath.Join(dir, "runbook.log")
	if err := add(target, os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, append([]byte("\n"), earlier...)); err != nil {
		return err
	}
	log.copy = target
	run.Quiet(run.Quick, "chown", "-R", owner+":", dir)
	return nil
}

func (log *Log) Path() string {
	return log.file
}

func (log *Log) Copy() string {
	return log.copy
}

func writable(path string) bool {
	return syscall.Access(path, canWrite) == nil
}

func (log *Log) Writable() bool {
	return log.file != "" && writable(log.file) && (log.copy == "" || writable(log.copy))
}

func add(path string, how int, data []byte) error {
	if path == "" {
		return nil
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|how, fileMode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func warn(once *sync.Once, err error) {
	if err != nil {
		once.Do(func() { fmt.Fprintln(os.Stderr, "cannot write to the log:", err) })
	}
}

func (log *Log) Write(data []byte) (int, error) {
	warn(&log.fileWarning, add(log.file, os.O_CREATE, data))
	warn(&log.copyWarning, add(log.copy, syscall.O_NOFOLLOW|syscall.O_NONBLOCK, data))
	return len(data), nil
}

func (log *Log) Line(level, text string) {
	fmt.Fprintf(log, "%s [%-5s] %s\n", time.Now().Format(time.DateTime), level, text)
}

func (log *Log) Failed(text string) error {
	log.Line("error", text)
	return run.Reported(text)
}

func (log *Log) Size() int64 {
	info, err := os.Stat(log.file)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (log *Log) Since(offset int64, count int) []string {
	file, err := os.Open(log.file)
	if err != nil {
		return nil
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil
	}
	reader := bufio.NewReader(file)
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
		if len(lines) == 2*count {
			lines = append(lines[:0], lines[count:]...)
		}
		if err != nil {
			return lines[max(0, len(lines)-count):]
		}
	}
}
