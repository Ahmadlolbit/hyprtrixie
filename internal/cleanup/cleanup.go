package cleanup

import (
	"os"
	"os/signal"
	"syscall"
)

func catch(channel chan<- os.Signal, watched ...os.Signal) {
	for _, one := range watched {
		if !signal.Ignored(one) {
			signal.Notify(channel, one)
		}
	}
}

func Notify(channel chan<- os.Signal) {
	catch(channel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
}

func SurviveBrokenPipes() {
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)
}

func NotifyInterrupt(channel chan<- os.Signal) {
	catch(channel, syscall.SIGINT)
}

func IgnoreQuit() func() {
	ignored := make(chan os.Signal, 1)
	catch(ignored, syscall.SIGQUIT)
	return func() { signal.Stop(ignored) }
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
