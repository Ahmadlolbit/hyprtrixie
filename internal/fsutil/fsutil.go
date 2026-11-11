package fsutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	dfFields   = 6
	executable = 1
)

type Filesystem struct {
	Device string
	Mount  string
	Use    int
	Known  bool
}

func percent(text string) (int, bool) {
	digits, found := strings.CutSuffix(text, "%")
	if !found || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	use, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return use, true
}

func Filesystems(df string) []Filesystem {
	seen := map[string]bool{}
	var found []Filesystem
	for _, line := range strings.Split(df, "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < dfFields || seen[fields[0]] {
			continue
		}
		seen[fields[0]] = true
		use, known := percent(fields[4])
		found = append(found, Filesystem{fields[0], fields[5], use, known})
	}
	return found
}

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

func Runnable(path string) bool {
	return syscall.Access(path, executable) == nil
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

func plain(path string, info fs.FileInfo, owners []int) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s isn't a plain file, so it isn't read", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if len(owners) > 0 && (!ok || !slices.Contains(owners, int(stat.Uid))) {
		return fmt.Errorf("%s belongs to someone else, so it isn't read", path)
	}
	return nil
}

func ReadPlain(path string, limit int64, owners ...int) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, syscall.ELOOP) {
		return nil, fmt.Errorf("%s is a link, so it isn't read", path)
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err == nil {
		err = plain(path, info, owners)
	}
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = fmt.Errorf("%s is bigger than %d bytes, so it isn't read", path, limit)
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func Lines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil || len(data) == 0 {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"), nil
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

type writer func(path string, data []byte, perm os.FileMode) error

func CopyTree(source, target string) error {
	return copyTree(source, target, atomic.Write)
}

func StageTree(source, target string) error {
	return copyTree(source, target, atomic.Stage)
}

func CopyEntry(source, target string) error {
	return copyEntry(source, target, atomic.Write)
}

func copyTree(source, target string, write writer) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyEntry(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name()), write); err != nil {
			return err
		}
	}
	return nil
}

func copyEntry(source, target string, write writer) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return copyLink(source, target)
	case info.IsDir():
		return copyDir(source, target, info.Mode().Perm(), write)
	case info.Mode().IsRegular():
		return copyFile(source, target, info.Mode().Perm(), write)
	}
	return nil
}

func copyLink(source, target string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}
	return atomic.Symlink(link, target)
}

func copyDir(source, target string, perm os.FileMode, write writer) error {
	if err := os.MkdirAll(target, perm|0o700); err != nil {
		return err
	}
	return copyTree(source, target, write)
}

func copyFile(source, target string, perm os.FileMode, write writer) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return write(target, data, perm)
}
