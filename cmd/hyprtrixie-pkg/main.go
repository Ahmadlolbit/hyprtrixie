package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `Usage: hyprtrixie-pkg install|remove

install  search every Debian package and install the ones you pick
remove   search the installed packages and remove the ones you pick

Tab picks several, Enter goes ahead, Esc leaves without changing anything.
`

func pick(prompt string, names []string) []string {
	output, _ := run.Filter(strings.Join(run.Sorted(names), "\n"), "fzf", "--multi", "--layout", "reverse", "--prompt", prompt+" > ",
		"--header", "Tab picks several, Enter goes ahead, Esc leaves",
		"--preview", `apt-cache show {1} 2>/dev/null | awk "NF == 0 { exit } { print }"`,
		"--preview-window", "right,60%,wrap")
	return run.Lines(output)
}

func available() []string {
	output, _ := run.Capture(run.Slow, "apt-cache", "pkgnames")
	return run.Lines(output)
}

func installed() []string {
	output, _ := run.Capture(run.Slow, "dpkg-query", "-W", "-f=${db:Status-Status}\t${Package}\n")
	var names []string
	for _, line := range run.Lines(output) {
		if status, name, ok := strings.Cut(line, "\t"); ok && status == "installed" {
			names = append(names, name)
		}
	}
	return names
}

func change(action, prompt string, names []string) int {
	packages := pick(prompt, names)
	if len(packages) == 0 {
		return 0
	}
	return run.Status("sudo", append([]string{"apt-get", action}, packages...)...)
}

func main() {
	switch env.Arg(1) {
	case "install":
		os.Exit(change("install", "Install", available()))
	case "remove":
		os.Exit(change("remove", "Remove", installed()))
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
