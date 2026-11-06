package main

import (
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/firewall"
)

const (
	trustUsage   = "usage: hyprtrixie trust [on|off]"
	notConnected = "you aren't connected to a network"
)

func trustCommand() int {
	switch action := env.Arg(2); action {
	case "":
		return trustShow()
	case "on", "off":
		return trustMark(action == "on")
	case "apply":
		return asRoot("trust apply", trustApply)
	}
	return fail(trustUsage)
}

func trustShow() int {
	connection, ok := firewall.Shown(firewall.Active())
	if !ok {
		return fail(notConnected)
	}
	if err := firewall.Trustable(connection.UUID); err != nil {
		return fail("%s can't be trusted: %v", connection.Name, err)
	}
	answer := "no"
	if firewall.Trusted(connection.UUID) {
		answer = "yes"
	}
	return say(answer + "\t" + connection.Name + "\n")
}

func trustNews(name string, trusted, applied bool) string {
	news := name + " isn't trusted any more, so LocalSend can't receive files on it"
	if trusted {
		news = name + " is trusted now, so LocalSend can receive files on it"
	}
	if !applied {
		news += ", from the next time you connect to it"
	}
	return news + "\n"
}

func trustMark(trusted bool) int {
	connection, ok := firewall.Shown(firewall.Active())
	if !ok {
		return fail(notConnected)
	}
	if err := firewall.Trustable(connection.UUID); trusted && err != nil {
		return fail("%s can't be trusted: %v", connection.Name, err)
	}
	applied, err := firewall.Mark(os.Stderr, connection, trusted)
	if err != nil {
		return fail("%s can't be changed: %v", connection.Name, err)
	}
	return say(trustNews(connection.Name, trusted, applied))
}

func trustApply() int {
	var err error
	if device := env.Arg(3); device != "" {
		err = firewall.Apply(os.Stderr, device, os.Getenv("CONNECTION_UUID"), env.Arg(4))
	} else {
		err = firewall.Refresh(os.Stderr)
	}
	if err != nil {
		return fail("the firewall can't follow the network: %v", err)
	}
	return 0
}
