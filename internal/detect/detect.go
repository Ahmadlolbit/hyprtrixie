package detect

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	firstOpenNvidia = 0x1e00
	kibPerMib       = 1024
	secureBootByte  = 4
	idGroupSize     = len("[0000:0000]")
)

var (
	sysroot   string
	laptopish = []string{"8", "9", "10", "11", "12", "14", "18", "21", "30", "31", "32"}
)

type machine struct {
	cpuVendor     string
	cpuModel      string
	gpuVendors    []string
	gpuNames      []string
	nvidiaOpen    bool
	wifiVendors   []string
	laptop        bool
	secureBoot    string
	virt          string
	rootFS        string
	rootSubvolume string
	rootEncrypted bool
	memoryMiB     int
	vendor        string
	product       string
}

func at(path string) string {
	return sysroot + path
}

func pciVendor(id string) string {
	switch id {
	case "8086":
		return "intel"
	case "1002", "1022":
		return "amd"
	case "10de":
		return "nvidia"
	case "10ec":
		return "realtek"
	case "14c3":
		return "mediatek"
	case "168c", "17cb":
		return "atheros"
	case "14e4":
		return "broadcom"
	}
	return "other"
}

func wifiDriverVendor(driver string) string {
	switch {
	case slices.Contains([]string{"iwlwifi", "iwlmvm", "iwlmld"}, driver):
		return "intel"
	case strings.HasPrefix(driver, "rtw88"), strings.HasPrefix(driver, "rtw89"), strings.HasPrefix(driver, "rtl8"), driver == "r8188eu":
		return "realtek"
	case strings.HasPrefix(driver, "mt7"):
		return "mediatek"
	case strings.HasPrefix(driver, "ath"):
		return "atheros"
	case slices.Contains([]string{"brcmfmac", "brcmsmac", "wl"}, driver):
		return "broadcom"
	}
	return "other"
}

func hex(text string) bool {
	return strings.Trim(text, "0123456789abcdef") == ""
}

func pciIDs(line string) (string, string) {
	for end := len(line); end >= idGroupSize; end-- {
		group := line[end-idGroupSize : end]
		if group[0] == '[' && group[5] == ':' && group[10] == ']' && hex(group[1:5]) && hex(group[6:10]) {
			return group[1:5], group[6:10]
		}
	}
	return "", ""
}

func gpuName(line string) string {
	_, rest, _ := strings.Cut(line, "]: ")
	vendor, device := pciIDs(rest)
	name, _, _ := strings.Cut(rest, " ["+vendor+":"+device+"]")
	return name
}

func addWord(list []string, word string) []string {
	if slices.Contains(list, word) {
		return list
	}
	return append(list, word)
}

func openNvidia(device string) bool {
	value, err := strconv.ParseUint(device, 16, 32)
	return err == nil && value >= firstOpenNvidia
}

func (found *machine) readGPUs(lspci string) {
	for _, line := range run.Lines(lspci) {
		if !strings.Contains(line, "[0300]:") && !strings.Contains(line, "[0302]:") && !strings.Contains(line, "[0380]:") {
			continue
		}
		vendor, device := pciIDs(line)
		name := pciVendor(vendor)
		found.gpuVendors = addWord(found.gpuVendors, name)
		found.gpuNames = append(found.gpuNames, gpuName(line))
		if name == "nvidia" && openNvidia(device) {
			found.nvidiaOpen = true
		}
	}
}

func (found *machine) readWifi(lspci string) {
	for _, line := range run.Lines(lspci) {
		if strings.Contains(line, "[0280]:") {
			vendor, _ := pciIDs(line)
			found.wifiVendors = addWord(found.wifiVendors, pciVendor(vendor))
		}
	}
	interfaces, _ := filepath.Glob(at("/sys/class/net/*/wireless"))
	for _, wireless := range interfaces {
		driver, _ := fsutil.RealPath(filepath.Join(filepath.Dir(wireless), "device/driver"))
		found.wifiVendors = addWord(found.wifiVendors, wifiDriverVendor(filepath.Base(driver)))
	}
}

