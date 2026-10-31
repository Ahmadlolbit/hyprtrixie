package fsutil

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

func RealPath(path string) (string, error) {
	full, err := absolute(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(full)
	if !errors.Is(err, fs.ErrNotExist) {
		return resolved, err
	}
	full = strings.TrimRight(full, "/")
	cut := strings.LastIndexByte(full, '/') + 1
	parent, err := filepath.EvalSymlinks(full[:cut])
	if err != nil {
		return "", err
	}
	last := filepath.Join(parent, full[cut:])
	target, err := os.Readlink(last)
	if err != nil {
		return last, nil
	}
	if !filepath.IsAbs(target) {
		target = parent + "/" + target
	}
	return RealPath(target)
}

func absolute(path string) (string, error) {
	if path == "" {
		return "", &fs.PathError{Op: "lstat", Path: path, Err: syscall.ENOENT}
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return dir + "/" + path, nil
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func IsLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Text(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(bytes.ReplaceAll(data, []byte{0}, nil)), "\n"), nil
}

func Optional(path string) string {
	text, err := Text(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, err)
	}
	return text
}

func Entries(dir string) []fs.DirEntry {
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return entries
}

func Images(dirs ...string) []string {
	var found []string
	for _, dir := range dirs {
		found = append(found, imagesIn(dir)...)
	}
	if len(found) < 2 {
		return found
	}
	return run.Sorted(found)
}

func imagesIn(dir string) []string {
	var found []string
	for _, entry := range Entries(dir) {
		if entry.Type().IsRegular() && image(entry.Name()) {
			found = append(found, filepath.Join(dir, entry.Name()))
		}
	}
	return found
}

func image(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	}
	return false
}
