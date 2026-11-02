package main

import (
	"fmt"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Println("hyprtrixie opens the setup with no arguments; for its commands see: hyprtrixie help")
		return
	}
	os.Exit(tui.Run())
}
