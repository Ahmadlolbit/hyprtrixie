package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/battery"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	trackPlaying   = "\U000f075a"
	trackPaused    = "\U000f03e4"
	batteryPlugged = "\U000f0084"
	lockLineRunes  = 60
	trackFormat    = "{{status}}\t{{title}}\t{{artist}}"
)

var (
	batteryGlyphs = []string{"\U000f0083", "\U000f007a", "\U000f007b", "\U000f007c", "\U000f007d", "\U000f007e", "\U000f007f", "\U000f0080", "\U000f0081", "\U000f0082", "\U000f0079"}
	markup        = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
)

func shortened(text string) string {
	runes := []rune(text)
	if len(runes) <= lockLineRunes {
		return text
	}
	return strings.TrimSpace(string(runes[:lockLineRunes-1])) + "…"
}

func chosenTrack(output string) []string {
	var paused []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 || fields[1] == "" {
			continue
		}
		if fields[0] == "Playing" {
			return fields
		}
		if fields[0] == "Paused" && paused == nil {
			paused = fields
		}
	}
	return paused
}

func trackLine() string {
	output, err := run.CaptureQuiet(run.Quick, "playerctl", "--all-players", "metadata", "--format", trackFormat)
	track := chosenTrack(output)
	if err != nil || track == nil {
		return ""
	}
	glyph, text := trackPaused, track[1]
	if track[0] == "Playing" {
		glyph = trackPlaying
	}
	if track[2] != "" {
		text += " · " + track[2]
	}
	return glyph + " " + markup.Replace(shortened(text))
}

func batteryLine() string {
	reading, found := battery.Read()
	if !found {
		return ""
	}
	glyph := batteryPlugged
	if !reading.Charging {
		glyph = batteryGlyphs[(reading.Percent+5)/10]
	}
	return fmt.Sprintf("%s %d%%", glyph, reading.Percent)
}

func lockShown(key string, line func() string) int {
	if settings.Get(key) == "yes" {
		fmt.Print(line())
	}
	return 0
}

func lockLine(args []string) int {
	switch arg(args, 0) {
	case "media":
		return lockShown("lock_media", trackLine)
	case "battery":
		return lockShown("lock_battery", batteryLine)
	}
	fmt.Fprintln(os.Stderr, "lock-line takes media or battery")
	return 2
}

func lockMedia(args []string) int {
	return onOff{"lock_media", putThen("lock_media", "yes", nothing), putThen("lock_media", "no", nothing)}.run(args)
}

func lockBattery(args []string) int {
	return onOff{"lock_battery", putThen("lock_battery", "yes", nothing), putThen("lock_battery", "no", nothing)}.run(args)
}
