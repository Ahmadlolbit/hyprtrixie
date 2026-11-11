package restic

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	driveMount = regexp.MustCompile(`^(/run)?/media/[^/]+/[^/]+`)
	execEscape = strings.NewReplacer(`%`, `%%`, `\`, `\\`, `"`, `\"`)
)

func USBMount(location string) (string, bool) {
	_, path, ok := local(location)
	if !ok {
		return "", false
	}
	mount := driveMount.FindString(path)
	return mount, mount != ""
}

func Unit(mountUnit, program string) string {
	return fmt.Sprintf(`[Unit]
Description=Back up to the USB drive
Requires=%[1]s
After=%[1]s

[Service]
Type=oneshot
ExecStart="%[2]s" usb run

[Install]
WantedBy=%[1]s
`, mountUnit, execEscape.Replace(program))
}