func firstField(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if name, value, found := strings.Cut(line, ":"); found && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (found *machine) readCPU() {
	cpuinfo, _ := os.ReadFile(at("/proc/cpuinfo"))
	found.cpuModel = firstField(string(cpuinfo), "model name")
	switch firstField(string(cpuinfo), "vendor_id") {
	case "":
	case "GenuineIntel":
		found.cpuVendor = "intel"
	case "AuthenticAMD":
		found.cpuVendor = "amd"
	default:
		found.cpuVendor = "other"
	}
}

func secureBootVariable() string {
	variables, _ := filepath.Glob(at("/sys/firmware/efi/efivars/SecureBoot-*"))
	for _, variable := range variables {
		data, err := os.ReadFile(variable)
		if err != nil {
			continue
		}
		if len(data) > secureBootByte && data[secureBootByte] == 1 {
			return "enabled"
		}
		return "disabled"
	}
	return "unknown"
}

func secureBoot() string {
	if !fsutil.IsDir(at("/sys/firmware/efi")) {
		return "legacy"
	}
	if _, err := exec.LookPath("mokutil"); err != nil {
		return secureBootVariable()
	}
	state, _ := run.CaptureQuiet(run.Quick, "mokutil", "--sb-state")
	switch {
	case strings.Contains(state, "enabled"):
		return "enabled"
	case strings.Contains(state, "disabled"):
		return "disabled"
	}
	return "unknown"
}

func laptop() bool {
	chassis, _ := fsutil.Text(at("/sys/class/dmi/id/chassis_type"))
	if slices.Contains(laptopish, chassis) {
		return true
	}
	batteries, _ := filepath.Glob(at("/sys/class/power_supply/BAT*"))
	return len(batteries) > 0
}

func findmnt(column string) string {
	output, _ := run.CaptureQuiet(run.Quick, "findmnt", "-no", column, "/")
	return output
}

func subvolume(options string) string {
	for _, option := range strings.Split(options, ",") {
		if name, ok := strings.CutPrefix(option, "subvol="); ok {
			return strings.TrimPrefix(name, "/")
		}
	}
	return ""
}

func encrypted(device string) bool {
	if device == "" {
		return false
	}
	types, _ := run.CaptureQuiet(run.Quick, "lsblk", "-no", "TYPE", "-s", device)
	return slices.Contains(run.Lines(types), "crypt")
}

func (found *machine) readRoot() {
	found.rootFS = findmnt("FSTYPE")
	found.rootSubvolume = subvolume(findmnt("OPTIONS"))
	device, _, _ := strings.Cut(findmnt("SOURCE"), "[")
	found.rootEncrypted = encrypted(device)
}

func memory() int {
	meminfo, _ := os.ReadFile(at("/proc/meminfo"))
	fields := strings.Fields(firstField(string(meminfo), "MemTotal"))
	if len(fields) == 0 {
		return 0
	}
	kib, _ := strconv.Atoi(fields[0])
	return kib / kibPerMib
}

func dmi(name string) string {
	value, _ := fsutil.Text(at("/sys/class/dmi/id/" + name))
	return value
}

func product() string {
	if version := dmi("product_version"); version != "" && version != "None" {
		return version
	}
	return dmi("product_name")
}

func all() machine {
	found := machine{
		laptop:     laptop(),
		secureBoot: secureBoot(),
		memoryMiB:  memory(),
		vendor:     dmi("sys_vendor"),
		product:    product(),
	}
	found.readCPU()
	lspci, _ := run.CaptureQuiet(run.Quick, "lspci", "-nn")
	found.readGPUs(lspci)
	found.readWifi(lspci)
	virt, _ := run.CaptureQuiet(run.Quick, "systemd-detect-virt")
	found.virt = cmp.Or(virt, "none")
	found.readRoot()
	return found
}

func (found machine) form() string {
	form := "desktop"
	if found.laptop {
		form = "laptop"
	}
	if found.virt != "none" {
		form += ", virtual machine (" + found.virt + ")"
	}
	return form
}

func (found machine) firmware() string {
	if found.secureBoot == "legacy" {
		return "BIOS"
	}
	return "UEFI"
}

func (found machine) root() string {
	root := cmp.Or(found.rootFS, "?")
	if found.rootSubvolume != "" {
		root += " (subvolume " + found.rootSubvolume + ")"
	}
	if found.rootEncrypted {
		root += ", encrypted"
	}
	return root
}

func (found machine) gpuLines() string {
	var text strings.Builder
	for _, name := range found.gpuNames {
		fmt.Fprintf(&text, "GPU         %s\n", name)
	}
	if len(found.gpuVendors) > 1 {
		fmt.Fprintf(&text, "Graphics    hybrid (%s)\n", strings.Join(found.gpuVendors, " "))
	}
	if found.nvidiaOpen {
		text.WriteString("NVIDIA      open kernel modules supported\n")
	}
	if len(found.gpuNames) == 0 {
		text.WriteString("GPU         none found\n")
	}
	return text.String()
}

func (found machine) summary() string {
	return fmt.Sprintf("Machine     %s %s\nCPU         %s (%s)\n", cmp.Or(found.vendor, "unknown"), found.product, cmp.Or(found.cpuModel, "unknown"), cmp.Or(found.cpuVendor, "?")) +
		found.gpuLines() +
		fmt.Sprintf("Wi-Fi       %s\nMemory      %d MiB\nForm        %s\nFirmware    %s, Secure Boot %s\nRoot        %s\n",
			cmp.Or(strings.Join(found.wifiVendors, " "), "none found"), found.memoryMiB, found.form(), found.firmware(), found.secureBoot, found.root())
}

func Summary() string {
	return all().summary()
}
