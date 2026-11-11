package apt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fetch"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ledger"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
)

var (
	dpkgJournal = "/var/lib/dpkg/updates"
	dpkgLock    = "/var/lib/dpkg/lock"
	aptLocks    = []string{"/var/lib/apt/lists/lock", "/var/cache/apt/archives/lock"}
	lockPause   = 5 * time.Second
	lockLimit   = 10 * time.Minute
)

const (
	backportsSuite = "trixie-backports"
	backportsMark  = "/backports"
	updateMaxAge   = time.Hour
	aptLimit       = 2 * time.Hour
	repairLimit    = 30 * time.Minute
	extrepoLimit   = 5 * time.Minute
	lockWait       = "DPkg::Lock::Timeout=600"
	attempts       = 5
	folderMode     = 0o755
	configMode     = 0o644
	madisonFields  = 3
)

func extrepoConfig() string {
	return env.Or("HYPRTRIXIE_EXTREPO_CONFIG", "/etc/extrepo/config.yaml")
}

func SourcesDir() string {
	return env.Or("HYPRTRIXIE_SOURCES_DIR", "/etc/apt/sources.list.d")
}

func stamp() string {
	return filepath.Join(env.StateDir(), "apt-updated")
}

func simulating() bool {
	return os.Getenv("APT_SIMULATE") == "1"
}

func held(path string) bool {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer file.Close()
	lock := syscall.Flock_t{Type: syscall.F_WRLCK}
	return syscall.FcntlFlock(file.Fd(), syscall.F_GETLK, &lock) == nil && lock.Type != syscall.F_UNLCK
}

func dpkgBusy() bool {
	return held(dpkgLock)
}

func waitForApt(out io.Writer) {
	for waited := time.Duration(0); !run.Stopped() && slices.ContainsFunc(aptLocks, held) && waited < lockLimit; waited += lockPause {
		if waited == 0 {
			fmt.Fprintln(out, "another apt is updating its lists or downloading, perhaps the daily update; waiting for it, up to 10 minutes")
		}
		time.Sleep(lockPause)
	}
}

func Interrupted() bool {
	for _, entry := range fsutil.Entries(dpkgJournal) {
		if strings.Trim(entry.Name(), "0123456789") == "" {
			return !dpkgBusy()
		}
	}
	return false
}

func repair(out io.Writer) error {
	if simulating() || !Interrupted() {
		return nil
	}
	fmt.Fprintln(out, "dpkg was interrupted, finishing what it left")
	return run.LoggedWithin(repairLimit, out, "dpkg", "--force-confdef", "--force-confold", "--configure", "-a")
}

func get(out io.Writer, args ...string) error {
	os.Setenv("DEBIAN_FRONTEND", "noninteractive")
	os.Setenv("NEEDRESTART_MODE", "a")
	os.Setenv("NEEDRESTART_SUSPEND", "1")
	os.Setenv("APT_LISTCHANGES_FRONTEND", "none")
	if err := repair(out); err != nil {
		return err
	}
	command := []string{"-y"}
	if simulating() {
		command = append(command, "-s")
	} else {
		waitForApt(out)
	}
	command = append(command, "-o", "Dpkg::Use-Pty=0", "-o", lockWait, "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold")
	return run.LoggedInEnglish(aptLimit, out, "apt-get", append(command, args...)...)
}

func fresh() bool {
	info, err := os.Stat(stamp())
	return err == nil && time.Since(info.ModTime()) < updateMaxAge
}

func Update(log *runlog.Log) error {
	if fresh() {
		return nil
	}
	return ForceUpdate(log)
}

func ForceUpdate(log *runlog.Log) error {
	if simulating() {
		return nil
	}
	if err := refresh(log); err != nil {
		return err
	}
	if err := noteUpdate(); err != nil {
		log.Line("warn", "cannot note the update: "+err.Error())
	}
	return nil
}

func noteUpdate() error {
	if err := os.MkdirAll(filepath.Dir(stamp()), folderMode); err != nil {
		return err
	}
	return atomic.Write(stamp(), nil, configMode)
}

