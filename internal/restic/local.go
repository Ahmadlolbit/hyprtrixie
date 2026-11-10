package restic

import (
	"path/filepath"
	"slices"
	"strings"
)

var remoteKinds = []string{"sftp:", "rest:", "s3:", "b2:", "azure:", "gs:", "swift:", "rclone:", "opendal:"}

func local(location string) (string, string, bool) {
	if slices.ContainsFunc(remoteKinds, func(kind string) bool { return strings.HasPrefix(location, kind) }) {
		return "", "", false
	}
	if path, found := strings.CutPrefix(location, "local:"); found {
		return "local:", path, true
	}
	return "", location, true
}

func within(folder, path string) string {
	if path == "" || filepath.IsAbs(path) || folder == "" {
		return path
	}
	return filepath.Join(folder, path)
}

func Absolute(location, home, folder string) string {
	prefix, path, ok := local(location)
	if !ok || path == "" {
		return location
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		path = filepath.Join(home, path[1:])
	}
	return prefix + filepath.Clean(within(folder, path))
}

func InsideHome(location, home, folder string) (string, bool) {
	_, path, ok := local(location)
	path = within(folder, path)
	if !ok || !filepath.IsAbs(path) {
		return "", false
	}
	clean := filepath.Clean(path)
	return clean, strings.HasPrefix(clean+"/", filepath.Clean(home)+"/")
}
