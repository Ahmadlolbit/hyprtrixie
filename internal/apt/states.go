package apt

import (
	"maps"
	"os"
	"strconv"
	"sync"
)

const dpkgStatus = "/var/lib/dpkg/status"

var remembered struct {
	sync.Mutex
	users     int
	stamp     string
	installed map[string]bool
	held      map[string]bool
}

func Remember() func() {
	remembered.Lock()
	remembered.users++
	remembered.Unlock()
	return func() {
		remembered.Lock()
		defer remembered.Unlock()
		remembered.users--
		if remembered.users == 0 {
			remembered.stamp, remembered.installed, remembered.held = "", nil, nil
		}
	}
}

func forgetStates() {
	remembered.Lock()
	remembered.stamp = ""
	remembered.Unlock()
}

func dpkgStamp() string {
	stamp := "dpkg"
	for _, path := range []string{dpkgStatus, dpkgJournal} {
		if info, err := os.Stat(path); err == nil {
			stamp += " " + strconv.FormatInt(info.ModTime().UnixNano(), 10) + " " + strconv.FormatInt(info.Size(), 10)
		}
	}
	return stamp
}

func keptStates(stamp string) (map[string]bool, map[string]bool, bool) {
	remembered.Lock()
	defer remembered.Unlock()
	if remembered.users == 0 || remembered.stamp != stamp {
		return nil, nil, false
	}
	return maps.Clone(remembered.installed), maps.Clone(remembered.held), true
}

func keepStates(stamp string, installed, held map[string]bool) {
	remembered.Lock()
	defer remembered.Unlock()
	if remembered.users > 0 {
		remembered.stamp, remembered.installed, remembered.held = stamp, maps.Clone(installed), maps.Clone(held)
	}
}
