package main

import (
	"fmt"
	"os"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const relayDevice = "/dev/video42"

func webcamStatus() int {
	if !installed("cam") {
		return fail("The IPU6 webcam isn't set up here: it comes with that camera (IPU6 webcam in the setup's System). Other webcams work in apps as they are.")
	}
	fmt.Println("Cameras libcamera finds:")
	code := run.Status("cam", "--list")
	fmt.Println("\nBrowsers and video calls reach them through PipeWire. For an app that takes only a normal webcam, run hyprtrixie webcam relay while you use it, or start the app with libcamerify.")
	return code
}

func webcamRelay() int {
	if os.Geteuid() == 0 {
		return fail("Run it without sudo: it asks for sudo only to load the relay.")
	}
	if !installed("cam") || !installed("gst-launch-1.0") {
		return fail("The IPU6 webcam isn't set up here, so there's nothing to relay.")
	}
	if run.Status("sudo", "modprobe", "v4l2loopback", "video_nr=42", "card_label=hyprtrixie webcam", "exclusive_caps=1") != 0 {
		return fail("The relay's kernel module didn't load: it's built for the newest kernel, so restart if one came since.")
	}
	if !fsutil.Exists(relayDevice) {
		return fail("Another app (such as OBS) loaded the relay's kernel module first, so there's no %s: close it, run sudo modprobe -r v4l2loopback, then try again.", relayDevice)
	}
	fmt.Printf("The camera is %s, called hyprtrixie webcam, until you press Ctrl+C.\n", relayDevice)
	return run.Status("gst-launch-1.0", "-q", "libcamerasrc", "!", "videoconvert", "!", "video/x-raw,format=YUY2", "!", "v4l2sink", "device="+relayDevice)
}

func webcamCommand() int {
	switch {
	case env.Arg(3) != "":
	case env.Arg(2) == "":
		return webcamStatus()
	case env.Arg(2) == "relay":
		return webcamRelay()
	}
	return fail("usage: hyprtrixie webcam [relay]")
}
