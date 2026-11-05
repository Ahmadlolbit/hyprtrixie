package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

func rereadInput() int {
	run.Quiet(run.Quick, "hyprctl", "reload", "config-only")
	return 0
}

func trackpointSpeed(args []string) int {
	value := arg(args, 0)
	if value == "" {
		return show("trackpoint_speed")
	}
	speed, err := strconv.ParseFloat(value, 64)
	if err != nil || speed < -1 || speed > 1 {
		fmt.Fprintln(os.Stderr, "trackpoint-speed takes a number from -1 to 1")
		return 2
	}
	return putThen("trackpoint_speed", value, rereadInput)()
}

func trackpointAccel(args []string) int {
	switch value := arg(args, 0); value {
	case "":
		return show("trackpoint_accel")
	case "adaptive", "flat":
		return putThen("trackpoint_accel", value, rereadInput)()
	}
	fmt.Fprintln(os.Stderr, "trackpoint-accel takes adaptive or flat")
	return 2
}

func trackpointScroll(args []string) int {
	on, off := putThen("trackpoint_scroll", "yes", rereadInput), putThen("trackpoint_scroll", "no", rereadInput)
	return onOff{"trackpoint_scroll", on, off}.run(args)
}
