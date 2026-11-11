package runlog

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	fileMode    = 0o644
	dirMode     = 0o755
	privateFile = 0o600
	privateDir  = 0o700
	runStamp    = "20060102-150405"
	canWrite    = 2
	hiddenStamp = ".secrets-hidden"
)

var secrets = regexp.MustCompile(`(wifi-sec\.psk|802-11-wireless-security\.psk|wpa-psk|psk=)[^\n]*`)

func Redact(text string) string {
	return secrets.ReplaceAllString(text, "$1 (hidden)")
}

func hideIn(path string) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	data, err := os.ReadFile(path)
	if hidden := Redact(string(data)); err == nil && hidden != string(data) {
		atomic.Write(path, []byte(hidden), info.Mode().Perm())
	}
}

func hideOldSecrets(dir string) {
	stamp := filepath.Join(dir, hiddenStamp)
	if fsutil.Exists(stamp) {
		return
	}
	logs, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	for _, path := range logs {
		hideIn(path)
	}
	atomic.Write(stamp, nil, fileMode)
}

type Log struct {
	file        string
	copy        string
	copied      *os.File
	mirror      func([]byte)
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
	hideOldSecrets(dir)
	path, err := atomic.Fresh(dir, "run-"+time.Now().Format(runStamp), ".log", fileMode)
	if err != nil {
		return nil, err
	}
	log := &Log{file: path}
	if err := atomic.Symlink(filepath.Base(path), filepath.Join(dir, "latest.log")); err != nil {
		log.Line("warn", "cannot point latest.log at this run: "+err.Error())
	}
	log.Line("info", "hyprtrixie "+env.Version+" started ("+what+")")
	return log, nil
}

func private(path string, how int, mode os.FileMode) (*os.File, error) {
	file, err := os.OpenFile(path, how|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, mode)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func hideInCopy(file *os.File) error {
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	hidden := Redact(string(data))
	if hidden == string(data) {
		return nil
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	_, err = file.Write([]byte(hidden))
	return err
}

func openCopy(folder *os.File, name string) (*os.File, error) {
	how := os.O_RDWR | os.O_APPEND | os.O_CREATE | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	handle, err := syscall.Openat(int(folder.Fd()), name, how, privateFile)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), filepath.Join(folder.Name(), name))
	if err := file.Chmod(privateFile); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func startCopy(folder *os.File, earlier []byte) (*os.File, error) {
	file, err := openCopy(folder, "runbook.log")
	if err != nil {
		return nil, err
	}
	err = hideInCopy(file)
	if err == nil {
		_, err = file.Write(append([]byte("\n"), earlier...))
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func (log *Log) Attach(home, owner string) error {
	dir := filepath.Join(home, "runbook-logs")
	if err := os.MkdirAll(dir, privateDir); err != nil {
		return err
	}
	folder, err := private(dir, os.O_RDONLY|syscall.O_DIRECTORY, privateDir)
	if err != nil {
		return fmt.Errorf("%s is not a folder", dir)
	}
	defer folder.Close()
	earlier, err := os.ReadFile(log.file)
	if err != nil {
		return err
	}
	copied, err := startCopy(folder, earlier)
	if err != nil {
		return err
	}
	log.copy, log.copied = copied.Name(), copied
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

func (log *Log) toCopy(data []byte) error {
	if log.copied == nil {
		return add(log.copy, syscall.O_NOFOLLOW|syscall.O_NONBLOCK, data)
	}
	_, err := log.copied.Write(data)
	return err
}

func warn(once *sync.Once, err error) {
	if err != nil {
		once.Do(func() { fmt.Fprintln(os.Stderr, "cannot write to the log:", err) })
	}
}

func (log *Log) Mirror(to func([]byte)) {
	log.mirror = to
}

func (log *Log) Write(data []byte) (int, error) {
	warn(&log.fileWarning, add(log.file, os.O_CREATE, data))
	warn(&log.copyWarning, log.toCopy(data))
	if log.mirror != nil {
		log.mirror(data)
	}
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
