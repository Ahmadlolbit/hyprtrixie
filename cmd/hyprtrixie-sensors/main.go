package main

import (
	"fmt"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/sensors"
)

const usage = `Usage: hyprtrixie-sensors

Prints the CPU temperature, the other temperatures worth watching (NVMe
drives, an AMD GPU) and the fans as JSON, read from /sys/class/hwmon. On a
ThinkPad it adds the fan level from /proc/acpi/ibm/fan. The bar's
temperature widget reads it every few seconds.
`

func main() {
	switch env.Arg(1) {
	case "":
		output, err := sensors.Read(os.Getenv("HYPRTRIXIE_SYSROOT")).JSON()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Stdout.Write(output)
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
