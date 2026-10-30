package cleanup

import (
	"os"
	"os/signal"
	"syscall"
)

var signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}

func Notify(channel chan<- os.Signal) {
	for _, watched := range signals {
		if !signal.Ignored(watched) {
			signal.Notify(channel, watched)
		}
	}
}

func OnSignal(action func()) func() {
	received := make(chan os.Signal, 1)
	Notify(received)
	go watch(received, action)
	return func() {
		signal.Stop(received)
		close(received)
	}
}

func watch(received chan os.Signal, action func()) {
	if caught, ok := <-received; ok {
		action()
		signal.Stop(received)
		Raise(caught)
	}
}

func Raise(caught os.Signal) {
	if number, ok := caught.(syscall.Signal); ok {
		syscall.Tgkill(syscall.Getpid(), syscall.Gettid(), number)
	}
}
