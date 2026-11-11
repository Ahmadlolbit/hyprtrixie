package disks

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
)

const (
	GiB         = int64(1) << 30
	MiB         = int64(1) << 20
	SmallestUse = 40 * GiB
	espType     = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
	msReserved  = "e3c9e316-0b5c-4db8-817d-f92df00215ae"
	msData      = "ebd0a0a2-b9e5-4433-87c0-68b6b72699c7"
	mbrNTFS     = "0x7"
)

var linuxSystems = []string{"btrfs", "crypto_LUKS", "ext2", "ext3", "ext4", "f2fs", "LVM2_member", "swap", "xfs"}

type Partition struct {
	Path        string      `json:"path"`
	Size        int64       `json:"size"`
	Type        string      `json:"type"`
	FSType      string      `json:"fstype"`
	PartType    string      `json:"parttype"`
	Label       string      `json:"partlabel"`
	Mountpoints []string    `json:"mountpoints"`
	Children    []Partition `json:"children"`
}

type Disk struct {
	Path       string      `json:"path"`
	Size       int64       `json:"size"`
	Type       string      `json:"type"`
	Model      string      `json:"model"`
	Transport  string      `json:"tran"`
	Removable  bool        `json:"rm"`
	ReadOnly   bool        `json:"ro"`
	Table      string      `json:"pttype"`
	FSType     string      `json:"fstype"`
	Serial     string      `json:"serial"`
	WWN        string      `json:"wwn"`
	Children   []Partition `json:"children"`
	Mountpoint []string    `json:"mountpoints"`
}

func Parse(listing string) ([]Disk, error) {
	var found struct {
		Devices []Disk `json:"blockdevices"`
	}
	if err := json.Unmarshal([]byte(listing), &found); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(found.Devices, func(disk Disk) bool { return disk.Type != "disk" }), nil
}

func (part Partition) mounted(where string) bool {
	if slices.ContainsFunc(part.Mountpoints, func(point string) bool { return point == where || strings.HasPrefix(point, where+"/") }) {
		return true
	}
	return slices.ContainsFunc(part.Children, func(child Partition) bool { return child.mounted(where) })
}

func (disk Disk) Holds(where string) bool {
	return slices.Contains(disk.Mountpoint, where) || slices.ContainsFunc(disk.Children, func(part Partition) bool { return part.mounted(where) })
}

func (disk Disk) Mounted() []string {
	var points []string
	queue := slices.Clone(disk.Children)
	for len(queue) > 0 {
		part := queue[0]
		queue = append(queue[1:], part.Children...)
		points = append(points, slices.DeleteFunc(slices.Clone(part.Mountpoints), func(point string) bool { return point == "" || point == "[SWAP]" })...)
	}
	slices.Sort(points)
	slices.Reverse(points)
	return points
}

func (disk Disk) Usable() bool {
	return !disk.ReadOnly && !disk.virtual() && disk.Size >= SmallestUse && !disk.Holds("/run/live/medium") && !disk.Holds("/")
}

func (disk Disk) ESP() (Partition, bool) {
	at := slices.IndexFunc(disk.Children, func(part Partition) bool { return strings.EqualFold(part.PartType, espType) })
	if at < 0 {
		return Partition{}, false
	}
	return disk.Children[at], true
}

func (disk Disk) Windows() bool {
	return slices.ContainsFunc(disk.Children, func(part Partition) bool {
		return strings.EqualFold(part.PartType, msReserved) || part.FSType == "ntfs" && (strings.EqualFold(part.PartType, msData) || part.PartType == mbrNTFS)
	})
}

func (disk Disk) External() bool {
	return disk.Transport == "usb" || disk.Removable
}

func (disk Disk) Contents() string {
	switch {
	case disk.Windows():
		return "Windows"
	case slices.Contains(linuxSystems, disk.FSType) || slices.ContainsFunc(disk.Children, Partition.linux):
		return "Linux"
	case disk.FSType != "" || slices.ContainsFunc(disk.Children, Partition.formatted):
		return "has files"
	case len(disk.Children) > 0:
		return "has partitions"
	}
	return "empty"
}

func (part Partition) linux() bool {
	return slices.Contains(linuxSystems, part.FSType) || slices.ContainsFunc(part.Children, Partition.linux)
}

func (part Partition) formatted() bool {
	return part.FSType != "" || slices.ContainsFunc(part.Children, Partition.formatted)
}

func (disk Disk) Name() string {
	model := strings.TrimSpace(disk.Model)
	if model == "" {
		model = "disk"
	}
	return model + " (" + strings.TrimPrefix(disk.Path, "/dev/") + ", " + Size(disk.Size) + ")"
}

func (disk Disk) virtual() bool {
	return slices.ContainsFunc([]string{"/dev/zram", "/dev/loop", "/dev/ram", "/dev/nbd"}, func(prefix string) bool { return strings.HasPrefix(disk.Path, prefix) })
}

func (disk Disk) ID() string {
	return cmp.Or(disk.WWN, disk.Serial)
}

func (part Partition) release(steps []Step) []Step {
	for _, child := range part.Children {
		steps = child.release(steps)
	}
	if slices.Contains(part.Mountpoints, "[SWAP]") {
		steps = append(steps, Step{Args: []string{"swapoff", part.Path}})
	}
	if part.Type == "lvm" {
		steps = append(steps, Step{Args: []string{"lvchange", "-an", part.Path}})
	}
	if part.Type == "crypt" {
		steps = append(steps, Step{Args: []string{"cryptsetup", "close", part.Path}})
	}
	return steps
}

func (disk Disk) Release() []Step {
	var steps []Step
	for _, part := range disk.Children {
		steps = part.release(steps)
	}
	return steps
}
