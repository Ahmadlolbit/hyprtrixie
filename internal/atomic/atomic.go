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
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/cleanup"
)

const maxLinks = 40

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

func Symlink(target, link string) error {
	temp := temporary(link)
	defer cleanup.OnSignal(func() { os.Remove(temp) })()
	if err := os.Symlink(target, temp); err != nil {
		return err
	}
	if err := os.Rename(temp, link); err != nil {
		os.Remove(temp)
		return err
	}
	return syncDir(filepath.Dir(link))
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

func replace(path string, source io.Reader, perm os.FileMode, adjust func(*os.File) error) error {
	temp := temporary(path)
	defer cleanup.OnSignal(func() { os.Remove(temp) })()
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
