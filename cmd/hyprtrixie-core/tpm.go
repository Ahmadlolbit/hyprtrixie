package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/bootimage"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/tpm"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
)

const tpmWarning = `This lets you type a short PIN at boot instead of the disk password.
The disk password keeps working, for when a firmware or boot change makes
the TPM refuse.

The boot image moves from initramfs-tools to dracut, for the newest kernel
only. A copy of today's goes into the boot menu first, under Advanced
options, as the kernel %s: if the new one doesn't start, pick that.
A restore point can't help here, because /boot isn't part of it.
`

const tpmDone = `Done. At the next boot, type your PIN. Your disk password still works.
If the new boot image doesn't start, pick the kernel %s
under Advanced options. Once the PIN has worked, take that copy away:
  sudo hyprtrixie tpm tidy
`

type luksRoot struct {
	mapper string
	device string
}

type backedOut int

const (
	untouched backedOut = iota
	putBack
	imageBack
	toolsBack
	stuck
)

func crypttab() string {
	return env.Or("HYPRTRIXIE_CRYPTTAB", "/etc/crypttab")
}

func dracutConfig() string {
	return env.Or("HYPRTRIXIE_DRACUT_CONF", "/etc/dracut.conf.d/hyprtrixie-tpm2.conf")
}

func joinedLog() *runlog.Log {
	log := runlog.Joined()
	log.Mirror(func(data []byte) { os.Stdout.Write(data) })
	return log
}

func findLuksRoot() (luksRoot, error) {
	source, _, _ := strings.Cut(detect.RootSource(), "[")
	listing, _ := run.CaptureQuiet(run.Quick, "lsblk", "-lnps", "-o", "NAME,TYPE", source)
	pool, _ := run.CaptureQuiet(run.Quick, "btrfs", "filesystem", "show", "/")
	devices := tpm.LuksDevices(listing)
	switch {
	case len(devices) == 0:
		return luksRoot{}, errors.New("the root isn't on an encrypted disk, so there's nothing for the TPM to unlock")
	case len(devices) > 1 || tpm.PoolDevices(pool) > 1:
		return luksRoot{}, errors.New("the root is on more than one disk, and TPM unlocking handles a root on one")
	}
	root := luksRoot{devices[0].Mapper, devices[0].Device}
	if run.Quiet(run.Quick, "cryptsetup", "isLuks", "--type", "luks2", root.device) != 0 {
		return luksRoot{}, fmt.Errorf("%s is LUKS1, which can't hold a TPM key", root.device)
	}
	return root, nil
}

func tpmPresent() bool {
	return fsutil.Exists(env.Or("HYPRTRIXIE_TPM_DEVICE", "/dev/tpmrm0"))
}

func crypttabWith(mapper string, on bool) (string, bool, error) {
	text, err := fsutil.Text(crypttab())
	if err != nil {
		return "", false, err
	}
	changed, found := tpm.Crypttab(text, mapper, on)
	if !found {
		return "", false, fmt.Errorf("%s has no line for %s", crypttab(), mapper)
	}
	return changed, changed != text, nil
}

func setCrypttab(mapper string, on bool) error {
	text, changed, err := crypttabWith(mapper, on)
	if err != nil || !changed {
		return err
	}
	return atomic.Install(crypttab(), []byte(text+"\n"), 0o644)
}

func writeDracutConfig(withTPM bool) error {
	if err := os.MkdirAll(filepath.Dir(dracutConfig()), 0o755); err != nil {
		return err
	}
	return atomic.Install(dracutConfig(), []byte(tpm.DracutConfig(withTPM)), 0o644)
}

func readImage(image string) tpm.Image {
	read := func(args ...string) string {
		output, _ := run.CaptureQuiet(run.Minute, "lsinitrd", append(args, image)...)
		return output
	}
	return tpm.Image{Listing: read(), Crypttab: read("-f", "etc/crypttab"), Cmdline: read("-f", "etc/cmdline.d/90crypt.conf")}
}

func checkImage(image, mapper string, withTPM bool) error {
	if problems := readImage(image).Problems(mapper, withTPM); len(problems) > 0 {
		return fmt.Errorf("the boot image lacks %s", strings.Join(problems, ", "))
	}
	return nil
}

func buildImage(log *runlog.Log, image, kernel, mapper string, withTPM bool) error {
	if err := run.Logged(log, "dracut", "--force", image, kernel); err != nil {
		return err
	}
	return checkImage(image, mapper, withTPM)
}

func replaceImage(log *runlog.Log, kernel, mapper string, withTPM bool) error {
	image := bootimage.Image(kernel)
	fresh := image + ".new"
	err := buildImage(log, fresh, kernel, mapper, withTPM)
	if err == nil {
		err = os.Rename(fresh, image)
	}
	if err != nil {
		os.Remove(fresh)
	}
	return err
}

func copyFile(from, to string) error {
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return atomic.Install(to, data, info.Mode().Perm())
}

