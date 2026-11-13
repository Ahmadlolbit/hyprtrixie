package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/notify"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
	"github.com/Ahmadlolbit/hyprtrixie/internal/wallpaper"
)

const (
	nasaAddress  = "https://api.nasa.gov/planetary/apod?api_key=DEMO_KEY"
	pictureLimit = "40M"
	secondsADay  = 86400
	retryAfter   = 60 * 60
)

var (
	daySources   = []string{"off", "drawn", "nasa", "folder"}
	pictureKind  = []string{".jpg", ".jpeg", ".png", ".webp"}
	jpegStart    = []byte{0xff, 0xd8, 0xff}
	pngStart     = []byte{0x89, 'P', 'N', 'G'}
	errNoPicture = errors.New("NASA's picture of the day isn't a picture today")
)

type pictureOfTheDay struct {
	Kind      string `json:"media_type"`
	Address   string `json:"url"`
	Large     string `json:"hdurl"`
	Title     string `json:"title"`
	Copyright string `json:"copyright"`
}

func themeColorsNow() theme.Colors {
	data, _ := os.ReadFile(filepath.Join(env.ThemeState(), "theme", "colors.toml"))
	colors := theme.Parse(string(data))
	colors.Resolve()
	return colors
}

func dayFile(name string) string {
	return filepath.Join(lookState("days"), name)
}

func keepDay(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return atomic.Write(path, data, 0o666)
}

func drawnPicture(date string) (string, error) {
	path := dayFile(date + "-" + currentTheme() + ".jpg")
	if fsutil.IsFile(path) {
		return path, nil
	}
	img := wallpaper.Contours(themeColorsNow(), wallpaper.Seed(date+" "+currentTheme()), wallpaper.Width, wallpaper.Height)
	data, err := wallpaper.JPEG(img)
	if err != nil {
		return "", err
	}
	return path, keepDay(path, data)
}

func pictureType(data []byte) string {
	switch {
	case bytes.HasPrefix(data, jpegStart):
		return ".jpg"
	case bytes.HasPrefix(data, pngStart):
		return ".png"
	}
	return ""
}

func nasaDay() (pictureOfTheDay, error) {
	var day pictureOfTheDay
	output, err := run.CaptureQuiet(run.Minute, "curl", "-fsS", "--proto", "=https", "--max-time", "30", nasaAddress)
	if err == nil {
		err = json.Unmarshal([]byte(output), &day)
	}
	if err == nil && (day.Kind != "image" || !strings.HasPrefix(cmp.Or(day.Large, day.Address), "https://")) {
		err = errNoPicture
	}
	return day, err
}

func nasaPicture(date string) (string, error) {
	for _, kind := range []string{".jpg", ".png"} {
		if path := dayFile(date + "-nasa" + kind); fsutil.IsFile(path) {
			return path, nil
		}
	}
	day, err := nasaDay()
	if err != nil {
		return "", err
	}
	part := dayFile(date + "-nasa.part")
	if err := os.MkdirAll(filepath.Dir(part), 0o777); err != nil {
		return "", err
	}
	defer os.Remove(part)
	if run.Quiet(run.Slow, "curl", "-fsSL", "--proto", "=https", "--max-time", "300", "--max-filesize", pictureLimit, "-o", part, cmp.Or(day.Large, day.Address)) != 0 {
		return "", errors.New("NASA's picture of the day didn't download")
	}
	data, err := os.ReadFile(part)
	kind := pictureType(data)
	if err != nil || kind == "" {
		return "", errors.New("NASA's picture of the day isn't a JPEG or PNG")
	}
	path := dayFile(date + "-nasa" + kind)
	if err := os.Rename(part, path); err != nil {
		return "", err
	}
	credit := day.Title
	if day.Copyright != "" {
		credit += "\n© " + strings.TrimSpace(day.Copyright)
	}
	notify.Post([]string{"--app-name=Wallpaper"}, "NASA's picture of the day", credit)
	return path, nil
}

func expandHome(path string) string {
	if rest, found := strings.CutPrefix(path, "~/"); found {
		return filepath.Join(env.Home(), rest)
	}
	return path
}

func listedPictures(list string) []string {
	lines, _ := fsutil.Lines(list)
	var found []string
	for _, line := range lines {
		path := expandHome(strings.TrimSpace(line))
		if path != "" && !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(list), path)
		}
		if fsutil.IsFile(path) {
			found = append(found, path)
		}
	}
	return found
}

func ownPictures(place string) []string {
	if fsutil.IsFile(place) {
		return listedPictures(place)
	}
	folder := place
	if real, err := filepath.EvalSymlinks(place); err == nil {
		folder = real
	}
	var found []string
	for _, entry := range fsutil.Entries(folder) {
		path := filepath.Join(place, entry.Name())
		if !strings.HasPrefix(entry.Name(), ".") && slices.Contains(pictureKind, strings.ToLower(filepath.Ext(entry.Name()))) && fsutil.IsFile(path) {
			found = append(found, path)
		}
	}
	slices.Sort(found)
	return found
}

