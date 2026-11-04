package chime

import (
	"bytes"
	"encoding/binary"
	"math"
)

const (
	Rate     = 48000
	seconds  = 1.8
	attack   = 0.012
	decay    = 3.2
	spacing  = 0.11
	fadeOut  = 0.15
	loudness = 0.32
	overtone = 0.3
	upper    = 0.1
	weight   = 1 + overtone + upper
)

var (
	Dark  = []float64{392.00, 493.88, 587.33, 783.99}
	Light = []float64{523.25, 659.25, 783.99, 1046.50}
)

type header struct {
	Riff     [4]byte
	Size     uint32
	Wave     [4]byte
	Format   [4]byte
	Chunk    uint32
	Kind     uint16
	Channels uint16
	Rate     uint32
	ByteRate uint32
	Align    uint16
	Bits     uint16
	Data     [4]byte
	DataSize uint32
}

func note(pitch, local float64) float64 {
	envelope := math.Min(local/attack, 1) * math.Exp(-decay*local)
	tone := math.Sin(2*math.Pi*pitch*local) + overtone*math.Sin(4*math.Pi*pitch*local) + upper*math.Sin(6*math.Pi*pitch*local)
	return envelope * tone / weight
}

func Sample(notes []float64, at float64) float64 {
	total := 0.0
	for index, pitch := range notes {
		if start := float64(index) * spacing; at >= start {
			total += note(pitch, at-start)
		}
	}
	fade := math.Min((seconds-at)/fadeOut, 1)
	return total * loudness * 2 / float64(len(notes)) * math.Max(fade, 0)
}

func WAV(notes []float64) []byte {
	count := int(seconds * Rate)
	samples := make([]byte, 2*count)
	for index := range count {
		value := Sample(notes, float64(index)/Rate)
		binary.LittleEndian.PutUint16(samples[2*index:], uint16(int16(math.Round(value*math.MaxInt16))))
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, header{
		Riff: [4]byte{'R', 'I', 'F', 'F'}, Size: uint32(36 + len(samples)), Wave: [4]byte{'W', 'A', 'V', 'E'},
		Format: [4]byte{'f', 'm', 't', ' '}, Chunk: 16, Kind: 1, Channels: 1, Rate: Rate, ByteRate: Rate * 2,
		Align: 2, Bits: 16, Data: [4]byte{'d', 'a', 't', 'a'}, DataSize: uint32(len(samples)),
	})
	out.Write(samples)
	return out.Bytes()
}
