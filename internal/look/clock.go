package look

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

func Now() (time.Time, bool) {
	text := os.Getenv("HYPRTRIXIE_NOW")
	if text == "" || text == "now" {
		return time.Now(), true
	}
	output, err := run.Capture(run.Quick, "date", "-d", text, "+%s")
	if err != nil {
		return time.Time{}, false
	}
	seconds, err := strconv.ParseInt(output, 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return time.Time{}, false
	}
	return time.Unix(seconds, 0), true
}

func DayOf(at time.Time) (int, int) {
	year, month, day := at.Date()
	_, offset := time.Date(year, month, day, 12, 0, 0, 0, at.Location()).Zone()
	return at.YearDay(), offset / 60
}

func DayOfDate(text string) (int, int, bool) {
	day, err := run.Capture(run.Quick, "date", "-d", text, "+%j")
	if err != nil {
		return 0, 0, false
	}
	zone, err := run.Capture(run.Quick, "date", "-d", text+" 12:00", "+%z")
	if err != nil {
		return 0, 0, false
	}
	number, err := strconv.Atoi(day)
	offset, ok := zoneMinutes(zone)
	if err != nil || !ok {
		fmt.Fprintf(os.Stderr, "date gave %q and %q\n", day, zone)
		return 0, 0, false
	}
	return number, offset, true
}

func zoneMinutes(text string) (int, bool) {
	if len(text) != 5 {
		return 0, false
	}
	minutes, ok := Minutes(text[1:3] + ":" + text[3:])
	switch {
	case ok && text[0] == '+':
		return minutes, true
	case ok && text[0] == '-':
		return -minutes, true
	}
	return 0, false
}

func Minutes(text string) (int, bool) {
	at, err := time.Parse("15:04", text)
	return at.Hour()*60 + at.Minute(), err == nil && len(text) == 5
}

func allDigits(text string) bool {
	for _, char := range []byte(text) {
		if char < '0' || char > '9' {
			return false
		}
	}
	return text != ""
}

func Clock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

func Inside(now, start, end int) bool {
	if start <= end {
		return now >= start && now < end
	}
	return now >= start || now < end
}

func WholeNumber(text string) (int64, bool) {
	value, err := strconv.ParseUint(text, 10, 63)
	return int64(value), err == nil
}

func IsCoordinates(value string) bool {
	latitude, longitude, ok := strings.Cut(value, ",")
	return ok && isDecimal(latitude, 0) && isDecimal(longitude, 0)
}

func isDecimal(text string, maxWhole int) bool {
	whole, fraction, dotted := strings.Cut(strings.TrimPrefix(text, "-"), ".")
	if maxWhole > 0 && len(whole) > maxWhole {
		return false
	}
	return allDigits(whole) && (!dotted || allDigits(fraction))
}

func ValidLocation(value string) bool {
	if value == "zone" {
		return true
	}
	latitude, longitude, ok := strings.Cut(value, ",")
	if !ok || !isDecimal(latitude, 3) || !isDecimal(longitude, 3) {
		return false
	}
	return within(latitude, 90) && within(longitude, 180)
}

func within(text string, limit float64) bool {
	value, err := strconv.ParseFloat(text, 64)
	return err == nil && value >= -limit && value <= limit
}
