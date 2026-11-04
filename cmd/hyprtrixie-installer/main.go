package main

import (
	"fmt"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Println("hyprtrixie-installer opens the installer with no arguments, as root, on the Hyprtrixie live system")
		return
	}
	os.Exit(tui.RunInstaller())
}