func packageStates() (map[string]bool, map[string]bool, error) {
	output, err := run.CaptureQuiet(run.Slow, "dpkg-query", "-W", "-f", "${Package}\t${Status}\n")
	installed, held := map[string]bool{}, map[string]bool{}
	for _, line := range run.Lines(output) {
		name, status, _ := strings.Cut(line, "\t")
		words := strings.Fields(status)
		if len(words) != 3 {
			continue
		}
		if words[2] == "installed" {
			installed[name] = true
		}
		if words[0] == "hold" {
			held[name] = true
		}
	}
	return installed, held, err
}

func Installed() (map[string]bool, error) {
	installed, _, err := packageStates()
	return installed, err
}

func Version(name string) string {
	output, err := run.CaptureQuiet(run.Minute, "dpkg-query", "-W", "-f", "${Version}", name)
	if err != nil {
		return ""
	}
	return output
}

func backportsVersion(name string) string {
	output, _ := run.CaptureQuiet(run.Minute, "apt-cache", "madison", name)
	for _, line := range run.Lines(output) {
		fields := strings.Split(line, "|")
		if len(fields) >= madisonFields && strings.Contains(fields[2], backportsSuite) {
			return strings.ReplaceAll(fields[1], " ", "")
		}
	}
	return ""
}

func needsBackports(name string, installed map[string]bool) bool {
	if !installed[name] {
		return true
	}
	wanted := backportsVersion(name)
	if wanted == "" {
		return false
	}
	return run.Quiet(run.Quick, "dpkg", "--compare-versions", Version(name), "lt", wanted) == 0
}

func packageName(name string) bool {
	return name != "" && strings.Trim(name[:1], "abcdefghijklmnopqrstuvwxyz0123456789") == "" &&
		strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789.+-") == ""
}

func heldBack(line string) (string, bool) {
	name, rest, found := strings.Cut(strings.TrimLeft(line, " "), " : Depends: ")
	if !found || !packageName(name) {
		return "", false
	}
	needed, rest, found := strings.Cut(rest, " (= ")
	if !found || !packageName(needed) {
		return "", false
	}
	version, rest, found := strings.Cut(rest, ") but ")
	if !found || version == "" {
		return "", false
	}
	if strings.HasPrefix(rest, "it is not going to be installed") {
		return name, true
	}
	candidate, rest, found := strings.Cut(rest, " ")
	return name, found && strings.Contains(candidate, "~bpo") && strings.HasPrefix(rest, "is to be installed")
}