func folderPicture(day int64) (string, error) {
	place := expandHome(settings.Get("wallpaper_folder"))
	pictures := ownPictures(place)
	if len(pictures) == 0 {
		return "", fmt.Errorf("no pictures in %s", place)
	}
	return pictures[day%int64(len(pictures))], nil
}

func localDay(at time.Time) int64 {
	year, month, day := at.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix() / secondsADay
}

func dayPicture(source string, at time.Time) (string, bool, error) {
	date := at.Format(time.DateOnly)
	switch source {
	case "nasa":
		path, err := nasaPicture(date)
		if err == nil {
			return path, true, nil
		}
		drawn, drawing := drawnPicture(date)
		return drawn, errors.Is(err, errNoPicture), drawing
	case "folder":
		path, err := folderPicture(localDay(at))
		return path, true, err
	}
	path, err := drawnPicture(date)
	return path, true, err
}

func dropOldDays(keep string) {
	old, _ := filepath.Glob(dayFile("*"))
	for _, path := range old {
		if path != keep {
			remove(path)
		}
	}
}

func dayKey(source string, at time.Time) string {
	return at.Format(time.DateOnly) + " " + source + " " + currentTheme()
}

func dayDue() (string, time.Time, bool) {
	source := settings.Get("day_wallpaper")
	at, ok := moment()
	if !ok || !slices.Contains(daySources[1:], source) || stored(lookState("day-wallpaper")) == dayKey(source, at) {
		return "", at, false
	}
	return source, at, retryDue("day-wallpaper-retry")
}

func dayLock() (*os.File, bool) {
	path := lookState("day-wallpaper.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return nil, false
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return nil, false
	}
	if syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		file.Close()
		return nil, false
	}
	return file, true
}

func currentBackground() string {
	current, _ := run.CaptureQuiet(run.Quick, run.Sibling("hyprtrixie-background"), "--current")
	return strings.TrimSpace(current)
}

func showDay(path string) bool {
	resolved, err := fsutil.RealPath(path)
	if err == nil && currentBackground() == resolved {
		return true
	}
	return run.Quiet(run.Slow, run.Sibling("hyprtrixie-background"), path) == 0
}

func dayWallpaperNow() int {
	held, ok := dayLock()
	if !ok {
		return 0
	}
	defer held.Close()
	source, at, due := dayDue()
	if !due {
		return 0
	}
	path, final, err := dayPicture(source, at)
	if err == nil && !showDay(path) {
		err = errors.New("the wallpaper didn't change")
	}
	if err != nil || (!final && networkUp()) {
		save(lookState("day-wallpaper-retry"), strconv.FormatInt(at.Unix()+retryAfter, 10))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dropOldDays(path)
	if final {
		save(lookState("day-wallpaper"), dayKey(source, at))
		remove(lookState("day-wallpaper-retry"))
	}
	return 0
}

func dayWallpaperTick() {
	if _, _, due := dayDue(); !due {
		return
	}
	if self, err := os.Executable(); err == nil {
		run.Quiet(run.Quick, "setsid", "-f", self, "wallpaper-day", "now")
	}
}

func dayPictureShowing() bool {
	current := currentBackground()
	for _, place := range []string{lookState("days"), expandHome(settings.Get("wallpaper_folder"))} {
		if resolved, err := fsutil.RealPath(place); err == nil && current != "" && strings.HasPrefix(current, resolved+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func dayWallpaperOff() int {
	code := 0
	if dayPictureShowing() {
		code = run.Quiet(run.Slow, run.Sibling("hyprtrixie-background"), "next")
	}
	dropOldDays("")
	return code
}

func folderSetting(place string) string {
	if strings.HasPrefix(place, "~") {
		return place
	}
	if full, err := filepath.Abs(place); err == nil {
		return full
	}
	return place
}

func chooseDaySource(value, place string) int {
	if value == "folder" && place != "" && !put("wallpaper_folder", folderSetting(place)) {
		return 1
	}
	if !put("day_wallpaper", value) {
		return 1
	}
	for _, name := range []string{"day-wallpaper", "day-wallpaper-retry"} {
		remove(lookState(name))
	}
	if value == "off" {
		return dayWallpaperOff()
	}
	return dayWallpaperNow()
}

func dayWallpaper(args []string) int {
	switch value := strings.ToLower(arg(args, 0)); {
	case value == "":
		return show("day_wallpaper")
	case value == "now":
		return dayWallpaperNow()
	case !slices.Contains(daySources, value) || (value != "folder" && arg(args, 1) != ""):
		fmt.Fprintln(os.Stderr, "wallpaper-day takes off, drawn, nasa, or folder and a folder or list of pictures")
		return 2
	default:
		return chooseDaySource(value, arg(args, 1))
	}
}
