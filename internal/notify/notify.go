package notify

import (
	"os/exec"
	"strings"
)

func Send(app, summary, body string) error {
	return exec.Command("notify-send", "--app-name="+app, summary, body).Run()
}

func Ask(args ...string) string {
	output, _ := exec.Command("notify-send", args...).Output()
	return strings.TrimSpace(string(output))
}
