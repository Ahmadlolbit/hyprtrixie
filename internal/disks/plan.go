package disks

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
)

const (
	espSize    = 1 * GiB
	bootSize   = 1 * GiB
	Mapper     = "hyprtrixie_crypt"
	Target     = "/target"
	top        = "/run/hyprtrixie-top"
	options    = "noatime,compress=zstd:1"
	RootName   = "hyprtrixie"
	Unfinished = RootName + "-unfinished"
	linuxData  = "0fc63daf-8483-4772-8e79-3d69d8477de4"
	linuxLUKS  = "ca7d7ccb-63ed-4c53-861c-1742536059cc"
)

type Plan struct {
	Disk   Disk
	Beside bool
	ESP    string
	Start  int64
	Size   int64
	Sector int64
	IDs    [3]string
	Reuses []string
}

type Step struct {
	Title  string
	Args   []string
	Secret bool
}

var volumes = [][2]string{{"home", "@home"}, {".snapshots", "@snapshots"}, {"var/log", "@var_log"}, {"swap", "@swap"}}

func NewID() string {
	var raw [16]byte
	binary.LittleEndian.PutUint64(raw[:8], rand.Uint64())
	binary.LittleEndian.PutUint64(raw[8:], rand.Uint64())
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

func ByID(id string) string {
	return "/dev/disk/by-partuuid/" + id
}

func Whole(disk Disk, ids [3]string) Plan {
	return Plan{Disk: disk, Size: disk.Size, Sector: 512, IDs: ids}
}

type RoomError struct {
	Free, Needed int64
}

func (err RoomError) Error() string {
	return fmt.Sprintf("the largest free space on this disk is %s; shrink Windows first to leave at least %s", Size(err.Free), Size(err.Needed))
}

func Beside(disk Disk, table Table, ids [3]string) (Plan, error) {
	esp, found := disk.ESP()
	switch {
	case table.Label != "gpt":
		return Plan{}, errors.New("this disk has an MBR partition table, so Windows on it isn't a UEFI install")
	case !found:
		return Plan{}, errors.New("this disk has no EFI partition to share")
	}
	free, reused := table.unfinished()
	gap := free.Largest()
	if gap.Size < SmallestUse+bootSize {
		return Plan{}, RoomError{gap.Size, SmallestUse + bootSize}
	}
	return Plan{Disk: disk, Beside: true, ESP: esp.Path, Start: gap.Start, Size: gap.Size, Sector: table.Sector, IDs: ids, Reuses: reused}, nil
}

func (plan Plan) ESPDevice() string {
	if plan.Beside {
		return plan.ESP
	}
	return ByID(plan.IDs[0])
}

func (plan Plan) BootDevice() string {
	return ByID(plan.IDs[1])
}

func (plan Plan) RootDevice() string {
	return ByID(plan.IDs[2])
}

func (plan Plan) sector(bytes int64) string {
	return strconv.FormatInt(bytes/plan.Sector, 10)
}

func (plan Plan) partitions() []string {
	if !plan.Beside {
		return []string{"sgdisk",
			"-n1:0:+1G", "-t1:ef00", "-u1:" + plan.IDs[0], "-c1:EFI",
			"-n2:0:+1G", "-t2:8300", "-u2:" + plan.IDs[1], "-c2:boot",
			"-n3:0:0", "-t3:8309", "-u3:" + plan.IDs[2], "-c3:" + Unfinished, plan.Disk.Path}
	}
	args := []string{"sgdisk"}
	for _, number := range plan.Reuses {
		args = append(args, "-d", number)
	}
	rootStart := plan.Start + bootSize
	end := plan.Start + plan.Size - plan.Sector
	return append(args,
		"-n0:"+plan.sector(plan.Start)+":"+plan.sector(rootStart-plan.Sector), "-t0:8300", "-u0:"+plan.IDs[1], "-c0:boot",
		"-n0:"+plan.sector(rootStart)+":"+plan.sector(end), "-t0:8309", "-u0:"+plan.IDs[2], "-c0:"+Unfinished, plan.Disk.Path)
}

func (plan Plan) Partition() []Step {
	var steps []Step
	if !plan.Beside {
		steps = append(steps, Step{"Clear the disk", []string{"sgdisk", "--zap-all", plan.Disk.Path}, false})
	}
	steps = append(steps,
		Step{"Make the partitions", plan.partitions(), false},
		Step{"Read the new partitions", []string{"partprobe", plan.Disk.Path}, false},
		Step{"Wait for them to appear", []string{"udevadm", "settle"}, false})
	if !plan.Beside {
		steps = append(steps, Step{"Format the EFI partition", []string{"mkfs.vfat", "-F", "32", "-n", "EFI", plan.ESPDevice()}, false})
	}
	return append(steps,
		Step{"Format the boot partition", []string{"mkfs.ext4", "-F", "-q", "-L", "boot", plan.BootDevice()}, false},
		Step{"Encrypt the rest", []string{"cryptsetup", "luksFormat", "--type", "luks2", "--batch-mode", "--key-file=-", plan.RootDevice()}, true},
		Step{"Unlock it", []string{"cryptsetup", "open", "--key-file=-", plan.RootDevice(), Mapper}, true},
		Step{"Make the btrfs filesystem", []string{"mkfs.btrfs", "-f", "-q", "-L", "hyprtrixie", "/dev/mapper/" + Mapper}, false})
}

func Layout() []Step {
	device := "/dev/mapper/" + Mapper
	steps := []Step{
		{"Open the filesystem", []string{"mkdir", "-p", top}, false},
		{"", []string{"mount", "-o", "subvolid=5", device, top}, false},
	}
	for _, name := range []string{"@", "@home", "@snapshots", "@var_log", "@swap"} {
		steps = append(steps, Step{"Make the " + name + " subvolume", []string{"btrfs", "subvolume", "create", top + "/" + name}, false})
	}
	steps = append(steps,
		Step{"", []string{"umount", top}, false},
		Step{"Mount the new system", []string{"mkdir", "-p", Target}, false},
		Step{"", []string{"mount", "-o", "subvol=@," + options, device, Target}, false})
	for _, volume := range volumes {
		mount := "subvol=" + volume[1] + "," + options
		if volume[1] == "@swap" {
			mount = "subvol=@swap,noatime"
		}
		steps = append(steps,
			Step{"", []string{"mkdir", "-p", Target + "/" + volume[0]}, false},
			Step{"", []string{"mount", "-o", mount, device, Target + "/" + volume[0]}, false})
	}
	return steps
}

func (plan Plan) Boot() []Step {
	return []Step{
		{"Mount the boot partition", []string{"mkdir", "-p", Target + "/boot"}, false},
		{"", []string{"mount", plan.BootDevice(), Target + "/boot"}, false},
		{"", []string{"mkdir", "-p", Target + "/boot/efi"}, false},
		{"", []string{"mount", plan.ESPDevice(), Target + "/boot/efi"}, false},
	}
}

type IDs struct {
	Root string
	Luks string
	Boot string
	ESP  string
}

func (ids IDs) Fstab() string {
	text := ""
	for _, volume := range append([][2]string{{"", "@"}}, volumes...) {
		mount := "subvol=" + volume[1] + "," + options
		if volume[1] == "@swap" {
			mount = "subvol=@swap,noatime"
		}
		text += fmt.Sprintf("UUID=%s /%s btrfs %s 0 0\n", ids.Root, volume[0], mount)
	}
	return text + fmt.Sprintf("UUID=%s /boot ext4 defaults 0 2\nUUID=%s /boot/efi vfat umask=0077 0 1\n", ids.Boot, ids.ESP)
}

func (ids IDs) Crypttab() string {
	return fmt.Sprintf("%s UUID=%s none luks,discard\n", Mapper, ids.Luks)
}
