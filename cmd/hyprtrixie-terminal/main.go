package main

import (
	"fmt"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
)

const usage = `Usage: hyprtrixie-terminal [--wait] COMMAND [ARGUMENT...]

Runs COMMAND in a floating terminal in the middle of the screen.
--wait keeps the terminal open when COMMAND ends, until a key is pressed.
`

func inside(wait string, command []string) int {
	status := run.Status(command[0], command[1:]...)
	if wait != "1" {
		return status
	}
	fmt.Println()
	if status == 0 {
		fmt.Print("Done. Press any key to close.")
	} else {
		fmt.Printf("It ended with an error (exit %d). Press any key to close.", status)
	}
	term.ReadKey()
	return status
}

func main() {
	args := os.Args[1:]
	wait := "0"
	if len(args) > 0 {
		switch args[0] {
		case "--inside":
			if len(args) < 3 {
				fmt.Fprint(os.Stderr, usage)
				os.Exit(2)
			}
			os.Exit(inside(args[1], args[2:]))
		case "--wait":
			wait = "1"
			args = args[1:]
		case "-h", "--help":
			fmt.Print(usage)
			return
		}
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}
	os.Exit(run.Report(run.Exec("kitty", append([]string{"--class", "hyprtrixie.floating", "-e", self, "--inside", wait}, args...)...)))
}
