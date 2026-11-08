package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Theme tokens. Color is never the only carrier of meaning: selection also uses a
// marker glyph and bold, and risk/admin always appear as text.
var (
	colAccent  = lipgloss.Color("#7DD3FC")
	colFocus   = lipgloss.Color("#38BDF8")
	colText    = lipgloss.Color("#CBD5E1")
	colStrong  = lipgloss.Color("#E2E8F0")
	colMuted   = lipgloss.Color("#94A3B8")
	colBorder  = lipgloss.Color("#334155")
	colSelect  = lipgloss.Color("#102A3E")
	colOK      = lipgloss.Color("#86EFAC")
	colWarn    = lipgloss.Color("#FCD34D")
	colDanger  = lipgloss.Color("#FCA5A5")
	styleBase  = lipgloss.NewStyle().Foreground(colText)
	styleMuted = lipgloss.NewStyle().Foreground(colMuted)
)

func accent(s string) string { return lipgloss.NewStyle().Bold(true).Foreground(colAccent).Render(s) }
func strong(s string) string { return lipgloss.NewStyle().Bold(true).Foreground(colStrong).Render(s) }
func muted(s string) string  { return styleMuted.Render(s) }
func okText(s string) string { return lipgloss.NewStyle().Foreground(colOK).Render(s) }
func warn(s string) string   { return lipgloss.NewStyle().Foreground(colWarn).Render(s) }
func danger(s string) string { return lipgloss.NewStyle().Foreground(colDanger).Render(s) }

// selected renders a full-width selected row with a marker, bold, and background.
func selected(s string, width int) string {
	return lipgloss.NewStyle().Bold(true).Foreground(colAccent).Background(colSelect).Render(fit("▌ "+s, width))
}

// fit truncates to width with an ellipsis and pads with spaces to exactly width cells.
func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = ansi.Truncate(s, width, "…")
	if pad := width - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// box draws a bordered panel of exactly w×h cells with the title in the top border.
// Body lines are truncated, never wrapped, so panels cannot change height.
func box(title, body string, w, h int, focused bool) string {
	if w < 6 || h < 3 {
		return ""
	}
	borderColor := colBorder
	titleStyle := styleMuted.Bold(true)
	if focused {
		borderColor = colFocus
		titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	}
	b := lipgloss.NewStyle().Foreground(borderColor)
	inner := w - 4
	title = ansi.Truncate(title, max(0, w-6), "…")
	top := b.Render("╭─ ") + titleStyle.Render(title) + b.Render(" "+strings.Repeat("─", max(0, w-5-ansi.StringWidth(title)))+"╮")
	lines := strings.Split(body, "\n")
	rows := make([]string, 0, h)
	rows = append(rows, top)
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		rows = append(rows, b.Render("│ ")+fit(line, inner)+b.Render(" │"))
	}
	rows = append(rows, b.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(rows, "\n")
}

// wrap breaks text to the width on word boundaries.
func wrap(s string, width int) string {
	if width < 4 {
		return s
	}
	return ansi.Wrap(s, width, "")
}

func bar(percent float64, width int) string {
	percent = max(0, min(100, percent))
	filled := int(percent / 100 * float64(width))
	return lipgloss.NewStyle().Foreground(colAccent).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("░", width-filled))
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(value)/float64(div), "KMGTPE"[exp])
}

func orUnavailable(value string) string {
	if value == "" {
		return "unavailable"
	}
	return value
}

func orNone(values []string) string {
	if len(values) == 0 {
		return "not configured"
	}
	return strings.Join(values, ", ")
}

func renderDraft(value string, cursor int) string {
	runes := []rune(value)
	cursor = max(0, min(cursor, len(runes)))
	if cursor == len(runes) {
		return string(runes) + accent("▏")
	}
	return string(runes[:cursor]) + lipgloss.NewStyle().Foreground(colStrong).Background(colAccent).Render(string(runes[cursor])) + string(runes[cursor+1:])
}
