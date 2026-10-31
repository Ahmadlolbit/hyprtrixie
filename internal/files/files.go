package files

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	oldSuffix    = ".hyprtrixie-old"
	newSuffix    = ".hyprtrixie-new"
	plainMode    = 0o644
	programMode  = 0o755
	databaseMode = 0o600
	systemOwner  = "root"
)

type Deployer struct {
	Log    *runlog.Log
	Person account.Account
	hashes map[string]string
}

func New(log *runlog.Log, person account.Account) (*Deployer, error) {
	hashes, err := readDatabase()
	if err != nil {
		return nil, err
	}
	return &Deployer{Log: log, Person: person, hashes: hashes}, nil
}

func databasePath() string {
	return filepath.Join(env.StateDir(), "files.db")
}

func readDatabase() (map[string]string, error) {
	lines, err := fsutil.Lines(databasePath())
	hashes := map[string]string{}
	for _, line := range lines {
		if path, hash, ok := strings.Cut(line, "\t"); ok {
			hashes[path] = hash
		}
	}
	return hashes, err
}

func (deployer *Deployer) writeDatabase() error {
	var text strings.Builder
	for _, known := range slices.Sorted(maps.Keys(deployer.hashes)) {
		text.WriteString(known + "\t" + deployer.hashes[known] + "\n")
	}
	if err := os.MkdirAll(env.StateDir(), programMode); err != nil {
		return err
	}
	return atomic.Write(databasePath(), []byte(text.String()), databaseMode)
}

func (deployer *Deployer) record(path, hash string) error {
	previous, known := deployer.hashes[path]
	if known && previous == hash {
		return nil
	}
	deployer.hashes[path] = hash
	err := deployer.writeDatabase()
	if err != nil && known {
		deployer.hashes[path] = previous
	} else if err != nil {
		delete(deployer.hashes, path)
	}
	return err
}

func fingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func Hash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func current(path string) string {
	if !fsutil.IsFile(path) {
		return ""
	}
	hash, _ := Hash(path)
	return hash
}

func ids(owner string) (int, int, error) {
	if owner == systemOwner {
		return 0, 0, nil
	}
	person, ok := account.Lookup(owner)
	if !ok {
		return 0, 0, fmt.Errorf("no account named %s", owner)
	}
	return person.IDs()
}

func install(path string, data []byte, mode os.FileMode, owner string) error {
	uid, gid, err := ids(owner)
	if err != nil {
		return err
	}
	return atomic.InstallAs(path, data, mode, uid, gid)
}

func place(path string, data []byte, mode os.FileMode, owner string) error {
	if err := os.MkdirAll(filepath.Dir(path), programMode); err != nil {
		return err
	}
	return install(path, data, mode, owner)
}

func (deployer *Deployer) settle(target, hash string, mode os.FileMode, owner string) error {
	uid, gid, err := ids(owner)
	if err != nil {
		return err
	}
	if err := os.Chmod(target, mode); err != nil {
		return err
	}
	if err := os.Chown(target, uid, gid); err != nil {
		return err
	}
	return deployer.record(target, hash)
}

func (deployer *Deployer) keepEdits(target string, data []byte, mode os.FileMode, owner string) error {
	if err := install(target+newSuffix, data, mode, owner); err != nil {
		return err
	}
	deployer.Log.Line("warn", "kept local edits in "+target+", new version written to "+target+newSuffix)
	return nil
}

func (deployer *Deployer) saveOld(target string) error {
	if err := run.Logged(deployer.Log, "cp", "-a", target, target+oldSuffix); err != nil {
		return err
	}
	deployer.Log.Line("info", "saved the previous "+target+" as "+target+oldSuffix)
	return nil
}

func (deployer *Deployer) existing(target string, data []byte, mode os.FileMode, owner string) (bool, error) {
	fresh := fingerprint(data)
	found := current(target)
	known := deployer.hashes[target]
	switch {
	case found == fresh:
		return true, deployer.settle(target, fresh, mode, owner)
	case known == fresh && found != known:
		return true, nil
	case known != "" && found != known:
		return true, deployer.keepEdits(target, data, mode, owner)
	case known == "" && !fsutil.Exists(target+oldSuffix):
		return false, deployer.saveOld(target)
	}
	return false, nil
}

func (deployer *Deployer) deploy(data []byte, target string, mode os.FileMode, owner string) (bool, error) {
	if fsutil.IsDir(target) {
		return false, fmt.Errorf("a folder is in the way of %s", target)
	}
	if fsutil.Exists(target) {
		if done, err := deployer.existing(target, data, mode, owner); done || err != nil {
			return false, err
		}
	}
	if err := place(target, data, mode, owner); err != nil {
		return false, err
	}
	if err := deployer.record(target, fingerprint(data)); err != nil {
		return false, err
	}
	deployer.Log.Line("info", "deployed "+target)
	return true, nil
}

