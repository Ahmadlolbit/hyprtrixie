package record

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/hypr"
	"github.com/Ahmadlolbit/hyprtrixie/internal/look"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	usage = `Usage: hyprtrixie-record [region|screen|toggle|stop]

Records a region or the focused screen to ~/Videos/Recordings, with no sound.
The bar shows a mark while it records. stop ends it, and toggle stops a
recording or starts one of a region.
`
	recorder   = "wf-recorder"
	noRecorder = "wf-recorder isn't installed: sudo apt install wf-recorder"
	endedEarly = "wf-recorder stopped as soon as it started, so nothing was recorded"
	settle     = 3 * time.Second
	watch      = time.Second
)

func stateFile() string {
	return filepath.Join(env.StateHome(), "hyprtrixie", "recording")
}

func recording() bool {
	return run.Within(run.Quick, "pgrep", "-x", recorder) == 0
}

func target(mode string) ([]string, bool) {
	if mode == "region" {
		area, err := run.Output("slurp", "-d")
		return []string{"-g", area}, err == nil
	}
	output, ok := hypr.FocusedOutput()
	return []string{"-o", output}, ok
}

func remember(file string) {
	err := os.MkdirAll(filepath.Dir(stateFile()), 0o777)
	if err == nil {
		err = atomic.Write(stateFile(), []byte(file), 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func newFile() (string, error) {
	dir := filepath.Join(run.UserDir("VIDEOS", filepath.Join(env.Home(), "Videos")), "Recordings")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return "", err
	}
	return atomic.Fresh(dir, "Recording from "+time.Now().Format("2006-01-02 15-04-05"), ".mp4", 0o666)
}

func start(mode string) int {
	return startUnless(mode, recording)
}

func stopped() bool {
	return false
}

func startUnless(mode string, running func() bool) int {
	if _, err := exec.LookPath(recorder); err != nil {
		fmt.Fprintln(os.Stderr, noRecorder)
		notify.Post([]string{"--app-name=Recording"}, "Can't record", noRecorder)
		return 1
	}
	if running() {
		fmt.Fprintln(os.Stderr, "already recording; stop it with hyprtrixie-record stop")
		return 1
	}
	where, ok := target(mode)
	if !ok {
		return 0
	}
	file, err := newFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	remember(file)
	err = run.Detach(watch, recorder, append([]string{"-y", "-f", file}, where...)...)
	switch {
	case errors.Is(err, run.ErrEnded):
		return 0
	case err != nil:
		os.Remove(file)
		fmt.Fprintln(os.Stderr, endedEarly)
		notify.Post([]string{"--app-name=Recording"}, "Can't record", endedEarly)
		return 1
	}
	look.CallShell("recording", "started")
	return 0
}

func saved() string {
	file := fsutil.Optional(stateFile())
	if rest, ok := strings.CutPrefix(file, env.Home()); ok {
		return "~" + rest
	}
	return file
}

func stop() int {
	if run.Within(run.Quick, "pkill", "-INT", "-x", recorder) != 0 {
		fmt.Fprintln(os.Stderr, "nothing is recording")
		return 1
	}
	run.Quiet(settle, "pidwait", "-x", recorder)
	look.CallShell("recording", "stopped")
	return notify.Post([]string{"--app-name=Recording"}, "Recording saved", saved())
}

func toggle() int {
	if recording() {
		return stop()
	}
	return startUnless("region", stopped)
}

func Main() {
	mode := env.Arg(1)
	if mode == "" {
		mode = "region"
	}
	switch mode {
	case "region", "screen":
		os.Exit(start(mode))
	case "toggle":
		os.Exit(toggle())
	case "stop":
		os.Exit(stop())
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
