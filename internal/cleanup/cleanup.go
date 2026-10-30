package cleanup

import (
	"os"
	"os/signal"
	"sync"
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
	done := make(chan struct{})
	go watch(received, done, action)
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			signal.Stop(received)
		})
	}
}

func watch(received chan os.Signal, done <-chan struct{}, action func()) {
	select {
	case caught := <-received:
		action()
		signal.Stop(received)
		Raise(caught)
	case <-done:
	}
}

func Raise(caught os.Signal) {
	if number, ok := caught.(syscall.Signal); ok {
		syscall.Kill(syscall.Getpid(), number)
	}
}
