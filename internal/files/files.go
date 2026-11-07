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
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ledger"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

const (
	oldSuffix    = ".hyprtrixie-old"
	newSuffix    = ".hyprtrixie-new"
	plainMode    = 0o644
	programMode  = 0o755
	databaseMode = 0o600
	systemOwner  = "root"
	keptBits     = fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky
)

type Deployer struct {
	Log    *runlog.Log
	Person account.Account
	hashes map[string]string
	placed map[string]bool
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

func (deployer *Deployer) Forget(prefixes ...string) error {
	before := len(deployer.hashes)
	maps.DeleteFunc(deployer.hashes, func(path, _ string) bool {
		return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(path, prefix) })
	})
	if len(deployer.hashes) == before {
		return nil
	}
	return deployer.writeDatabase()
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
	if fsutil.IsLink(target) {
		return deployer.record(target, hash)
	}
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

func KeepCopy(from, to string) error {
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(from)
		if err != nil {
			return err
		}
		return atomic.Symlink(link, to)
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("cannot tell who owns " + from)
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return atomic.InstallAs(to, data, info.Mode()&keptBits, int(owner.Uid), int(owner.Gid))
}

func (deployer *Deployer) saveOld(target string) error {
	if err := KeepCopy(target, target+oldSuffix); err != nil {
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
	case known != "":
		return false, nil
	case fsutil.Exists(target+oldSuffix) || deployer.inHome(target):
		return true, deployer.keepEdits(target, data, mode, owner)
	}
	return false, deployer.saveOld(target)
}

func (deployer *Deployer) deploy(data []byte, target string, mode os.FileMode, owner string) (bool, error) {
	return deployer.put(data, target, mode, owner, true)
}

func (deployer *Deployer) put(data []byte, target string, mode os.FileMode, owner string, keepEdits bool) (bool, error) {
	ledger.Note(ledger.File, target)
	if fsutil.IsDir(target) {
		return false, fmt.Errorf("a folder is in the way of %s", target)
	}
	if keepEdits && fsutil.Exists(target) {
		if done, err := deployer.existing(target, data, mode, owner); done || err != nil {
			return false, err
		}
	}
	if err := place(target, data, mode, owner); err != nil {
		return false, err
	}
	if err := deployer.record(target, fingerprint(data)); err != nil {
		return true, err
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

func (deployer *Deployer) Changed(source, target string, pairs ...string) (bool, error) {
	data, err := deployer.read(source)
	if err != nil {
		return false, err
	}
	return deployer.Content([]byte(Render(string(data), pairs...)), target)
}

func (deployer *Deployer) File(source, target string, pairs ...string) error {
	_, err := deployer.Changed(source, target, pairs...)
	return err
}

func (deployer *Deployer) Program(source, target string) error {
	data, err := deployer.read(source)
	if err != nil {
		return err
	}
	_, err = deployer.deploy(data, target, programMode, systemOwner)
	return err
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

func (deployer *Deployer) inHome(path string) bool {
	home := deployer.Person.Home
	return home != "" && strings.HasPrefix(path, filepath.Clean(home)+"/")
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
	return deployer.ToolAt(name, filepath.Join(env.BinDir(), name))
}

func (deployer *Deployer) ToolAt(name, target string) error {
	source := filepath.Join(env.ToolsDir(), name)
	program, err := os.Readlink(source)
	if err == nil {
		if err := deployer.placeOnce(program); err != nil {
			return err
		}
		return deployer.link(filepath.Join(env.BinDir(), program), target)
	}
	if unchanged(source, target) {
		return deployer.Program(source, target)
	}
	data, err := deployer.read(source)
	if err != nil {
		return err
	}
	_, err = deployer.put(data, target, programMode, systemOwner, false)
	return err
}

func (deployer *Deployer) placeOnce(program string) error {
	if deployer.placed[program] {
		return nil
	}
	if err := deployer.Tool(program); err != nil {
		return err
	}
	if deployer.placed == nil {
		deployer.placed = map[string]bool{}
	}
	deployer.placed[program] = true
	return nil
}

func unchanged(source, target string) bool {
	info, err := os.Lstat(target)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	hash := current(source)
	return hash != "" && hash == current(target)
}

func (deployer *Deployer) link(program, target string) error {
	ledger.Note(ledger.File, target)
	if current, err := os.Readlink(target); err == nil && current == program {
		return nil
	}
	if fsutil.IsDir(target) {
		return fmt.Errorf("a folder is in the way of %s", target)
	}
	if err := atomic.Symlink(program, target); err != nil {
		return err
	}
	deployer.Log.Line("info", "deployed "+target+" as a link to "+program)
	return nil
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

func EnsureUserLine(log *runlog.Log, person account.Account, path, line string) error {
	list, err := fsutil.Lines(path)
	if err != nil || slices.Contains(list, line) {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		line = "\n" + line
	}
	return person.Append(log, path, line+"\n")
}

func EnsureLine(path, line string) error {
	list, err := fsutil.Lines(path)
	if err != nil || slices.Contains(list, line) {
		return err
	}
	return writeLines(path, append(list, line))
}

func (deployer *Deployer) forgetOne(path string) error {
	if _, known := deployer.hashes[path]; !known {
		return nil
	}
	delete(deployer.hashes, path)
	return deployer.writeDatabase()
}

func restoreOld(path string) (bool, error) {
	if !fsutil.Exists(path + oldSuffix) {
		return false, nil
	}
	return true, os.Rename(path+oldSuffix, path)
}

func (deployer *Deployer) Remove(paths ...string) error {
	before := len(deployer.hashes)
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		delete(deployer.hashes, path)
	}
	if len(deployer.hashes) == before {
		return nil
	}
	return deployer.writeDatabase()
}

func (deployer *Deployer) RemoveTree(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return deployer.Forget(strings.TrimSuffix(dir, "/") + "/")
}

func (deployer *Deployer) ours(path string) bool {
	if link, err := os.Readlink(path); err == nil {
		return filepath.Dir(link) == env.BinDir()
	}
	found := current(path)
	return found == "" || found == deployer.hashes[path]
}

func (deployer *Deployer) Undeploy(path string) (string, error) {
	if err := os.Remove(path + newSuffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if !deployer.ours(path) {
		return "kept", deployer.forgetOne(path)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	restored, err := restoreOld(path)
	if err != nil {
		return "", err
	}
	if restored {
		return "restored", deployer.forgetOne(path)
	}
	return "removed", deployer.forgetOne(path)
}
