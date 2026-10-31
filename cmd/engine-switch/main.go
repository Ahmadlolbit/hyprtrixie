package main

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const usage = `engine-switch - choose which container engine answers to "docker"

  engine-switch docker    use Docker CE
  engine-switch podman    use Podman; docker commands are passed to podman
  engine-switch status    show the engine in use
`

const shimScript = "#!/bin/sh\nexec /usr/bin/podman \"$@\"\n"

var (
	stateFile = "/var/lib/hyprtrixie/container-engine"
	shim      = "/usr/local/bin/docker"
	docker    = "/usr/bin/docker"
)

func current() string {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return "docker"
	}
	return strings.TrimRight(string(data), "\n")
}

func asRoot() {
	if os.Geteuid() == 0 {
		return
	}
	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}
	os.Exit(run.Report(run.Exec("sudo", append([]string{self}, os.Args[1:]...)...)))
}

func userSocket(action string) {
	user := os.Getenv("SUDO_USER")
	if user == "" || user == "root" {
		return
	}
	uid, err := run.Capture(run.Quick, "id", "-u", user)
	if err != nil {
		return
	}
	if _, err := strconv.Atoi(uid); err != nil {
		return
	}
	info, err := os.Stat("/run/user/" + uid + "/bus")
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return
	}
	run.Quiet(run.Slow, "systemctl", "--user", "--machine="+user+"@", action, "--now", "podman.socket")
}

func remember(engine string) {
	err := os.MkdirAll(filepath.Dir(stateFile), 0o777)
	if err == nil {
		err = atomic.Write(stateFile, []byte(engine+"\n"), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func useDocker() int {
	if _, err := exec.LookPath(docker); err != nil {
		fmt.Fprintln(os.Stderr, "Docker is not installed")
		return 1
	}
	userSocket("disable")
	run.Quiet(run.Slow, "systemctl", "disable", "--now", "podman.socket")
	if err := atomic.Symlink(docker, shim); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if run.Within(run.Slow, "systemctl", "enable", "--now", "docker.socket", "docker.service") != 0 {
		return 1
	}
	remember("docker")
	fmt.Println("docker now talks to Docker CE")
	return 0
}

func usePodman() int {
	if _, err := exec.LookPath("podman"); err != nil {
		fmt.Fprintln(os.Stderr, "Podman is not installed")
		return 1
	}
	run.Quiet(run.Slow, "systemctl", "disable", "--now", "docker.service", "docker.socket")
	if err := atomic.Install(shim, []byte(shimScript), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if run.Within(run.Slow, "systemctl", "enable", "--now", "podman.socket") != 0 {
		return 1
	}
	userSocket("enable")
	remember("podman")
	fmt.Println("docker now talks to Podman")
	return 0
}

func status() {
	fmt.Printf("engine: %s\n", current())
	if target, err := os.Readlink(shim); err == nil {
		fmt.Printf("docker: %s\n", target)
		return
	}
	if info, err := os.Stat(shim); err == nil && info.Mode().IsRegular() {
		fmt.Println("docker: podman shim")
		return
	}
	fmt.Println("docker: not set up")
}

func main() {
	switch cmp.Or(env.Arg(1), "status") {
	case "docker":
		asRoot()
		os.Exit(useDocker())
	case "podman":
		asRoot()
		os.Exit(usePodman())
	case "status":
		status()
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}
