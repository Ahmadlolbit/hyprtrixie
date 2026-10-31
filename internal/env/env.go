package env

import (
	"os"
	"os/user"
	"path/filepath"
)

func Or(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func Arg(index int) string {
	if index < len(os.Args) {
		return os.Args[index]
	}
	return ""
}

func Home() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if current, err := user.Current(); err == nil && current.HomeDir != "" {
		return current.HomeDir
	}
	return "/"
}

func ConfigHome() string {
	return Or("XDG_CONFIG_HOME", filepath.Join(Home(), ".config"))
}

func StateHome() string {
	return Or("XDG_STATE_HOME", filepath.Join(Home(), ".local", "state"))
}

func ThemeRoot() string {
	return Or("HYPRTRIXIE_THEME_ROOT", ShareDir())
}

func AppConfig() string {
	return filepath.Join(ConfigHome(), "hyprtrixie")
}

func ThemeState() string {
	return filepath.Join(StateHome(), "hyprtrixie", "current")
}

func CacheHome() string {
	return Or("XDG_CACHE_HOME", filepath.Join(Home(), ".cache"))
}

func ShareDir() string {
	return Or("HYPRTRIXIE_SHARE_DIR", "/usr/share/hyprtrixie")
}

func Root() string {
	return os.Getenv("HYPRTRIXIE_ROOT")
}

func BinDir() string {
	return Or("HYPRTRIXIE_BIN_DIR", "/usr/local/bin")
}

func ToolsDir() string {
	return Or("HYPRTRIXIE_TOOLS_DIR", filepath.Join(Root(), "build", "bin"))
}

func Manifests() string {
	return Or("HYPRTRIXIE_MANIFESTS", filepath.Join(Or("HYPRTRIXIE_ROOT", "."), "manifests"))
}

func CacheDir() string {
	return Or("HYPRTRIXIE_CACHE_DIR", "/var/cache/hyprtrixie")
}

func LogDir() string {
	return Or("HYPRTRIXIE_LOG_DIR", "/var/log/hyprtrixie")
}

func StateDir() string {
	return Or("HYPRTRIXIE_STATE_DIR", "/var/lib/hyprtrixie")
}