func (deployer *Deployer) read(source string) ([]byte, error) {
	if !filepath.IsAbs(source) {
		source = filepath.Join(env.Root(), source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, deployer.Log.Failed("missing source file: " + source)
	}
	return data, nil
}

func (deployer *Deployer) Content(data []byte, target string) (bool, error) {
	return deployer.deploy(data, target, plainMode, systemOwner)
}

func (deployer *Deployer) File(source, target string, pairs ...string) (bool, error) {
	data, err := deployer.read(source)
	if err != nil {
		return false, err
	}
	return deployer.Content([]byte(Render(string(data), pairs...)), target)
}

func (deployer *Deployer) Program(source, target string) (bool, error) {
	data, err := deployer.read(source)
	if err != nil {
		return false, err
	}
	return deployer.deploy(data, target, programMode, systemOwner)
}

func regularFiles(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.Type().IsRegular() {
			found = append(found, path)
		}
		return err
	})
	slices.Sort(found)
	return found, err
}

func modeOf(path string) os.FileMode {
	info, err := os.Stat(path)
	if err == nil && info.Mode()&0o111 != 0 {
		return programMode
	}
	return plainMode
}

func (deployer *Deployer) Tree(source, target string) error {
	if !filepath.IsAbs(source) {
		source = filepath.Join(env.Root(), source)
	}
	if !fsutil.IsDir(source) {
		return errors.New(source + " is not a folder")
	}
	paths, err := regularFiles(source)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := deployer.treeFile(source, path, target); err != nil {
			return err
		}
	}
	return nil
}

func (deployer *Deployer) treeFile(source, path, target string) error {
	data, err := deployer.read(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(source, path)
	if err != nil {
		return err
	}
	_, err = deployer.deploy(data, filepath.Join(target, rel), modeOf(path), systemOwner)
	return err
}

func (deployer *Deployer) userDir(rel string) error {
	path := deployer.Person.Path(rel)
	if fsutil.IsDir(path) {
		return nil
	}
	return deployer.Person.Run(deployer.Log, "mkdir", "-p", path)
}

func (deployer *Deployer) userFile(source, rel string, mode os.FileMode) error {
	if err := deployer.userDir(filepath.Dir(rel)); err != nil {
		return err
	}
	data, err := deployer.read(source)
	if err != nil {
		return err
	}
	_, err = deployer.deploy(data, deployer.Person.Path(rel), mode, deployer.Person.Name)
	return err
}

func (deployer *Deployer) UserFile(source, rel string) error {
	return deployer.userFile(source, rel, plainMode)
}

func (deployer *Deployer) UserProgram(source, rel string) error {
	return deployer.userFile(source, rel, programMode)
}

func (deployer *Deployer) UserOnce(source, rel string) error {
	if err := deployer.userDir(filepath.Dir(rel)); err != nil {
		return err
	}
	target := deployer.Person.Path(rel)
	if fsutil.Exists(target) {
		return nil
	}
	data, err := deployer.read(source)
	if err != nil {
		return err
	}
	if err := place(target, data, plainMode, deployer.Person.Name); err != nil {
		return err
	}
	deployer.Log.Line("info", "created "+target)
	return nil
}

func Render(text string, pairs ...string) string {
	for _, pair := range pairs {
		key, value, _ := strings.Cut(pair, "=")
		text = strings.ReplaceAll(text, "{{ "+key+" }}", value)
	}
	return text
}

func (deployer *Deployer) Tool(name string) error {
	target := filepath.Join(env.BinDir(), name)
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	_, err := deployer.Program(filepath.Join(env.ToolsDir(), name), target)
	return err
}

func writeLines(path string, list []string) error {
	return atomic.Write(path, []byte(strings.Join(list, "\n")+"\n"), plainMode)
}

func setting(line, key string) bool {
	line = strings.TrimPrefix(line, "#")
	return strings.HasPrefix(strings.TrimLeft(line, " \t\r\f\v"), key+"=")
}

func ConfSet(path, key, value string) error {
	list, err := fsutil.Lines(path)
	if err != nil {
		return err
	}
	found := false
	for index, line := range list {
		if setting(line, key) {
			list[index] = key + "=" + value
			found = true
		}
	}
	if !found {
		list = append(list, key+"="+value)
	}
	return writeLines(path, list)
}

func EnsureLine(path, line string) error {
	list, err := fsutil.Lines(path)
	if err != nil || slices.Contains(list, line) {
		return err
	}
	return writeLines(path, append(list, line))
}