func siblings(output string) []string {
	var names []string
	for _, line := range strings.Split(output, "\n") {
		if name, ok := heldBack(line); ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return slices.DeleteFunc(slices.Compact(names), func(name string) bool { return backportsVersion(name) == "" })
}

func installStable(log *runlog.Log, specs []string) error {
	var extra []string
	var failure error
	for attempt := 1; attempt <= attempts; attempt++ {
		var output bytes.Buffer
		failure = get(io.MultiWriter(log, &output), append(append([]string{"install"}, specs...), extra...)...)
		if failure == nil {
			return nil
		}
		more := siblings(output.String())
		if len(more) == 0 {
			return failure
		}
		log.Line("warn", fmt.Sprintf("attempt %d: taking %s from %s to match libraries already installed from there", attempt, strings.Join(more, " "), backportsSuite))
		ledger.Note(ledger.Wants, more...)
		for _, name := range more {
			extra = append(extra, name+"/"+backportsSuite)
		}
	}
	return failure
}

func sorted(specs []string, installed map[string]bool) ([]string, []string) {
	var stable, backports []string
	for _, spec := range specs {
		name, wanted := strings.CutSuffix(spec, backportsMark)
		if wanted {
			if needsBackports(name, installed) {
				backports = append(backports, name)
			}
		} else if !installed[spec] {
			stable = append(stable, spec)
		}
	}
	return stable, backports
}

func unheld(log *runlog.Log, names []string, held map[string]bool) []string {
	return slices.DeleteFunc(names, func(name string) bool {
		if held[name] {
			log.Line("warn", name+" is held, so it stays as it is")
		}
		return held[name]
	})
}

func Install(log *runlog.Log, specs ...string) error {
	for _, spec := range specs {
		ledger.Note(ledger.Wants, strings.TrimSuffix(spec, backportsMark))
	}
	installed, held, err := packageStates()
	if err != nil {
		return err
	}
	stable, backports := sorted(specs, installed)
	stable, backports = unheld(log, stable, held), unheld(log, backports, held)
	if len(stable)+len(backports) == 0 {
		log.Line("info", "already installed: "+strings.Join(specs, " "))
		return nil
	}
	if err := Update(log); err != nil {
		return err
	}
	if len(stable) > 0 {
		log.Line("info", "installing: "+strings.Join(stable, " "))
		if err := installStable(log, stable); err != nil {
			return err
		}
	}
	if len(backports) == 0 {
		return nil
	}
	log.Line("info", "installing from "+backportsSuite+": "+strings.Join(backports, " "))
	return get(log, append([]string{"install", "-t", backportsSuite}, backports...)...)
}

func List(log *runlog.Log, list string) ([]string, error) {
	path := filepath.Join(env.Manifests(), "packages", list+".list")
	if !fsutil.IsFile(path) {
		return nil, log.Failed("unknown package list: " + list)
	}
	var found []string
	for _, line := range strings.Split(fsutil.Optional(path), "\n") {
		if strings.TrimSpace(line) != "" {
			found = append(found, line)
		}
	}
	return found, nil
}

func InstallLists(log *runlog.Log, lists ...string) error {
	var specs []string
	for _, list := range lists {
		more, err := List(log, list)
		if err != nil {
			return err
		}
		if len(more) == 0 {
			return errors.New("the package list " + list + " is empty")
		}
		specs = append(specs, more...)
	}
	return Install(log, specs...)
}

func Present(specs ...string) error {
	installed, err := Installed()
	if err != nil {
		return err
	}
	for _, spec := range specs {
		if name := strings.TrimSuffix(spec, backportsMark); !installed[name] {
			return errors.New(name + " is not installed")
		}
	}
	return nil
}

func ListsPresent(log *runlog.Log, lists ...string) error {
	for _, list := range lists {
		specs, err := List(log, list)
		if err != nil {
			return err
		}
		if err := Present(specs...); err != nil {
			return err
		}
	}
	return nil
}

func Commands(names ...string) error {
	var missing []string
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return errors.New("missing: " + strings.Join(missing, " "))
	}
	return nil
}

func sourceFile(name string) string {
	return filepath.Join(SourcesDir(), "extrepo_"+name+".sources")
}

func ExtrepoEnabled(name string) bool {
	text, err := fsutil.Text(sourceFile(name))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Enabled: no") {
			return false
		}
	}
	return true
}

func Fetch(log *runlog.Log, names ...string) error {
	if err := Update(log); err != nil {
		return err
	}
	for _, name := range names {
		if err := get(log, "install", "--download-only", "--reinstall", name); err != nil {
			return err
		}
	}
	return nil
}

func InstallCached(log *runlog.Log, names ...string) error {
	return get(log, append([]string{"install", "--no-download"}, names...)...)
}

