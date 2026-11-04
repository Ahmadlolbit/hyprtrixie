package burst

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"time"
)

const (
	Size = 32
	tick = time.Second / 60
	mark = "logo6"
)

type Frame [Size]uint32

type shown struct {
	ticks  int
	name   string
	pixels Frame
}

//go:embed flipbook.txt
var flipbook string

var loop = read(flipbook)

func read(text string) []shown {
	var frames []shown
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		fields := strings.Fields(line)
		if len(fields) != Size+2 {
			continue
		}
		ticks, err := strconv.Atoi(fields[0])
		if err != nil || ticks < 1 {
			continue
		}
		frame := shown{ticks: ticks, name: fields[1]}
		for row, word := range fields[2:] {
			value, _ := strconv.ParseUint(word, 16, 32)
			frame.pixels[row] = uint32(value)
		}
		frames = append(frames, frame)
	}
	return frames
}

func Length() time.Duration {
	total := 0
	for _, frame := range loop {
		total += frame.ticks
	}
	return time.Duration(total) * tick
}

func markStart() time.Duration {
	start := 0
	for _, frame := range loop {
		if frame.name == mark {
			break
		}
		start += frame.ticks
	}
	return time.Duration(start) * tick
}

func Still() Frame {
	for _, frame := range loop {
		if frame.name == mark {
			return frame.pixels
		}
	}
	return Frame{}
}

func At(elapsed time.Duration) (Frame, time.Duration) {
	length := Length()
	if length == 0 {
		return Frame{}, time.Hour
	}
	position := (max(elapsed, 0) + markStart()) % length
	at := int(position / tick)
	for _, frame := range loop {
		if at < frame.ticks {
			return frame.pixels, time.Duration(frame.ticks-at)*tick - position%tick
		}
		at -= frame.ticks
	}
	return Still(), length
}

func cell(top, bottom bool) rune {
	switch {
	case top && bottom:
		return '█'
	case top:
		return '▀'
	case bottom:
		return '▄'
	}
	return ' '
}

func Lines(frame Frame) []string {
	lines := make([]string, 0, Size/2)
	for row := 0; row < Size; row += 2 {
		var line strings.Builder
		for column := Size - 1; column >= 0; column-- {
			line.WriteRune(cell(frame[row]>>column&1 == 1, frame[row+1]>>column&1 == 1))
		}
		lines = append(lines, line.String())
	}
	return lines
}

func PNG(scale int, paint color.Color) ([]byte, error) {
	still := Still()
	side := Size * scale
	picture := image.NewNRGBA(image.Rect(0, 0, side, side))
	for at := range side * side {
		x, y := at%side, at/side
		if still[y/scale]>>(Size-1-x/scale)&1 == 1 {
			picture.Set(x, y, paint)
		}
	}
	var out bytes.Buffer
	err := png.Encode(&out, picture)
	return out.Bytes(), err
}
