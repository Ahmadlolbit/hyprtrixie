package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Ahmadlolbit/hyprtrixie/internal/burst"
)

const (
	clearColumns = 4
	clearRows    = 2
	logoColumns  = burst.Size + 2*clearColumns
	textColumns  = 56
)

type logoTick struct{}

type welcome struct {
	*page
	started time.Time
}

func logoFits(width, height int) bool {
	return width >= logoColumns+textColumns && height >= burst.Size/2+clearRows
}

func (view *welcome) draw(width, height int) string {
	if !logoFits(width, height) {
		return view.page.draw(width, height)
	}
	text, wrapped := view.page.render(width-logoColumns, height-clearRows)
	if wrapped || lipgloss.Width(text) > width-logoColumns {
		return view.page.draw(width, height)
	}
	frame, _ := burst.At(time.Since(view.started))
	logo := strings.Repeat("\n", clearRows) + strings.Join(burst.Lines(frame), "\n")
	left := lipgloss.NewStyle().PaddingLeft(clearColumns).Width(logoColumns).Render(logo)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat("\n", clearRows)+text)
}
