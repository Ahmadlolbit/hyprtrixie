package webapp

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

const (
	prefix      = "webapp-"
	suffix      = ".desktop"
	addressKey  = "X-Hyprtrixie-Address="
	nameKey     = "Name="
	slugLength  = 12
	GenericIcon = "applications-internet"
)

var (
	browsers = []string{"helium", "chromium", "brave-browser", "google-chrome", "microsoft-edge"}
	goodName = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} ._+-]*$`) })
	goodSlug = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[a-z0-9-]+$`) })
	unsafe   = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile("[\\s\"'`$\\\\<>]") })
)

func Folder() string {
	return filepath.Join(env.DataHome(), "applications")
}

func IconFolder() string {
	return filepath.Join(env.DataHome(), "icons", "webapps")
}

func Slug(name string) string {
	slug := strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug != "" {
		return slug
	}
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])[:slugLength]
}

func entryFile(slug string) string {
	return filepath.Join(Folder(), prefix+slug+suffix)
}

func File(name string) string {
	return entryFile(Slug(name))
}

func Icon(name string) string {
	return filepath.Join(IconFolder(), Slug(name)+".ico")
}

func Check(name, address string) error {
	if !goodName().MatchString(name) {
		return errors.New("a name uses letters, digits, spaces and . _ + -")
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || unsafe().MatchString(address) {
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

func Entry(name, address, icon string) string {
	return "[Desktop Entry]\nType=Application\n" + nameKey + name + "\nComment=" + address + " in its own window\nExec=hyprtrixie-webapp open " + Slug(name) +
		"\nIcon=" + icon + "\nCategories=Network;\nStartupNotify=true\nTerminal=false\n" + addressKey + address + "\n"
}

func field(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if value, ok := strings.CutPrefix(line, key); ok {
			return value
		}
	}
	return ""
}

func Address(slug string) string {
	if !goodSlug().MatchString(slug) {
		return ""
	}
	return field(fsutil.Optional(entryFile(slug)), addressKey)
}

func Class(slug string) string {
	return prefix + slug
}

func Taken(name string) string {
	if other := field(fsutil.Optional(File(name)), nameKey); other != name {
		return other
	}
	return ""
}

func Names() []string {
	var names []string
	for _, entry := range fsutil.Entries(Folder()) {
		if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), suffix) {
			if name := field(fsutil.Optional(filepath.Join(Folder(), entry.Name())), nameKey); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}
