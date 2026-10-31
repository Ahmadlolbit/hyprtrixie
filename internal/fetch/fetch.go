package fetch

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	folderMode    = 0o755
	stateFileMode = 0o644
	releaseFields = 4
	sourceFields  = 3
)

type Release struct {
	Name    string
	Version string
	URL     string
	SHA256  string
}

type Source struct {
	URL    string
	Commit string
}

func manifestLine(file, name string) []string {
	for _, line := range strings.Split(fsutil.Optional(filepath.Join(env.Manifests(), file)), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == name {
			return fields
		}
	}
	return nil
}

func HasRelease(name string) bool {
	return manifestLine("releases", name) != nil
}

func GetRelease(log *runlog.Log, name string) (Release, error) {
	fields := manifestLine("releases", name)
	if len(fields) < releaseFields {
		return Release{}, log.Failed("no pinned release named " + name)
	}
	return Release{fields[0], fields[1], fields[2], fields[3]}, nil
}

func GetSource(log *runlog.Log, name string) (Source, error) {
	fields := manifestLine("sources", name)
	if len(fields) < sourceFields {
		return Source{}, log.Failed("no pinned source named " + name)
	}
	return Source{fields[1], fields[2]}, nil
}

func sumOK(file, want string) bool {
	hash, err := files.Hash(file)
	return err == nil && hash == want
}

func download(log *runlog.Log, address, target string) error {
	return run.Logged(log, "curl", "-fL", "--retry", "3", "--retry-delay", "3", "--connect-timeout", "20", "-o", target, address)
}

func fileName(address string) string {
	return address[strings.LastIndex(address, "/")+1:]
}

func FetchRelease(log *runlog.Log, name string) (string, error) {
	release, err := GetRelease(log, name)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(env.CacheDir(), "downloads")
	file := filepath.Join(dir, release.Name+"-"+release.Version+"-"+fileName(release.URL))
	if err := os.MkdirAll(dir, folderMode); err != nil {
		return "", err
	}
	if sumOK(file, release.SHA256) {
		return file, nil
	}
	log.Line("info", "downloading "+release.URL)
	if err := download(log, release.URL, file+".part"); err != nil {
		return "", err
	}
	if !sumOK(file+".part", release.SHA256) {
		os.Remove(file + ".part")
		return "", log.Failed("checksum mismatch for " + release.Name + " " + release.Version)
	}
	return file, os.Rename(file+".part", file)
}

func marker(kind, name string) string {
	return filepath.Join(env.StateDir(), kind, name)
}

func marked(kind, name, want string) bool {
	return strings.TrimRight(fsutil.Optional(marker(kind, name)), "\n") == want
}

func mark(kind, name, value string) error {
	if err := os.MkdirAll(filepath.Dir(marker(kind, name)), folderMode); err != nil {
		return err
	}
	return atomic.Write(marker(kind, name), []byte(value+"\n"), stateFileMode)
}

func ReleaseCurrent(log *runlog.Log, name string) bool {
	release, err := GetRelease(log, name)
	return err == nil && marked("releases", name, release.Version)
}

func ReleaseMark(log *runlog.Log, name string) error {
	release, err := GetRelease(log, name)
	if err != nil {
		return err
	}
	return mark("releases", name, release.Version)
}

func SourceCurrent(log *runlog.Log, name string) bool {
	source, err := GetSource(log, name)
	return err == nil && marked("sources", name, source.Commit)
}

func SourceMark(log *runlog.Log, name string) error {
	source, err := GetSource(log, name)
	if err != nil {
		return err
	}
	return mark("sources", name, source.Commit)
}

func head(dir string) string {
	output, err := run.CaptureQuiet(run.Quick, "git", "-C", dir, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return output
}

func clone(log *runlog.Log, source Source, part, name string, paths []string) error {
	if err := run.Logged(log, "git", "init", "-q", part); err != nil {
		return err
	}
	if err := run.Logged(log, "git", "-C", part, "remote", "add", "origin", source.URL); err != nil {
		return err
	}
	if len(paths) > 0 {
		sparse := append([]string{"-C", part, "sparse-checkout", "set", "--no-cone"}, paths...)
		if err := run.Logged(log, "git", sparse...); err != nil {
			return err
		}
	}
	log.Line("info", "fetching "+name+" at "+source.Commit)
	if err := run.Logged(log, "git", "-C", part, "fetch", "-q", "--depth", "1", "--filter=blob:none", "origin", source.Commit); err != nil {
		return err
	}
	return run.Logged(log, "git", "-C", part, "checkout", "-q", "FETCH_HEAD")
}

func Git(log *runlog.Log, name, target string, paths ...string) error {
	source, err := GetSource(log, name)
	if err != nil {
		return err
	}
	if fsutil.IsDir(filepath.Join(target, ".git")) && head(target) == source.Commit {
		return nil
	}
	part := target + ".part"
	if err := os.RemoveAll(part); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), folderMode); err != nil {
		return err
	}
	if err := clone(log, source, part, name, paths); err != nil {
		return err
	}
	if head(part) != source.Commit {
		return log.Failed(name + " is not at the pinned commit")
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(part, target)
}

func GitCached(log *runlog.Log, name string, paths ...string) (string, error) {
	target := filepath.Join(env.CacheDir(), "sources", name)
	return target, Git(log, name, target, paths...)
}

func ReleaseExtract(log *runlog.Log, name, member, target string, mode os.FileMode) error {
	if ReleaseCurrent(log, name) && fsutil.Exists(target) {
		return nil
	}
	file, err := FetchRelease(log, name)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "hyprtrixie")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := run.Logged(log, "tar", "-xzf", file, "-C", dir, member); err != nil {
		return err
	}
	if err := run.Logged(log, "install", "-D", "-m", strconv.FormatUint(uint64(mode), 8), filepath.Join(dir, member), target); err != nil {
		return err
	}
	return ReleaseMark(log, name)
}
