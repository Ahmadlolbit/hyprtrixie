package osinstall

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/disks"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

type Answers struct {
	Disk     string `json:"disk"`
	Beside   bool   `json:"beside"`
	Password string `json:"password"`
	User     string `json:"user"`
	Name     string `json:"name"`
	Secret   string `json:"secret"`
	Host     string `json:"host"`
	Zone     string `json:"zone"`
	Keyboard string `json:"keyboard"`
	Locale   string `json:"locale"`
	DiskID   string `json:"disk_id"`
	Config   string `json:"config"`
}

var (
	userPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	hostPattern   = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	zonePattern   = regexp.MustCompile(`^[A-Za-z_]+(/[A-Za-z0-9_+-]+)*$`)
	localePattern = regexp.MustCompile(`^[a-z]{2,3}_[A-Z]{2}\.UTF-8$`)
	taken         = []string{"root", "daemon", "bin", "sys", "sync", "games", "man", "lp", "mail", "news", "uucp", "proxy", "www-data", "backup", "list", "irc", "nobody", "systemd-network", "messagebus", "sddm", "polkitd"}
	secureBoot    = detect.SecureBoot
	listing       = []string{"--json", "--bytes", "--output", "PATH,SIZE,TYPE,MODEL,TRAN,RM,RO,PTTYPE,FSTYPE,PARTTYPE,PARTLABEL,MOUNTPOINTS,SERIAL,WWN"}
)

func Read(path string) (Answers, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Answers{}, err
	}
	var answers Answers
	if err := json.Unmarshal(data, &answers); err != nil {
		return Answers{}, err
	}
	return answers, answers.Check()
}

func (answers Answers) Check() error {
	switch {
	case !strings.HasPrefix(answers.Disk, "/dev/"):
		return errors.New("no disk was picked")
	case answers.Password == "" || answers.Secret == "":
		return errors.New("a password is missing")
	case !userPattern.MatchString(answers.User) || slices.Contains(taken, answers.User):
		return errors.New("the user name has to start with a small letter, use small letters, digits, - and _, and not be one the system uses")
	case strings.ContainsAny(answers.Name, ":,\n"):
		return errors.New("the full name can't have : or , in it")
	case !hostPattern.MatchString(answers.Host):
		return errors.New("the computer's name can only have small letters, digits and - in it")
	case !zonePattern.MatchString(answers.Zone):
		return errors.New("the time zone isn't one like Europe/Istanbul")
	case !localePattern.MatchString(answers.Locale):
		return errors.New("the language isn't one like en_US.UTF-8")
	}
	return nil
}

func Ready() error {
	switch secureBoot() {
	case "enabled":
		return errors.New("this computer has Secure Boot on, and Hyprtrixie doesn't start with it: restart into the firmware setup (often F2, F10 or Del), turn Secure Boot off, then start the installer again")
	case "legacy":
		return errors.New("this computer started the stick in legacy BIOS mode, and Hyprtrixie needs UEFI: start it again and pick the stick's UEFI entry in the boot menu")
	}
	return nil
}

func Disks() ([]disks.Disk, error) {
	output, err := run.CaptureQuiet(run.Quick, "lsblk", listing...)
	if err != nil {
		return nil, err
	}
	all, err := disks.Parse(output)
	return slices.DeleteFunc(all, func(disk disks.Disk) bool { return !disk.Usable() }), err
}

func PlanFor(answers Answers) (disks.Plan, error) {
	output, err := run.CaptureQuiet(run.Quick, "lsblk", append(slices.Clone(listing), answers.Disk)...)
	if err != nil {
		return disks.Plan{}, err
	}
	all, err := disks.Parse(output)
	if err != nil || len(all) != 1 || all[0].Path != answers.Disk || !all[0].Usable() {
		return disks.Plan{}, errors.New(answers.Disk + " can't take the install: it's too small, read-only, or in use")
	}
	if answers.DiskID != "" && all[0].ID() != answers.DiskID {
		return disks.Plan{}, errors.New(answers.Disk + " isn't the disk you picked any more; start the installer again")
	}
	ids := [3]string{disks.NewID(), disks.NewID(), disks.NewID()}
	if !answers.Beside {
		return disks.Whole(all[0], ids), nil
	}
	dump, err := run.CaptureQuiet(run.Quick, "sfdisk", "--json", answers.Disk)
	if err != nil {
		return disks.Plan{}, err
	}
	table, err := disks.ReadTable(dump)
	if err != nil {
		return disks.Plan{}, err
	}
	return disks.Beside(all[0], table, ids)
}
