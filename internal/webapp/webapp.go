package webapp

import (
	"errors"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	prefix      = "webapp-"
	suffix      = ".desktop"
	GenericIcon = "applications-internet"
)

var (
	browsers = []string{"helium", "chromium", "brave-browser", "google-chrome", "microsoft-edge"}
	goodName = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} ._+-]*$`)
	unsafe   = regexp.MustCompile("[\\s\"'`$\\\\<>]")
)

func Folder() string {
	return filepath.Join(env.DataHome(), "applications")
}

func IconFolder() string {
	return filepath.Join(env.DataHome(), "icons", "webapps")
}

func Slug(name string) string {
	return strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(name), "-"), "-")
}

func File(name string) string {
	return filepath.Join(Folder(), prefix+Slug(name)+suffix)
}

func Icon(name string) string {
	return filepath.Join(IconFolder(), Slug(name)+".ico")
}

func Check(name, address string) error {
	if !goodName.MatchString(name) || Slug(name) == "" {
		return errors.New("a name uses letters, digits, spaces and . _ + -")
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || unsafe.MatchString(address) {
		return errors.New("that isn't a web address: it starts with https:// and has no spaces or quotes")
	}
	return nil
}

func Browser() string {
	for _, name := range browsers {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

func Favicon(address string) string {
	parsed, err := url.Parse(address)
	if err != nil {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/favicon.ico"
}

func Entry(name, address, browser, icon string) string {
	command := browser + " --app=" + strings.ReplaceAll(address, "%", "%%") + " --class=" + prefix + Slug(name)
	return "[Desktop Entry]\nType=Application\nName=" + name + "\nComment=" + address + " in its own window\nExec=" + command +
		"\nIcon=" + icon + "\nCategories=Network;\nStartupNotify=true\nTerminal=false\n"
}

func nameIn(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if name, ok := strings.CutPrefix(line, "Name="); ok {
			return name
		}
	}
	return ""
}

func Names() []string {
	var names []string
	for _, entry := range fsutil.Entries(Folder()) {
		if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), suffix) {
			if name := nameIn(fsutil.Optional(filepath.Join(Folder(), entry.Name()))); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}
