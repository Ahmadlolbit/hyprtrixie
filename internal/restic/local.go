package restic

import (
	"path/filepath"
	"slices"
	"strings"
)

var remoteKinds = []string{"sftp:", "rest:", "s3:", "b2:", "azure:", "gs:", "swift:", "rclone:", "opendal:"}

func InsideHome(location, home string) (string, bool) {
	path := strings.TrimPrefix(location, "local:")
	if slices.ContainsFunc(remoteKinds, func(kind string) bool { return strings.HasPrefix(location, kind) }) || !filepath.IsAbs(path) {
		return "", false
	}
	clean := filepath.Clean(path)
	return clean, strings.HasPrefix(clean+"/", filepath.Clean(home)+"/")
}
