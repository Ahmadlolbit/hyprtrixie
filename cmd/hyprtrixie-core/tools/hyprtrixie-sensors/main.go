package sensors

import (
	"fmt"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/sensors"
)

const usage = `Usage: hyprtrixie-sensors [cpu|cameras]

Prints the CPU temperature, the other temperatures worth watching (NVMe
drives, an AMD GPU) and the fans as JSON, read from /sys/class/hwmon. On a
ThinkPad it adds the fan level from /proc/acpi/ibm/fan. cpu reads only the
CPU, which is what the bar shows; the rest is read while its popup is open.
cameras prints the apps that hold a camera open, one line each time that
changes, and keeps running.
`

func show(report sensors.Report) {
	output, err := report.JSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(output)
}

func Main() {
	root := os.Getenv("HYPRTRIXIE_SYSROOT")
	switch env.Arg(1) {
	case "":
		show(sensors.Read(root))
	case "cpu":
		show(sensors.ReadCPU(root))
	case "cameras":
		if err := sensors.Follow(root, os.Stdout); err != nil {
			os.Exit(1)
		}
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
