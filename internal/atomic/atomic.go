package atomic

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
)

const (
	maxLinks   = 40
	maxCopies  = 100
	randomLong = 11
	staleAfter = time.Hour
)

var signalWait = 10 * time.Second

func Write(path string, data []byte, perm os.FileMode) error {
	target, err := follow(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	switch {
	case err == nil && !info.Mode().IsRegular():
		return os.WriteFile(target, data, 0)
	case err == nil:
		return replace(target, bytes.NewReader(data), perm, like(info))
	case errors.Is(err, fs.ErrNotExist):
		return replace(target, bytes.NewReader(data), perm, nil)
	}
	return err
}

func Install(path string, data []byte, perm os.FileMode) error {
	return replace(path, bytes.NewReader(data), perm, exactly(perm))
}

func InstallAs(path string, data []byte, perm os.FileMode, uid, gid int) error {
	return replace(path, bytes.NewReader(data), perm, func(file *os.File) error {
		if err := file.Chown(uid, gid); err != nil {
			return err
		}
		return file.Chmod(perm)
	})
}

func Stage(path string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closed := file.Close(); err == nil {
		err = closed
	}
	return err
}

func SyncFolders(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}
		return syncDir(path)
	})
}

func Symlink(target, link string) error {
	temp := temporary(link)
	defer guarded(temp)()
	if err := os.Symlink(target, temp); err != nil {
		return err
	}
	if err := os.Rename(temp, link); err != nil {
		os.Remove(temp)
		return err
	}
	sweep(link)
	return syncDir(filepath.Dir(link))
}

func Fresh(dir, name, extension string, perm os.FileMode) (string, error) {
	for number := 1; number <= maxCopies; number++ {
		path := filepath.Join(dir, numbered(name, number)+extension)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return path, file.Close()
	}
	return "", fmt.Errorf("%s: %d files with this name are there already", filepath.Join(dir, name+extension), maxCopies)
}

func numbered(name string, number int) string {
	if number == 1 {
		return name
	}
	return name + " (" + strconv.Itoa(number) + ")"
}

func follow(path string) (string, error) {
	for range maxLinks {
		link, err := os.Readlink(path)
		if err != nil {
			return path, nil
		}
		if !filepath.IsAbs(link) {
			dir, err := filepath.EvalSymlinks(filepath.Dir(path))
			if err != nil {
				return "", err
			}
			link = filepath.Join(dir, link)
		}
		path = link
	}
	return "", fmt.Errorf("%s: too many levels of symbolic links", path)
}

func like(info fs.FileInfo) func(*os.File) error {
	return func(file *os.File) error {
		if err := file.Chmod(info.Mode().Perm()); err != nil {
			return err
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !ok || os.Geteuid() != 0 {
			return nil
		}
		return file.Chown(int(owner.Uid), int(owner.Gid))
	}
}

func exactly(perm os.FileMode) func(*os.File) error {
	return func(file *os.File) error { return file.Chmod(perm) }
}

func temporary(path string) string {
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"."+strconv.FormatUint(rand.Uint64(), 36))
}

func leftOver(name, prefix string, info fs.FileInfo) bool {
	random, found := strings.CutPrefix(name, prefix)
	_, err := strconv.ParseUint(random, 36, 64)
	return found && err == nil && len(random) >= randomLong && random == strings.ToLower(random) &&
		!info.IsDir() && time.Since(info.ModTime()) > staleAfter
}

func sweep(path string) {
	dir, prefix := filepath.Dir(path), "."+filepath.Base(path)+"."
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		if info, err := entry.Info(); err == nil && leftOver(entry.Name(), prefix, info) {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func guarded(temp string) func() {
	finished := make(chan struct{})
	stop := cleanup.OnSignal(func() {
		select {
		case <-finished:
		case <-time.After(signalWait):
		}
		os.Remove(temp)
	})
	return func() {
		close(finished)
		stop()
	}
}

func replace(path string, source io.Reader, perm os.FileMode, adjust func(*os.File) error) error {
	temp := temporary(path)
	defer guarded(temp)()
	file, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	err = fill(file, source, adjust)
	if closed := file.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Rename(temp, path)
	}
	if err != nil {
		os.Remove(temp)
		return err
	}
	sweep(path)
	return syncDir(filepath.Dir(path))
}

func fill(file *os.File, source io.Reader, adjust func(*os.File) error) error {
	if _, err := io.Copy(file, source); err != nil {
		return err
	}
	if adjust != nil {
		if err := adjust(file); err != nil {
			return err
		}
	}
	return file.Sync()
}

func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer handle.Close()
	if err := handle.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) {
		return err
	}
	return nil
}
