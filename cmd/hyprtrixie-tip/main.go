package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
)

const usage = `Usage: hyprtrixie-tip [terminal|welcome|notify]

Prints one tip at random. The tips come from /usr/share/hyprtrixie/tips.txt
and ~/.config/hyprtrixie/tips.txt, one tip per line.
terminal prints it dimmed, unless tips in new terminals are turned off;
welcome prints it unless tips on the welcome screen are turned off;
notify shows it as a notification, at most once a day, unless tip notifications are off.
`

var (
	tipsFile = env.Or("HYPRTRIXIE_TIPS", "/usr/share/hyprtrixie/tips.txt")
	switches = env.Or("HYPRTRIXIE_TIP_SWITCHES", "/etc/hyprtrixie/tips")
)

func enabled(name string) bool {
	data, err := os.ReadFile(switches)
	if err != nil {
		return true
	}
	return !slices.Contains(strings.Split(string(data), "\n"), name+"=no")
}

func tip() (string, bool) {
	var tips []string
	for _, file := range []string{tipsFile, filepath.Join(env.ConfigHome(), "hyprtrixie", "tips.txt")} {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) != "" {
				tips = append(tips, line)
			}
		}
	}
	if len(tips) == 0 {
		return "", false
	}
	return tips[rand.IntN(len(tips))], true
}

func printTip() int {
	line, ok := tip()
	if !ok {
		return 1
	}
	fmt.Println(line)
	return 0
}

func terminalTip() int {
	if !enabled("terminal") {
		return 0
	}
	if line, ok := tip(); ok {
		fmt.Printf("\x1b[2mtip: %s\x1b[0m\n", line)
	}
	return 0
}

func welcomeTip() int {
	if !enabled("welcome") {
		return 1
	}
	return printTip()
}

func notifyTip() int {
	if !enabled("notify") {
		return 0
	}
	stamp := filepath.Join(env.StateHome(), "hyprtrixie", "tip-shown")
	today := time.Now().Format(time.DateOnly)
	if data, err := os.ReadFile(stamp); err == nil && strings.TrimSpace(string(data)) == today {
		return 0
	}
	line, ok := tip()
	if !ok {
		return 0
	}
	if notify.Send("Tips", "Tip", line) != nil {
		return 1
	}
	if err := remember(stamp, today); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func remember(stamp, today string) error {
	if err := os.MkdirAll(filepath.Dir(stamp), 0o777); err != nil {
		return err
	}
	return atomic.Write(stamp, []byte(today+"\n"), 0o666)
}

func main() {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "":
		os.Exit(printTip())
	case "terminal":
		os.Exit(terminalTip())
	case "welcome":
		os.Exit(welcomeTip())
	case "notify":
		os.Exit(notifyTip())
	case "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