func ExtrepoDisable(log *runlog.Log, name string) error {
	if err := run.LoggedWithin(extrepoLimit, log, "extrepo", "disable", name); err != nil {
		return err
	}
	if err := os.Remove(stamp()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func extrepoEnable(log *runlog.Log, name string) (bool, error) {
	if err := Install(log, "extrepo"); err != nil || ExtrepoEnabled(name) {
		return false, err
	}
	return true, run.LoggedWithin(extrepoLimit, log, "extrepo", "enable", name)
}

func ExtrepoAhead(log *runlog.Log, names ...string) {
	if len(names) == 0 || Install(log, "extrepo") != nil {
		return
	}
	added := false
	for _, name := range names {
		if !ExtrepoEnabled(name) && run.LoggedWithin(extrepoLimit, log, "extrepo", "enable", name) == nil {
			added = true
		}
	}
	if added {
		ForceUpdate(log)
	}
}

func ExtrepoAllows(policy string) (bool, error) {
	lines, err := fsutil.Lines(extrepoConfig())
	return slices.Contains(lines, "- "+policy), err
}

func ExtrepoPolicy(policy string) error {
	if !fsutil.IsFile(extrepoConfig()) {
		return errors.New("no " + extrepoConfig())
	}
	lines, err := fsutil.Lines(extrepoConfig())
	if err != nil {
		return err
	}
	wanted := "- " + policy
	if slices.Contains(lines, wanted) {
		return nil
	}
	changed := false
	for index, line := range lines {
		if rest, commented := strings.CutPrefix(line, "#"); commented && strings.TrimLeft(rest, " ") == wanted {
			lines[index] = wanted
			changed = true
		}
	}
	if !changed {
		lines = append(lines, wanted)
	}
	return atomic.Write(extrepoConfig(), []byte(strings.Join(lines, "\n")+"\n"), configMode)
}

func Bare(spec string) string {
	return strings.TrimSuffix(spec, backportsMark)
}

func Candidate(spec string) string {
	output, _ := run.CaptureInEnglish(run.Minute, "apt-cache", "policy", Bare(spec))
	for _, line := range run.Lines(output) {
		if rest, found := strings.CutPrefix(strings.TrimSpace(line), "Candidate:"); found {
			if version := strings.TrimSpace(rest); version != "(none)" {
				return version
			}
			return ""
		}
	}
	return ""
}

func field(text, name string) string {
	for _, line := range strings.Split(text, "\n") {
		if rest, found := strings.CutPrefix(line, name+":"); found {
			if fields := strings.Fields(rest); len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return ""
}

func releaseAddress(name string) (string, error) {
	text, err := fsutil.Text(sourceFile(name))
	if err != nil {
		return "", err
	}
	address := strings.TrimSuffix(field(text, "URIs"), "/")
	suite := field(text, "Suites")
	if strings.HasSuffix(suite, "/") {
		return address + "/" + strings.TrimPrefix(suite, "./") + "InRelease", nil
	}
	return address + "/dists/" + suite + "/InRelease", nil
}

func ExtrepoTry(log *runlog.Log, name string) error {
	enabled, err := extrepoEnable(log, name)
	if err != nil {
		return err
	}
	update := Update
	if enabled {
		update = ForceUpdate
	}
	failure := update(log)
	if failure == nil && !ExtrepoEnabled(name) {
		return errors.New("the " + name + " repository was turned off, because apt rejects it")
	}
	if failure == nil || errors.Is(failure, run.ErrStopped) {
		return failure
	}
	address, err := releaseAddress(name)
	if err != nil {
		return err
	}
	if run.Quiet(run.Minute, "curl", "-fsS", "--max-time", "20", "-o", "/dev/null", address) == 0 {
		return failure
	}
	log.Line("warn", "the "+name+" repository can't be reached from here, turning it off")
	run.LoggedWithin(extrepoLimit, log, "extrepo", "disable", name)
	if err := ForceUpdate(log); err != nil {
		return err
	}
	return errors.New("the " + name + " repository can't be reached")
}

func upstream(version string) string {
	if _, rest, found := strings.Cut(version, ":"); found {
		version = rest
	}
	if cut := strings.LastIndexByte(version, '-'); cut > 0 {
		version = version[:cut]
	}
	return version
}

func releaseInPlace(log *runlog.Log, packageName, pinned string) bool {
	if installed, _ := Installed(); !installed[packageName] {
		return false
	}
	version := Version(packageName)
	if run.Quiet(run.Quick, "dpkg", "--compare-versions", version, "lt", pinned) == 0 {
		return false
	}
	if upstream(version) == pinned {
		log.Line("info", packageName+" "+pinned+" already installed")
	} else {
		log.Line("info", packageName+" "+version+" is newer than "+pinned+", the version pinned here, so it stays as it is")
	}
	return true
}

func InstallRelease(log *runlog.Log, name, packageName string) error {
	ledger.Note(ledger.Wants, packageName)
	release, err := fetch.GetRelease(log, name)
	if err != nil {
		return err
	}
	if releaseInPlace(log, packageName, release.Version) {
		return nil
	}
	file, err := fetch.FetchRelease(log, name)
	if err != nil {
		return err
	}
	if err := Update(log); err != nil {
		return err
	}
	return get(log, "install", file)
}

func Unneeded(log *runlog.Log, names ...string) error {
	if len(names) == 0 {
		return nil
	}
	if err := mark(log, "auto", names); err != nil {
		return err
	}
	return get(log, "autoremove")
}
