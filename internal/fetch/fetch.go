package fetch

import (
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	folderMode     = 0o755
	stateFileMode  = 0o644
	executableBits = 0o111
	releaseFields  = 4
	sourceFields   = 3
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

type Person interface {
	RunWithin(limit time.Duration, out io.Writer, args ...string) error
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
	return run.Logged(log, "curl", "-fL", "--retry", "3", "--retry-delay", "3", "--connect-timeout", "20", "--speed-limit", "1024", "--speed-time", "60", "-o", target, address)
}

func fileName(address string) string {
	return address[strings.LastIndex(address, "/")+1:]
}

func downloads() string {
	return filepath.Join(env.CacheDir(), "downloads")
}

func keptIndex() string {
	return filepath.Join(downloads(), "kept")
}

func readKept() map[string]string {
	kept := map[string]string{}
	for _, line := range strings.Split(fsutil.Optional(keptIndex()), "\n") {
		if name, file, found := strings.Cut(line, "\t"); found && file == filepath.Base(file) && !strings.HasPrefix(file, ".") {
			kept[name] = file
		}
	}
	return kept
}

func writeKept(kept map[string]string) error {
	var text strings.Builder
	for _, name := range slices.Sorted(maps.Keys(kept)) {
		text.WriteString(name + "\t" + kept[name] + "\n")
	}
	return atomic.Write(keptIndex(), []byte(text.String()), stateFileMode)
}

func keepOnly(log *runlog.Log, name, file string) {
	kept := readKept()
	older := kept[name]
	if older == file {
		return
	}
	if older != "" {
		if err := os.Remove(filepath.Join(downloads(), older)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Line("warn", "cannot remove the older download "+older+": "+err.Error())
		}
	}
	kept[name] = file
	if err := writeKept(kept); err != nil {
		log.Line("warn", "cannot note the download "+file+": "+err.Error())
	}
}

func claimed() map[string]bool {
	files := map[string]bool{filepath.Base(keptIndex()): true}
	for _, line := range strings.Split(fsutil.Optional(filepath.Join(env.Manifests(), "releases")), "\n") {
		if fields := strings.Fields(line); len(fields) >= releaseFields {
			files[fields[0]+"-"+fields[1]+"-"+fileName(fields[2])] = true
		}
	}
	for _, file := range readKept() {
		files[file] = true
	}
	return files
}

func tidy(log *runlog.Log) {
	files := claimed()
	for _, entry := range fsutil.Entries(downloads()) {
		name := entry.Name()
		if !entry.Type().IsRegular() || files[name] || strings.HasSuffix(name, ".part") {
			continue
		}
		if err := os.Remove(filepath.Join(downloads(), name)); err != nil {
			log.Line("warn", "cannot remove the old download "+name+": "+err.Error())
		}
	}
}

func fetchChecked(log *runlog.Log, release Release, file string) error {
	log.Line("info", "downloading "+release.URL)
	if err := download(log, release.URL, file+".part"); err != nil {
		return err
	}
	if !sumOK(file+".part", release.SHA256) {
		os.Remove(file + ".part")
		return log.Failed("checksum mismatch for " + release.Name + " " + release.Version)
	}
	return os.Rename(file+".part", file)
}

func FetchRelease(log *runlog.Log, name string) (string, error) {
	release, err := GetRelease(log, name)
	if err != nil {
		return "", err
	}
	file := filepath.Join(downloads(), release.Name+"-"+release.Version+"-"+fileName(release.URL))
	if err := os.MkdirAll(downloads(), folderMode); err != nil {
		return "", err
	}
	tidy(log)
	if !sumOK(file, release.SHA256) {
		if err := fetchChecked(log, release, file); err != nil {
			return "", err
		}
	}
	keepOnly(log, release.Name, filepath.Base(file))
	return file, nil
}

func ReleaseShared(log *runlog.Log, name string) (string, error) {
	file, err := FetchRelease(log, name)
	if err != nil {
		return "", err
	}
	if err := run.LoggedWithin(run.Quick, log, "chmod", "a+x", env.CacheDir(), downloads()); err != nil {
		return "", err
	}
	return file, run.LoggedWithin(run.Quick, log, "chmod", "a+r", file)
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

func Released(name string) bool {
	return fsutil.Exists(marker("releases", name))
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
	stalled := []string{"-c", "http.lowSpeedLimit=1024", "-c", "http.lowSpeedTime=60", "-C", part}
	if err := run.Logged(log, "git", append(stalled, "fetch", "-q", "--depth", "1", "--filter=blob:none", "origin", source.Commit)...); err != nil {
		return err
	}
	return run.Logged(log, "git", append(stalled, "checkout", "-q", "FETCH_HEAD")...)
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

func GitShared(log *runlog.Log, name string) (string, error) {
	cached, err := GitCached(log, name)
	if err != nil {
		return "", err
	}
	if err := run.LoggedWithin(run.Quick, log, "chmod", "a+x", env.CacheDir(), filepath.Dir(cached)); err != nil {
		return "", err
	}
	return cached, run.LoggedWithin(run.Minute, log, "chmod", "-R", "a+rX", cached)
}

func GitCopy(log *runlog.Log, name, target string, person Person) error {
	source, err := GetSource(log, name)
	if err != nil {
		return err
	}
	var current strings.Builder
	if person.RunWithin(run.Quick, &current, "git", "-C", target, "rev-parse", "HEAD") == nil && strings.TrimSpace(current.String()) == source.Commit {
		return nil
	}
	cached, err := GitShared(log, name)
	if err != nil {
		return err
	}
	part := target + ".part"
	for _, args := range [][]string{{"rm", "-rf", part}, {"cp", "-R", cached, part}, {"rm", "-rf", target}, {"mv", "-T", part, target}} {
		if err := person.RunWithin(run.Slow, log, args...); err != nil {
			return err
		}
	}
	return nil
}

func inPlace(target string, mode os.FileMode) bool {
	return fsutil.Exists(target) && (mode&executableBits == 0 || fsutil.Runnable(target))
}

func ReleaseExtract(log *runlog.Log, name, member, target string, mode os.FileMode) error {
	if ReleaseCurrent(log, name) && inPlace(target, mode) {
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
