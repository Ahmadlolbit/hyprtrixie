package notify

import (
	"slices"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const limit = 30 * time.Second

var markup = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func Markup(text string) string {
	return markup.Replace(text)
}

func arguments(options []string, summary, body string) []string {
	args := append(slices.Clone(options), "--", summary)
	if body != "" {
		args = append(args, Markup(body))
	}
	return args
}

func Post(options []string, summary, body string) int {
	return run.Within(limit, "notify-send", arguments(options, summary, body)...)
}

func Ask(options []string, summary, body string) string {
	output, _ := run.Output("notify-send", arguments(options, summary, body)...)
	return strings.TrimSpace(output)
}
