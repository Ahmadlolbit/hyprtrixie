package sensors

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const shortFile = 256

var (
	cpuSensors = []string{"coretemp", "k10temp", "zenpower", "cpu_thermal", "thinkpad", "acpitz"}
	cpuLabels  = map[string]string{"coretemp": "Package id 0", "k10temp": "Tctl", "zenpower": "Tdie"}
)

type Temp struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

type Fan struct {
	Label string `json:"label"`
	RPM   int64  `json:"rpm"`
}

type Report struct {
	CPU   *int64  `json:"cpu"`
	Temps []Temp  `json:"temps"`
	Fans  []Fan   `json:"fans"`
	Level *string `json:"level"`
}

type scan struct {
	report Report
	others []Temp
	drives int
	rank   int
}

func Read(root string) Report {
	current := scan{report: Report{Temps: []Temp{}, Fans: []Fan{}}, rank: len(cpuSensors)}
	for _, dir := range entries(root+"/sys/class/hwmon", "hwmon", "") {
		current.add(dir)
	}
	if cpu := current.report.CPU; cpu != nil {
		current.report.Temps = append(current.report.Temps, Temp{"CPU", *cpu})
	}
	current.report.Temps = append(current.report.Temps, current.others...)
	current.report.Level = fanLevel(root + "/proc/acpi/ibm/fan")
	return current.report
}

func (current *scan) add(dir string) {
	name, ok := text(filepath.Join(dir, "name"))
	if !ok {
		return
	}
	current.addCPU(dir, name)
	current.addTemp(dir, name)
	current.report.Fans = append(current.report.Fans, fans(dir, name)...)
}

func (current *scan) addCPU(dir, name string) {
	index := slices.Index(cpuSensors[:current.rank], name)
	if index < 0 {
		return
	}
	if value, ok := degrees(cpuInput(dir, name)); ok {
		current.report.CPU, current.rank = &value, index
	}
}

func (current *scan) addTemp(dir, name string) {
	if name != "nvme" && name != "amdgpu" {
		return
	}
	value, ok := degrees(filepath.Join(dir, "temp1_input"))
	if !ok {
		return
	}
	label := "GPU"
	if name == "nvme" {
		current.drives++
		label = driveLabel(current.drives)
	}
	current.others = append(current.others, Temp{label, value})
}

func (report Report) JSON() ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(report); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func driveLabel(drive int) string {
	if drive > 1 {
		return "NVMe " + strconv.Itoa(drive)
	}
	return "NVMe"
}

func entries(dir, prefix, suffix string) []string {
	found, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var paths []string
	for _, entry := range found {
		if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), suffix) {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	return paths
}

func head(path string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	buffer := make([]byte, shortFile)
	count, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false
	}
	return string(buffer[:count]), true
}

func text(path string) (string, bool) {
	content, ok := head(path)
	if !ok {
		return "", false
	}
	return strings.TrimRight(strings.ReplaceAll(content, "\x00", ""), "\n"), true
}

func reading(path string) (int64, bool) {
	content, _ := head(path)
	line, _, complete := strings.Cut(content, "\n")
	value, err := strconv.ParseUint(strings.Trim(line, " \t"), 10, 62)
	return int64(value), complete && err == nil
}

func degrees(path string) (int64, bool) {
	value, ok := reading(path)
	if !ok {
		return 0, false
	}
	return (value + 500) / 1000, true
}

func labelled(dir, want string) (string, bool) {
	for _, label := range entries(dir, "temp", "_label") {
		if value, ok := text(label); ok && value == want {
			return strings.TrimSuffix(label, "_label") + "_input", true
		}
	}
	return "", false
}

func cpuInput(dir, name string) string {
	if want, ok := cpuLabels[name]; ok {
		if input, ok := labelled(dir, want); ok {
			return input
		}
	}
	return filepath.Join(dir, "temp1_input")
}

func fans(dir, name string) []Fan {
	var found []Fan
	for _, fan := range entries(dir, "fan", "_input") {
		if value, ok := reading(fan); ok && (name == "thinkpad" || value > 0) {
			found = append(found, Fan{fanLabel(fan), value})
		}
	}
	return found
}

func fanLabel(fan string) string {
	if label, ok := text(strings.TrimSuffix(fan, "_input") + "_label"); ok {
		return label
	}
	digits := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(fan), "fan"), "_input")
	if count, err := strconv.ParseInt(digits, 10, 64); err == nil && count > 1 {
		return "Fan " + digits
	}
	return "Fan"
}

func fanLevel(path string) *string {
	data, _ := os.ReadFile(path)
	_, rest, found := strings.Cut("\n"+string(data), "\nlevel:")
	line, _, complete := strings.Cut(rest, "\n")
	level := strings.Join(strings.Fields(line), "")
	if !found || !complete || level == "" {
		return nil
	}
	return &level
}
