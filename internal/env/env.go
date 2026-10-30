package env

import (
	"os"
	"path/filepath"
)

func Or(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func Home() string {
	return os.Getenv("HOME")
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
