package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/term"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
)

const guideLeave = "Leave the guide"

func guideTitle(page string) string {
	for _, line := range strings.Split(fsutil.Optional(page), "\n") {
		if title, found := strings.CutPrefix(line, "# "); found {
			return title
		}
	}
	return strings.TrimSuffix(filepath.Base(page), ".md")
}

func guideText(pages []string) string {
	var text strings.Builder
	for _, page := range pages {
		text.WriteString(fsutil.Optional(page))
		text.WriteString("\n")
	}
	return text.String()
}

func guideMenu(pages []string) int {
	titles := make([]string, 0, len(pages)+1)
	for _, page := range pages {
		titles = append(titles, guideTitle(page))
	}
	titles = append(titles, guideLeave)
	for {
		pick, ok := ui.Choose("Guide: pick a topic", titles)
		if !ok || pick == guideLeave {
			return 0
		}
		for index, title := range titles[:len(pages)] {
			if title == pick {
				ui.Page(fsutil.Optional(pages[index]))
			}
		}
	}
}

func guide() int {
	dir := filepath.Join(root(), "docs", "guide")
	pages, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	if len(pages) == 0 {
		return say("The guide has no pages yet. They go in " + dir + ", one Markdown file per topic.\n")
	}
	if !term.IsTerminal(os.Stdout) {
		return say(guideText(pages))
	}
	return guideMenu(pages)
}
