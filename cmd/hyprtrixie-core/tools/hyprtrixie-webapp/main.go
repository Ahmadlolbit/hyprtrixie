package webapp

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/webapp"
)

const usage = `Usage: hyprtrixie-webapp add [NAME URL [ICON]]
       hyprtrixie-webapp remove [NAME]
       hyprtrixie-webapp list
       hyprtrixie-webapp open SLUG

add makes a website an app of its own: it shows in the launcher with the
site's icon, or the ICON file you give, and opens in its own window. It needs
a browser built on Chromium, such as Helium. With no name, add asks and remove
lets you pick. open is what the launcher runs.
`

const noBrowser = "this needs a browser built on Chromium: install Helium or Chromium first"

func ask(input *bufio.Reader, question string) string {
	fmt.Print(question)
	line, _ := input.ReadString('\n')
	return strings.TrimSpace(line)
}

func chooseIcon(name, address string) string {
	if err := os.MkdirAll(webapp.IconFolder(), 0o777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return webapp.GenericIcon
	}
	file := webapp.Icon(name)
	if run.Within(run.Quick, "curl", "-fsSL", "--max-time", "8", "-o", file, webapp.Favicon(address)) != 0 {
		os.Remove(file)
		return webapp.GenericIcon
	}
	return file
}

func add(args []string) int {
	input := bufio.NewReader(os.Stdin)
	if len(args) < 2 {
		args = []string{ask(input, "Name: "), ask(input, "Web address: ")}
	}
	name, address := args[0], args[1]
	if err := webapp.Check(name, address); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if other := webapp.Taken(name); other != "" {
		fmt.Fprintf(os.Stderr, "that clashes with your web app %s; pick another name\n", other)
		return 2
	}
	if webapp.Browser() == "" {
		fmt.Fprintln(os.Stderr, noBrowser)
		return 1
	}
	icon := ""
	if len(args) > 2 {
		icon = args[2]
	} else {
		icon = chooseIcon(name, address)
	}
	if err := os.MkdirAll(webapp.Folder(), 0o777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := atomic.Write(webapp.File(name), []byte(webapp.Entry(name, address, icon)), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%s is in the launcher\n", name)
	return 0
}

func open(slug string) int {
	address := webapp.Address(slug)
	if address == "" {
		fmt.Fprintf(os.Stderr, "no web app called %s\n", slug)
		return 1
	}
	browser := webapp.Browser()
	if browser == "" {
		fmt.Fprintln(os.Stderr, noBrowser)
		return 1
	}
	return run.Report(run.Exec(browser, "--app="+address, "--class="+webapp.Class(slug)))
}

func pick() string {
	names := run.Sorted(webapp.Names())
	if len(names) == 0 {
		fmt.Println("there are no web apps to remove")
		return ""
	}
	output, _ := run.Filter(strings.Join(names, "\n"), "fzf", "--layout", "reverse", "--prompt", "Remove: ")
	return strings.TrimSpace(output)
}

func remove(args []string) int {
	name := strings.Join(args, " ")
	if name == "" {
		name = pick()
	}
	if name == "" {
		return 0
	}
	if !fsutil.IsFile(webapp.File(name)) {
		fmt.Fprintf(os.Stderr, "no web app named %s; see hyprtrixie-webapp list\n", name)
		return 1
	}
	if err := os.Remove(webapp.File(name)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	os.Remove(webapp.Icon(name))
	fmt.Printf("%s is gone\n", name)
	return 0
}

func list() int {
	for _, name := range run.Sorted(webapp.Names()) {
		fmt.Println(name)
	}
	return 0
}

func Main() {
	switch env.Arg(1) {
	case "add":
		os.Exit(add(os.Args[2:]))
	case "remove":
		os.Exit(remove(os.Args[2:]))
	case "list":
		os.Exit(list())
	case "open":
		os.Exit(open(env.Arg(2)))
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
