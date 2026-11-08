package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/catalog"
)

// palette is the Ctrl+K search over every tool in the catalog.
type palette struct {
	draft   string
	pos     int
	cursor  int
	results []catalog.Tool
}

func newPalette() *palette {
	p := &palette{}
	p.refresh()
	return p
}

func (p *palette) refresh() {
	p.results = catalog.Search(p.draft)
	p.cursor = max(0, min(p.cursor, len(p.results)-1))
}

// Update returns the chosen tool (if any) and whether the palette should close.
func (p *palette) Update(msg tea.KeyPressMsg) (*catalog.Tool, bool) {
	switch msg.String() {
	case "esc":
		return nil, true
	case "enter":
		if p.cursor < len(p.results) {
			tool := p.results[p.cursor]
			return &tool, true
		}
		return nil, true
	case "up", "ctrl+p":
		p.cursor = max(0, p.cursor-1)
	case "down", "ctrl+n":
		p.cursor = min(len(p.results)-1, p.cursor+1)
	default:
		before := p.draft
		p.draft, p.pos = editTextDraft(p.draft, p.pos, msg, 60)
		if p.draft != before {
			p.cursor = 0
			p.refresh()
		}
	}
	return nil, false
}

func (p *palette) View(w, h int) string {
	lines := []string{"Search: " + renderDraft(p.draft, p.pos), ""}
	rows := max(1, h-len(lines))
	start := 0
	if p.cursor >= rows {
		start = p.cursor - rows + 1
	}
	if len(p.results) == 0 {
		lines = append(lines, muted("No tool matches. Try a symptom such as \"no internet\" or \"disk full\"."))
	}
	for i := start; i < len(p.results) && i < start+rows; i++ {
		t := p.results[i]
		text := t.Title + "  " + muted(string(t.Area)+" · "+badges(t))
		if i == p.cursor {
			lines = append(lines, selected(t.Title+"  ·  "+string(t.Area)+" · "+badges(t), w))
		} else {
			lines = append(lines, "  "+text)
		}
	}
	return strings.Join(lines, "\n")
}

// badges is the text form of risk and admin need, shown wherever a tool is listed.
func badges(t catalog.Tool) string {
	parts := []string{map[catalog.Risk]string{
		catalog.RiskRead: "read-only", catalog.RiskLaunch: "opens a window",
		catalog.RiskChange: "changes system", catalog.RiskDestructive: "destructive",
	}[t.Risk]}
	switch t.Admin {
	case catalog.AdminRequired:
		parts = append(parts, "admin")
	case catalog.AdminOptional:
		parts = append(parts, "more data as admin")
	}
	return strings.Join(parts, " · ")
}
