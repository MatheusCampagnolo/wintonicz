package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type column struct {
	title string
	width int // 0 = takes the remaining width
}

type tableConfig[T any] struct {
	title    string
	noun     string // "processes", used in empty/limited text
	columns  []column
	load     func() ([]T, error)
	row      func(T) []string
	detail   func(T) string
	match    func(T, string) bool   // nil disables the / filter
	enter    func(T) tea.Cmd        // optional primary action
	enterKey string                 // status-bar label for enter
	note     string                 // always-visible explanatory line under the table
	extra    map[string]extraKey[T] // additional single-key actions on the selected row
}

type loadedMsg[T any] struct {
	gen   int
	items []T
	err   error
}

// tableScreen is the one table implementation behind processes, ports, services,
// USB, storage, and panels.
type tableScreen[T any] struct {
	cfg       tableConfig[T]
	gen       int
	loading   bool
	err       error
	items     []T
	visible   []T
	cursor    int
	offset    int
	filter    string
	draft     string
	draftPos  int
	filtering bool
	height    int
}

func newTable[T any](cfg tableConfig[T]) *tableScreen[T] {
	return &tableScreen[T]{cfg: cfg}
}

func (t *tableScreen[T]) Init() tea.Cmd {
	t.gen++
	t.loading = true
	t.err = nil
	gen := t.gen
	load := t.cfg.load
	return func() tea.Msg {
		items, err := load()
		return loadedMsg[T]{gen: gen, items: items, err: err}
	}
}

func (t *tableScreen[T]) Editing() bool { return t.filtering }
func (t *tableScreen[T]) Dirty() bool   { return t.filter != "" }

func (t *tableScreen[T]) Title() string {
	switch {
	case t.loading:
		return t.cfg.title + " · loading…"
	case t.err != nil:
		return t.cfg.title
	case t.filter != "":
		return fmt.Sprintf("%s · %d of %d", t.cfg.title, len(t.visible), len(t.items))
	}
	return fmt.Sprintf("%s · %d", t.cfg.title, len(t.items))
}

func (t *tableScreen[T]) Hints() []hint {
	if t.filtering {
		return []hint{{"enter", "apply"}, {"esc", "cancel"}, {"ctrl+u", "clear"}}
	}
	hints := []hint{{"↑↓", "move"}}
	if t.cfg.enter != nil {
		hints = append(hints, hint{"enter", t.cfg.enterKey})
	}
	if t.cfg.match != nil {
		hints = append(hints, hint{"/", "filter"})
	}
	for key, x := range t.cfg.extra {
		hints = append(hints, hint{key, x.label})
	}
	return append(hints, hint{"r", "refresh"})
}

func (t *tableScreen[T]) applyFilter() {
	t.visible = t.visible[:0]
	query := strings.ToLower(strings.TrimSpace(t.filter))
	for _, item := range t.items {
		if query == "" || t.cfg.match == nil || t.cfg.match(item, query) {
			t.visible = append(t.visible, item)
		}
	}
	t.cursor = max(0, min(t.cursor, len(t.visible)-1))
}

func (t *tableScreen[T]) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg[T]:
		if msg.gen != t.gen {
			return t, nil // a newer refresh superseded this result
		}
		t.loading = false
		t.err = msg.err
		if msg.err == nil {
			t.items = msg.items
			t.applyFilter()
		}
	case tea.KeyPressMsg:
		if t.filtering {
			return t, t.editFilter(msg)
		}
		return t, t.navigate(msg)
	}
	return t, nil
}

func (t *tableScreen[T]) editFilter(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		t.filter = strings.TrimSpace(t.draft)
		t.filtering = false
		t.cursor, t.offset = 0, 0
		t.applyFilter()
	case "esc":
		t.filtering = false
		t.draft = t.filter
	default:
		t.draft, t.draftPos = editTextDraft(t.draft, t.draftPos, msg, 80)
	}
	return nil
}

