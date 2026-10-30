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

func CacheHome() string {
	return Or("XDG_CACHE_HOME", filepath.Join(Home(), ".cache"))
}
