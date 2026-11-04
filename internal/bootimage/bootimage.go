package bootimage

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const kernelPrefix = "vmlinuz-"

func found(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func Dracut() bool {
	return !found("update-initramfs") && found("dracut")
}

func Folder() string {
	return env.Or("HYPRTRIXIE_BOOT", "/boot")
}

func Newest() string {
	output, _ := run.CaptureQuiet(run.Minute, "ls", "-v", "--quoting-style=literal", Folder())
	newest := ""
	for _, name := range run.Lines(output) {
		if version, ok := strings.CutPrefix(name, kernelPrefix); ok {
			newest = version
		}
	}
	return newest
}

func Image(kernel string) string {
	return filepath.Join(Folder(), "initrd.img-"+kernel)
}

func Rebuild(all bool) []string {
	switch {
	case Dracut():
		if kernel := Newest(); kernel != "" {
			return []string{"dracut", "--force", Image(kernel), kernel}
		}
		return []string{"dracut", "--force"}
	case all:
		return []string{"update-initramfs", "-u", "-k", "all"}
	}
	return []string{"update-initramfs", "-u"}
}
