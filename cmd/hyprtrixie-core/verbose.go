package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

func verboseOn(flag string) int {
	err := os.MkdirAll(filepath.Dir(flag), 0o777)
	if err == nil {
		err = atomic.Write(flag, nil, 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fail("cannot write %s", flag)
	}
	fmt.Println("Verbose logging is on from the next login: log out and back in.")
	return 0
}

func verboseStatus(flag string) {
	if fsutil.IsFile(flag) {
		fmt.Println("on")
	} else {
		fmt.Println("off")
	}
}

func verbose() int {
	word := cmp.Or(env.Arg(2), "status")
	if os.Geteuid() == 0 {
		return fail("run it as yourself, without sudo: hyprtrixie verbose %s", word)
	}
	flag := filepath.Join(env.AppConfig(), "verbose")
	switch word {
	case "on":
		return verboseOn(flag)
	case "off":
		remove(flag)
		fmt.Println("Verbose logging is off from the next login.")
	case "status":
		verboseStatus(flag)
	default:
		return fail("usage: hyprtrixie verbose [on|off|status]")
	}
	return 0
}
