package pick

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

const usage = `Usage: hyprtrixie-pick colour|qr

colour picks a colour from the screen and copies its hex code. qr reads the
QR code in a region of the screen and copies its text. Both show a
notification with what was copied.
`

func tell(title, text string) int {
	return notify.Post([]string{"--app-name=Pick"}, title, text)
}

func copied(text string) {
	run.Supply(run.Quick, os.Stderr, text, "wl-copy")
}

func hexIn(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if word := strings.TrimSpace(line); theme.IsHex(word) {
			return word
		}
	}
	return ""
}

func colour() int {
	output, err := run.Output("hyprpicker", "--format=hex", "--no-fancy")
	picked := hexIn(output)
	if err != nil || picked == "" {
		return 0
	}
	copied(picked)
	return tell("Colour copied", picked)
}

func readCode(area string) (string, bool) {
	dir, err := os.MkdirTemp("", "hyprtrixie-pick")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return "", false
	}
	defer os.RemoveAll(dir)
	image := filepath.Join(dir, "region.png")
	if run.Within(run.Quick, "grim", "-g", area, image) != 0 {
		return "", false
	}
	text, err := run.CaptureQuiet(run.Quick, "zbarimg", "--quiet", "--raw", image)
	return strings.TrimSpace(text), err == nil && strings.TrimSpace(text) != ""
}

func qr() int {
	area, err := run.Output("slurp", "-d")
	if err != nil {
		return 0
	}
	text, ok := readCode(area)
	if !ok {
		return tell("No QR code there", "Pick the area around the code and try again")
	}
	copied(text)
	return tell("QR code copied", text)
}

func Main() {
	switch env.Arg(1) {
	case "colour", "color":
		os.Exit(colour())
	case "qr":
		os.Exit(qr())
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
