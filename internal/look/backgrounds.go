package look

import (
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

func OwnBackgrounds(theme string) string {
	folder := filepath.Join(env.AppConfig(), "backgrounds", theme)
	if !fsutil.IsLink(folder) {
		return folder
	}
	if real, err := filepath.EvalSymlinks(folder); err == nil {
		return real
	}
	return folder
}