func (t *tableScreen[T]) navigate(msg tea.KeyPressMsg) tea.Cmd {
	page := max(1, t.height-2)
	switch msg.String() {
	case "up", "k":
		t.cursor--
	case "down", "j":
		t.cursor++
	case "pgup":
		t.cursor -= page
	case "pgdown":
		t.cursor += page
	case "home", "g":
		t.cursor = 0
	case "end", "G":
		t.cursor = len(t.visible) - 1
	case "/":
		if t.cfg.match != nil {
			t.filtering = true
			t.draft = t.filter
			t.draftPos = len([]rune(t.draft))
		}
	case "esc", "backspace":
		if t.filter != "" {
			t.filter, t.draft = "", ""
			t.cursor, t.offset = 0, 0
			t.applyFilter()
		}
	case "r":
		return t.Init()
	case "enter":
		if t.cfg.enter != nil && t.cursor < len(t.visible) {
			return t.cfg.enter(t.visible[t.cursor])
		}
	default:
		if x, ok := t.cfg.extra[msg.String()]; ok && t.cursor < len(t.visible) {
			return x.fn(t.visible[t.cursor])
		}
	}
	t.cursor = max(0, min(t.cursor, len(t.visible)-1))
	return nil
}

func (t *tableScreen[T]) View(w, h int) string {
	t.height = h
	var out []string
	if t.filtering {
		out = append(out, "filter: "+renderDraft(t.draft, t.draftPos))
	} else if t.filter != "" {
		out = append(out, muted("filter: ")+strong(t.filter)+muted("  (esc clears)"))
	}
	reserved := len(out)

	switch {
	case t.loading && len(t.items) == 0:
		return strings.Join(append(out, muted("Loading…")), "\n")
	case t.err != nil:
		msg := danger("✗ Could not read "+t.cfg.noun+": ") + t.err.Error()
		return strings.Join(append(out, wrap(msg, w), "", muted("Press r to try again. This is an error, not an empty result.")), "\n")
	case len(t.items) == 0:
		return strings.Join(append(out, "No "+t.cfg.noun+" were found."), "\n")
	case len(t.visible) == 0:
		return strings.Join(append(out, "Nothing matches the filter."), "\n")
	}

	widths := t.widths(w)
	out = append(out, muted(t.rowText(columnTitles(t.cfg.columns), widths)))
	rows := h - len(out)
	if t.cfg.note != "" {
		rows--
	}
	rows = max(1, rows)
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+rows {
		t.offset = t.cursor - rows + 1
	}
	_ = reserved
	for i := t.offset; i < len(t.visible) && i < t.offset+rows; i++ {
		text := t.rowText(t.cfg.row(t.visible[i]), widths)
		if i == t.cursor {
			out = append(out, selected(strings.TrimPrefix(text, "  "), w))
		} else {
			out = append(out, styleBase.Render(text))
		}
	}
	if t.cfg.note != "" {
		out = append(out, muted(t.cfg.note))
	}
	return strings.Join(out, "\n")
}

func columnTitles(cols []column) []string {
	titles := make([]string, len(cols))
	for i, c := range cols {
		titles[i] = c.title
	}
	return titles
}

func (t *tableScreen[T]) widths(total int) []int {
	widths := make([]int, len(t.cfg.columns))
	used := 2 // leading marker space
	flex := -1
	for i, c := range t.cfg.columns {
		if c.width == 0 {
			flex = i
			continue
		}
		widths[i] = c.width
		used += c.width + 2
	}
	if flex >= 0 {
		widths[flex] = max(8, total-used)
	}
	return widths
}

func (t *tableScreen[T]) rowText(cells []string, widths []int) string {
	var b strings.Builder
	b.WriteString("  ")
	for i, w := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		b.WriteString(fit(cell, w))
		if i < len(widths)-1 {
			b.WriteString("  ")
		}
	}
	return b.String()
}

func (t *tableScreen[T]) Details(w int) string {
	if t.cfg.detail == nil || t.cursor >= len(t.visible) {
		return ""
	}
	return wrap(t.cfg.detail(t.visible[t.cursor]), w)
}

// extraKey is a labelled single-key action on the selected row (for example "e" end process).
type extraKey[T any] struct {
	label string
	fn    func(T) tea.Cmd
}
