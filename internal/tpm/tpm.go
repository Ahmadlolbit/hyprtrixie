package tpm

import (
	"path/filepath"
	"slices"
	"strings"
)

const (
	Option = "tpm2-device=auto"
	before = "0-before-tpm-"
)

type Device struct {
	Mapper string
	Device string
}

type Image struct {
	Listing  string
	Crypttab string
	Cmdline  string
}

func Before(kernel string) string {
	return before + kernel
}

func LuksDevices(listing string) []Device {
	lines := strings.Split(strings.TrimSpace(listing), "\n")
	var found []Device
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != "crypt" || index+1 == len(lines) {
			continue
		}
		if parent := strings.Fields(lines[index+1]); len(parent) == 2 {
			found = append(found, Device{filepath.Base(fields[0]), parent[0]})
		}
	}
	return found
}

func PoolDevices(show string) int {
	count := 0
	for _, line := range strings.Split(show, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == "devid" {
			count++
		}
	}
	return count
}

func withOption(options string, on bool) string {
	kept := slices.DeleteFunc(strings.Split(options, ","), func(option string) bool { return option == Option || option == "" })
	if on {
		kept = append(kept, Option)
	}
	return strings.Join(kept, ",")
}

func Crypttab(text, mapper string, on bool) (string, bool) {
	lines := strings.Split(text, "\n")
	found := false
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != mapper {
			continue
		}
		found = true
		options := ""
		if len(fields) > 3 {
			options = fields[3]
		}
		kept := fields[:3]
		if changed := withOption(options, on); changed != "" {
			kept = append(kept, changed)
		}
		lines[index] = strings.Join(kept, " ")
	}
	return strings.Join(lines, "\n"), found
}

func optionsOf(text, mapper string) ([]string, bool) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != mapper {
			continue
		}
		if len(fields) > 3 {
			return strings.Split(fields[3], ","), true
		}
		return nil, true
	}
	return nil, false
}

func DracutConfig(withTPM bool) string {
	modules := "crypt"
	if withTPM {
		modules += " tpm2-tss"
	}
	return "hostonly=\"yes\"\nadd_dracutmodules+=\" " + modules + " \"\n"
}

func (image Image) Problems(mapper string, withTPM bool) []string {
	var problems []string
	options, found := optionsOf(image.Crypttab, mapper)
	switch {
	case !found:
		problems = append(problems, "a crypttab line for "+mapper)
	case withTPM && !slices.Contains(options, Option):
		problems = append(problems, Option+" for "+mapper)
	}
	if !strings.Contains(image.Cmdline, "rd.luks.uuid=") {
		problems = append(problems, "rd.luks.uuid")
	}
	parts := []string{"systemd-cryptsetup"}
	if withTPM {
		parts = append(parts, "libtss2-esys")
	}
	for _, part := range parts {
		if !strings.Contains(image.Listing, part) {
			problems = append(problems, part)
		}
	}
	return problems
}
