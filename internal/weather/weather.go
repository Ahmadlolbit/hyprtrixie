package weather

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/ordered"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const api = "https://api.open-meteo.com/v1/forecast"

func URL(coordinates, units string) string {
	latitude, longitude, _ := strings.Cut(coordinates, ",")
	wind := "mph"
	if units != "fahrenheit" {
		units = "celsius"
		wind = "kmh"
	}
	return api + "?latitude=" + latitude + "&longitude=" + longitude +
		"&current=temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,weather_code,is_day" +
		"&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max" +
		"&timezone=auto&forecast_days=5&temperature_unit=" + units + "&wind_speed_unit=" + wind
}

func Fetch(url string) ([]byte, error) {
	output, err := run.Capture(run.Slow, "curl", "-fsS", "--max-time", "20", url)
	return []byte(output), err
}

func Stamp(data []byte, fetched int64) ([]byte, error) {
	document, err := ordered.Parse(data)
	if err != nil {
		return nil, err
	}
	raw, ok := document.Get("current")
	if !ok {
		return nil, errors.New("no current weather")
	}
	current, err := ordered.Parse(raw)
	if err != nil {
		return nil, err
	}
	temperature, ok := current.Get("temperature_2m")
	if !ok || bytes.Equal(bytes.TrimSpace(temperature), []byte("null")) {
		return nil, errors.New("no current temperature")
	}
	document.Set("fetched", json.RawMessage(strconv.FormatInt(fetched, 10)))
	return document.Indented()
}

type saved struct {
	Current struct {
		Temperature any `json:"temperature_2m"`
		Feels       any `json:"apparent_temperature"`
		Time        any `json:"time"`
	} `json:"current"`
	Units struct {
		Temperature any `json:"temperature_2m"`
		Feels       any `json:"apparent_temperature"`
	} `json:"current_units"`
}

func Summary(data []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var weather saved
	if err := decoder.Decode(&weather); err != nil {
		return "", err
	}
	return text(weather.Current.Temperature) + text(weather.Units.Temperature) + ", feels like " +
		text(weather.Current.Feels) + text(weather.Units.Feels) + ", at " + text(weather.Current.Time), nil
}

func text(value any) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprint(value)
}
