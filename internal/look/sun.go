package look

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	noon    = 720
	fullDay = 1440
	zenith  = 90.833
)

func ZoneLookup(dir, zone string) (string, bool) {
	for _, name := range []string{"zone1970.tab", "zone.tab"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if found, ok := ZoneCoordinates(string(data), zone); ok {
			return found, true
		}
	}
	return "", false
}

func ZoneCoordinates(table, zone string) (string, bool) {
	for table != "" {
		var line string
		line, table, _ = strings.Cut(table, "\n")
		fields := strings.Split(line, "\t")
		if len(fields) >= 3 && fields[2] == zone {
			return coordinatesOf(fields[1])
		}
	}
	return "", false
}

func coordinatesOf(text string) (string, bool) {
	if len(text) < 2 {
		return "", false
	}
	split := strings.IndexAny(text[1:], "+-") + 1
	if split == 0 {
		return "", false
	}
	latitude, ok := degrees(text[:split])
	if !ok {
		return "", false
	}
	longitude, ok := degrees(text[split:])
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%.4f,%.4f", latitude, longitude), true
}

func degrees(text string) (float64, bool) {
	digits := text[1:]
	if !allDigits(digits) {
		return 0, false
	}
	size := 2 + len(digits)%2
	sign := 1.0
	if text[0] == '-' {
		sign = -1
	}
	return sign * (part(digits, 0, size) + part(digits, size, 2)/60 + part(digits, size+2, 2)/3600), true
}

func part(digits string, from, length int) float64 {
	if from >= len(digits) {
		return 0
	}
	value, err := strconv.ParseFloat(digits[from:min(from+length, len(digits))], 64)
	if err != nil {
		return 0
	}
	return value
}

func SunMinutes(latitude, longitude float64, day, offset int) (int, int) {
	angle := 2 * math.Pi / 365 * float64(day-1)
	equation := 229.18 * (0.000075 + 0.001868*math.Cos(angle) - 0.032077*math.Sin(angle) - 0.014615*math.Cos(2*angle) - 0.040849*math.Sin(2*angle))
	declination := 0.006918 - 0.399912*math.Cos(angle) + 0.070257*math.Sin(angle) - 0.006758*math.Cos(2*angle) + 0.000907*math.Sin(2*angle) -
		0.002697*math.Cos(3*angle) + 0.00148*math.Sin(3*angle)
	radians := latitude * math.Pi / 180
	cosine := math.Cos(zenith*math.Pi/180)/(math.Cos(radians)*math.Cos(declination)) - math.Tan(radians)*math.Tan(declination)
	if cosine >= 1 {
		return noon, noon
	}
	if cosine <= -1 {
		return 0, fullDay
	}
	hourAngle := math.Atan2(math.Sqrt(1-cosine*cosine), cosine) * 180 / math.Pi
	rise := int(noon - 4*(longitude+hourAngle) - equation + float64(offset) + 0.5)
	set := int(noon - 4*(longitude-hourAngle) - equation + float64(offset) + 0.5)
	return max(rise, 0), min(set, fullDay)
}