func fallbackPairs(kernel string) [][2]string {
	file := func(version string) string { return filepath.Join(bootimage.Folder(), "vmlinuz-"+version) }
	return [][2]string{
		{file(kernel), file(tpm.Before(kernel))},
		{bootimage.Image(kernel), bootimage.Image(tpm.Before(kernel))},
	}
}

func fallbackFiles() []string {
	found, _ := filepath.Glob(filepath.Join(bootimage.Folder(), "*-"+tpm.Before("*")))
	return found
}

func dropFiles(paths []string) error {
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func updateGrub(log *runlog.Log) error {
	if apt.Commands("update-grub") != nil {
		return nil
	}
	return run.Logged(log, "update-grub")
}

func keepFallback(log *runlog.Log, kernel string) error {
	var made []string
	for _, pair := range fallbackPairs(kernel) {
		if fsutil.Exists(pair[1]) {
			continue
		}
		if err := copyFile(pair[0], pair[1]); err != nil {
			dropFiles(made)
			return err
		}
		made = append(made, pair[1])
	}
	return updateGrub(log)
}

func tidyFallback(log *runlog.Log) error {
	old := fallbackFiles()
	if len(old) == 0 {
		return nil
	}
	if err := dropFiles(old); err != nil {
		return err
	}
	return updateGrub(log)
}

func restoreImage(kernel string) error {
	return copyFile(bootimage.Image(tpm.Before(kernel)), bootimage.Image(kernel))
}

func trialImage(log *runlog.Log, root luksRoot, kernel string) error {
	if err := writeDracutConfig(true); err != nil {
		return err
	}
	if err := apt.InstallLists(log, "tpm2"); err != nil {
		return err
	}
	if err := apt.Fetch(log, "dracut", "initramfs-tools"); err != nil {
		return err
	}
	folder, err := os.MkdirTemp("", "hyprtrixie-tpm-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(folder)
	return buildImage(log, filepath.Join(folder, "initrd.img"), kernel, root.mapper, false)
}

func backOut(log *runlog.Log, kernel string) backedOut {
	installed := apt.InstallCached(log, "initramfs-tools") == nil
	if installed && run.Logged(log, "update-initramfs", "-u", "-k", kernel) == nil {
		os.Remove(dracutConfig())
		tidyFallback(log)
		return putBack
	}
	switch {
	case restoreImage(kernel) != nil:
		return stuck
	case installed:
		return toolsBack
	}
	return imageBack
}

func switchToDracut(log *runlog.Log, root luksRoot, kernel string) (backedOut, error) {
	if err := keepFallback(log, kernel); err != nil {
		os.Remove(dracutConfig())
		return untouched, err
	}
	err := apt.InstallCached(log, "dracut")
	if err == nil {
		err = checkImage(bootimage.Image(kernel), root.mapper, false)
	}
	if err == nil {
		return untouched, nil
	}
	return backOut(log, kernel), err
}

func switchFailed(how backedOut, err error, kernel string) int {
	switch how {
	case untouched:
		return fail("Moving to dracut stopped: %v. Nothing at boot has changed.", err)
	case imageBack:
		return fail("Moving to dracut stopped: %v. The boot image from before is back and asks for your disk password, but dracut is half set up: run sudo apt install initramfs-tools before the next update.", err)
	case toolsBack:
		return fail("Moving to dracut stopped: %v. initramfs-tools and the boot image from before are back, and your disk password works as before, but initramfs-tools couldn't make a new boot image: run sudo update-initramfs -u before the next update to see why.", err)
	case stuck:
		return fail("Moving to dracut stopped: %v. Neither initramfs-tools nor the old boot image could be put back. Before you restart, run sudo apt install initramfs-tools; if the next boot doesn't start, pick the kernel %s under Advanced options.", err, tpm.Before(kernel))
	}
	return fail("Moving to dracut stopped: %v. initramfs-tools and its boot image are back, and your disk password works as before.", err)
}

func enroll(device string) bool {
	return run.Status("systemd-cryptenroll", "--wipe-slot=tpm2", "--tpm2-device=auto", "--tpm2-with-pin=yes", "--tpm2-pcrs=7", device) == 0
}

func useTPM(log *runlog.Log, root luksRoot, kernel string) error {
	err := setCrypttab(root.mapper, true)
	if err == nil {
		err = replaceImage(log, kernel, root.mapper, true)
	}
	if err == nil {
		return nil
	}
	setCrypttab(root.mapper, false)
	run.Status("systemd-cryptenroll", "--wipe-slot=tpm2", root.device)
	return err
}

func turnOn(root luksRoot, kernel string) int {
	log := joinedLog()
	if err := trialImage(log, root, kernel); err != nil {
		os.Remove(dracutConfig())
		return fail("dracut can't make a boot image that unlocks this disk: %v. Nothing at boot has changed.", err)
	}
	if how, err := switchToDracut(log, root, kernel); err != nil {
		return switchFailed(how, err, kernel)
	}
	if !enroll(root.device) {
		return fail("The TPM key wasn't added. Your disk password works as before.")
	}
	if err := useTPM(log, root, kernel); err != nil {
		return fail("The boot image with the TPM didn't come out right: %v. The one you have asks for your disk password, as before.", err)
	}
	return say(fmt.Sprintf(tpmDone, tpm.Before(kernel)))
}

func tpmOn() int {
	root, err := findLuksRoot()
	if err != nil {
		return fail("%v", err)
	}
	if !tpmPresent() {
		return fail("This computer has no TPM 2.0, or it's turned off in the firmware.")
	}
	if !ui.CanAsk() {
		return fail("Run it in a terminal: it asks for your disk password and a new PIN.")
	}
	kernel := bootimage.Newest()
	_, changed, err := crypttabWith(root.mapper, true)
	switch {
	case err != nil:
		return fail("%v", err)
	case kernel == "":
		return fail("There's no kernel in %s.", bootimage.Folder())
	case !changed && unlocksWithTPM(root.mapper, kernel):
		return fail("TPM unlocking is already on. For a new PIN, turn it off and on again.")
	}
	fmt.Printf(tpmWarning, tpm.Before(kernel))
	if !ui.Confirm("Turn on unlocking with the TPM and a PIN?") {
		return fail("Nothing was changed.")
	}
	if _, err := snapshotBefore("before turning on TPM unlocking"); err != nil {
		return fail("%v", err)
	}
	return turnOn(root, kernel)
}

func passwordImage(log *runlog.Log, root luksRoot) error {
	if !bootimage.Dracut() {
		return nil
	}
	if err := writeDracutConfig(false); err != nil {
		return err
	}
	return replaceImage(log, bootimage.Newest(), root.mapper, false)
}

func tpmOff() int {
	root, err := findLuksRoot()
	if err != nil {
		return fail("%v", err)
	}
	if run.Status("systemd-cryptenroll", "--wipe-slot=tpm2", root.device) != 0 {
		return fail("The TPM key wasn't removed.")
	}
	if err := setCrypttab(root.mapper, false); err != nil {
		return fail("%v", err)
	}
	log := joinedLog()
	if err := passwordImage(log, root); err != nil {
		return fail("The boot image wasn't rebuilt: %v. The one you have asks for your disk password once the TPM key is gone.", err)
	}
	if err := tidyFallback(log); err != nil {
		return fail("%v", err)
	}
	return say("Done. At the next boot, type your disk password.\n")
}

func tpmTidy() int {
	if len(fallbackFiles()) == 0 {
		return say("There's no boot entry from before TPM unlocking.\n")
	}
	if err := tidyFallback(joinedLog()); err != nil {
		return fail("%v", err)
	}
	return say("The boot entry from before TPM unlocking is gone.\n")
}

func unlocksWithTPM(mapper, kernel string) bool {
	return len(readImage(bootimage.Image(kernel)).Problems(mapper, true)) == 0
}

func imageState(root luksRoot) string {
	contents := readImage(bootimage.Image(bootimage.Newest()))
	if len(contents.Problems(root.mapper, true)) == 0 {
		return "The newest kernel's boot image unlocks with the TPM and your PIN."
	}
	if problems := contents.Problems(root.mapper, false); len(problems) > 0 {
		return "The newest kernel's boot image lacks " + strings.Join(problems, ", ") + "."
	}
	return "The newest kernel's boot image asks for the disk password."
}

func tpmStatus() int {
	tool, present := "initramfs-tools", "no TPM 2.0 found"
	if bootimage.Dracut() {
		tool = "dracut"
	}
	if tpmPresent() {
		present = "a TPM 2.0 is there"
	}
	fmt.Printf("Boot image: %s\nTPM: %s\n", tool, present)
	if len(fallbackFiles()) > 0 {
		fmt.Println("The boot menu keeps the boot image from before under Advanced options. Once the PIN has worked: sudo hyprtrixie tpm tidy")
	}
	root, err := findLuksRoot()
	if err != nil {
		return say(err.Error() + "\n")
	}
	if tool == "dracut" {
		fmt.Println(imageState(root))
	}
	return run.Status("systemd-cryptenroll", root.device)
}

func tpmCommand() int {
	commands := map[string]func() int{"on": tpmOn, "off": tpmOff, "tidy": tpmTidy, "status": tpmStatus, "": tpmStatus}
	command, known := commands[env.Arg(2)]
	if !known || env.Arg(3) != "" {
		return fail("usage: hyprtrixie tpm on|off|tidy|status")
	}
	if env.Arg(2) == "" || env.Arg(2) == "status" {
		return asRoot("tpm "+env.Arg(2), command)
	}
	return asRoot("tpm "+env.Arg(2), alone(command))
}
