package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const partSuffix = ".hyprtrixie-part"

func (deployer *Deployer) inHome(path string) bool {
	home := deployer.Person.Home
	return home != "" && strings.HasPrefix(path, filepath.Clean(home)+"/")
}

func (deployer *Deployer) homeOwner(path string) (account.Account, bool, error) {
	if deployer.inHome(path) {
		return deployer.Person, true, nil
	}
	return account.Owner(path)
}

func (deployer *Deployer) userWrite(path string, data []byte, mode os.FileMode) error {
	person, part := deployer.Person, path+partSuffix
	err := person.Write(deployer.Log, part, string(data))
	for _, args := range [][]string{{"chmod", strconv.FormatUint(uint64(mode.Perm()), 8), part}, {"sync", part}, {"mv", "-fT", part, path}} {
		if err == nil {
			err = person.Run(deployer.Log, args...)
		}
	}
	if err != nil {
		run.Quiet(run.Quick, "runuser", person.RunuserArgs("rm", "-f", part)...)
	}
	return err
}

func (deployer *Deployer) settleHome(target string, data []byte, mode os.FileMode) error {
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || strconv.FormatUint(uint64(owner.Uid), 10) != deployer.Person.UID {
		return deployer.userWrite(target, data, mode)
	}
	if info.Mode().Perm() == mode.Perm() {
		return nil
	}
	return deployer.Person.Run(deployer.Log, "chmod", strconv.FormatUint(uint64(mode.Perm()), 8), target)
}

func (deployer *Deployer) remove(path string) error {
	person, ok, err := deployer.homeOwner(path)
	if err != nil {
		return err
	}
	if ok {
		return person.Run(deployer.Log, "rm", "-f", path)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (deployer *Deployer) restoreOld(path string) (bool, error) {
	if !fsutil.Exists(path + oldSuffix) {
		return false, nil
	}
	person, ok, err := deployer.homeOwner(path)
	if err != nil {
		return false, err
	}
	if ok {
		return true, person.Run(deployer.Log, "mv", "-fT", path+oldSuffix, path)
	}
	return true, os.Rename(path+oldSuffix, path)
}
