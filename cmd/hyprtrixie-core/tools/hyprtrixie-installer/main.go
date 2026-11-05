package installer

import (
	"fmt"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/osinstall"
	"github.com/Ahmadlolbit/hyprtrixie/internal/tui"
)

func Main() {
	if len(os.Args) > 1 {
		fmt.Println("hyprtrixie-installer opens the installer with no arguments, as root, on the Hyprtrixie live system")
		return
	}
	if os.Getenv("HYPRTRIXIE_ROOT") == "" {
		os.Setenv("HYPRTRIXIE_ROOT", osinstall.Clone())
	}
	os.Exit(tui.RunInstaller())
}
