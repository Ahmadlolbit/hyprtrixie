package notify

import (
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const limit = 30 * time.Second

func Post(args ...string) int {
	return run.Within(limit, "notify-send", args...)
}

func Ask(args ...string) string {
	output, _ := run.Output("notify-send", args...)
	return strings.TrimSpace(output)
}
