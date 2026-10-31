package runlog

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const fileMode = 0o644

type Log struct {
	file        string
	copy        string
	fileWarning sync.Once
	copyWarning sync.Once
}

func Joined() *Log {
	return &Log{file: os.Getenv("HYPRTRIXIE_LOG_FILE"), copy: os.Getenv("HYPRTRIXIE_USER_LOG")}
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
