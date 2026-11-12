package look

import (
	"path/filepath"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

func OwnBackgrounds(theme string) string {
	return fsutil.Followed(filepath.Join(env.AppConfig(), "backgrounds", theme))
}
