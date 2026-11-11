package sensors

import (
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	deviceEvents = syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO
	cameraEvents = syscall.IN_OPEN | syscall.IN_CLOSE_WRITE | syscall.IN_CLOSE_NOWRITE
)

type devices struct {
	descriptor int
	file       *os.File
	root       string
}

func endWithParent() {
	syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_PDEATHSIG, uintptr(syscall.SIGTERM), 0)
}

func watchDevices(root string) (changes, error) {
	descriptor, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	watch := &devices{descriptor: descriptor, file: os.NewFile(uintptr(descriptor), "inotify"), root: root}
	if _, err := syscall.InotifyAddWatch(descriptor, root+"/dev", deviceEvents); err != nil {
		watch.file.Close()
		return nil, err
	}
	watch.cameras()
	return watch, nil
}

func (watch *devices) cameras() {
	found, _ := filepath.Glob(watch.root + "/dev/video*")
	for _, node := range found {
		syscall.InotifyAddWatch(watch.descriptor, node, cameraEvents)
	}
}

func (watch *devices) read(limit time.Duration) bool {
	buffer := make([]byte, 4096)
	if watch.file.SetReadDeadline(time.Now().Add(limit)) != nil {
		time.Sleep(limit)
		return false
	}
	_, err := watch.file.Read(buffer)
	return err == nil
}

func (watch *devices) next() {
	if watch.read(rescan) {
		for watch.read(settle) {
		}
	}
	watch.cameras()
}
