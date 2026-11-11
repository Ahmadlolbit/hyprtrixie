package sensors

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var (
	ignoredHolders = []string{"pipewire", "wireplumber"}
	pollEvery      = 10 * time.Second
	rescan         = time.Minute
	settle         = 300 * time.Millisecond
)

type changes interface {
	next()
}

type poller struct{}

func (poller) next() {
	time.Sleep(pollEvery)
}

func numeric(name string) bool {
	return name != "" && strings.Trim(name, "0123456789") == ""
}

func holdsCamera(process string) bool {
	descriptors := filepath.Join(process, "fd")
	found, _ := os.ReadDir(descriptors)
	for _, descriptor := range found {
		if target, err := os.Readlink(filepath.Join(descriptors, descriptor.Name())); err == nil && strings.HasPrefix(target, "/dev/video") {
			return true
		}
	}
	return false
}

func Holders(root string) []string {
	var names []string
	for _, process := range entries(root+"/proc", "", "") {
		if !numeric(filepath.Base(process)) || !holdsCamera(process) {
			continue
		}
		name, _ := text(filepath.Join(process, "comm"))
		if name != "" && !slices.Contains(ignoredHolders, name) && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func Follow(root string, out io.Writer) error {
	endWithParent()
	watch, err := watchDevices(root)
	if err != nil {
		return follow(root, out, poller{})
	}
	return follow(root, out, watch)
}

func follow(root string, out io.Writer, events changes) error {
	shown := "\x00"
	for {
		if line := strings.Join(Holders(root), "\t"); line != shown {
			if _, err := fmt.Fprintln(out, line); err != nil {
				return err
			}
			shown = line
		}
		events.next()
	}
}
