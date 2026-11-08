package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/catalog"
	"github.com/MatheusCampagnolo/wintonicz/internal/config"
	"github.com/MatheusCampagnolo/wintonicz/internal/privilege"
	"github.com/charmbracelet/x/ansi"
)

const (
	minWidth  = 60
	minHeight = 16
)

// toolOrder is the sidebar order. Tools missing from this list still appear, after it.
var toolOrder = []string{
	"inspect.system", "inspect.processes", "inspect.ports", "inspect.services", "inspect.keyservices", "inspect.usb", "inspect.diskhealth", "inspect.security", "inspect.startup", "inspect.programs",
	"inspect.storage", "inspect.resources", "net.status", "net.quick", "net.check", "net.flushdns", "win.panels", "settings.about",
}

var areaOrder = []catalog.Area{catalog.AreaInspect, catalog.AreaNetwork, catalog.AreaRepair, catalog.AreaClean, catalog.AreaWindows, catalog.AreaReports, catalog.AreaSettings}

type entry struct {
	id    string
	title string
	area  catalog.Area // "" for Home
	tool  *catalog.Tool
}

type focusArea int

const (
	focusSidebar focusArea = iota
	focusContent
)

type model struct {
	services app.Services
	version  string
	elevated bool
	cfg      config.Config
	entries  []entry
	cursor   int
	active   int
	screens  map[string]Screen
	focus    focusArea
	width    int
	height   int
	palette  *palette
	flow     *flow
	help     bool
	notice   string
	noticeEr bool
}

func buildEntries() []entry {
	entries := []entry{{id: "home", title: "Home"}}
	byArea := map[catalog.Area][]catalog.Tool{}
	for _, id := range toolOrder {
		if t, ok := catalog.Find(id); ok {
			byArea[t.Area] = append(byArea[t.Area], t)
		}
	}
	for _, t := range catalog.All() {
		if !contains(toolOrder, t.ID) {
			byArea[t.Area] = append(byArea[t.Area], t)
		}
	}
	for _, area := range areaOrder {
		for _, t := range byArea[area] {
			tool := t
			entries = append(entries, entry{id: t.ID, title: t.Title, area: t.Area, tool: &tool})
		}
	}
	return entries
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func newModel(services app.Services, version string) model {
	return model{
		services: services,
		version:  version,
		elevated: privilege.IsElevated(),
		cfg:      loadConfig(),
		entries:  buildEntries(),
		screens:  map[string]Screen{},
		width:    100,
		height:   30,
	}
}

// Run starts the interactive interface.
func Run(services app.Services, version string) error {
	_, err := tea.NewProgram(newModel(services, version)).Run()
	return err
}

func (m model) Init() tea.Cmd {
	for i, e := range m.entries {
		if string(e.area) == m.cfg.DefaultArea {
			return m.open(i)
		}
	}
	return m.open(0)
}

// open makes entry i the active tool: shows its screen (refreshing data) or, for
// action tools, starts the action flow.
func (m *model) open(i int) tea.Cmd {
	if i < 0 || i >= len(m.entries) {
		return nil
	}
	e := m.entries[i]
	if e.tool != nil && e.tool.Kind == catalog.KindAction {
		m.flow = newFlow(*e.tool, m.cfg)
		return nil
	}
	screen, ok := m.screens[e.id]
	if !ok {
		screen = buildScreen(e.id, m.services, m.version)
		if screen == nil {
			return nil
		}
		m.screens[e.id] = screen
	}
	m.active, m.cursor, m.focus = i, i, focusContent
	return route(e.id, screen.Init())
}

func (m model) activeScreen() Screen {
	if m.active < 0 || m.active >= len(m.entries) {
		return nil
	}
	return m.screens[m.entries[m.active].id]
}

func (m model) editing() bool {
	if m.focus == focusContent {
		if s := m.activeScreen(); s != nil {
			return s.Editing()
		}
	}
	return false
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case routedMsg:
		if s, ok := m.screens[msg.id]; ok {
			next, cmd := s.Update(msg.msg)
			m.screens[msg.id] = next
			return m, route(msg.id, cmd)
		}
		return m, nil
	case noticeMsg:
		m.notice, m.noticeEr = msg.text, msg.isErr
		return m, nil
	case elevatedMsg:
		switch {
		case msg.err == nil:
			return m, tea.Quit
		case errors.Is(msg.err, privilege.ErrCancelled):
			m.notice, m.noticeEr = "Administrator approval was cancelled. Nothing changed.", false
		default:
			m.notice, m.noticeEr = "Could not restart as administrator: "+msg.err.Error(), true
		}
		return m, nil
	case flowEvent:
		if m.flow != nil {
			cmd, closed := m.flow.Update(msg)
			if closed {
				m.flow = nil
			}
			return m, cmd
		}
		return m, nil
	case tea.KeyPressMsg:
		m.notice = ""
		return m.updateKey(msg)
	}
	return m, nil
}

func (m model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.flow != nil {
		if key == "ctrl+c" && !m.flow.running() {
			return m, tea.Quit
		}
		cmd, closed := m.flow.Update(msg)
		if closed {
			m.flow = nil
		}
		return m, cmd
	}
	if m.palette != nil {
		tool, closed := m.palette.Update(msg)
		if closed {
			m.palette = nil
		}
		if tool != nil {
			for i, e := range m.entries {
				if e.id == tool.ID {
					return m, m.open(i)
				}
			}
		}
		return m, nil
	}
	if m.help {
		m.help = false
		return m, nil
	}
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.editing() {
		return m.toScreen(msg)
	}
	switch key {
	case "q":
		return m, tea.Quit
	case "ctrl+k", "ctrl+p":
		m.palette = newPalette()
		return m, nil
	case "?":
		m.help = true
		return m, nil
	case "a":
		if !m.elevated {
			return m, elevateCmd()
		}
		return m, nil
	case "1", "2", "3", "4", "5", "6":
		return m.jumpToArea(int(key[0] - '1'))
	}
	if m.focus == focusSidebar {
		switch key {
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(len(m.entries)-1, m.cursor+1)
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.entries) - 1
		case "enter", "right", "l", "tab":
			return m, m.open(m.cursor)
		}
		return m, nil
	}
	s := m.activeScreen()
	switch key {
	case "tab", "left", "h":
		m.focus = focusSidebar
		return m, nil
	case "esc", "backspace":
		if s == nil || !s.Dirty() {
			m.focus = focusSidebar
			return m, nil
		}
	}
	return m.toScreen(msg)
}

func (m model) toScreen(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.activeScreen()
	if s == nil {
		return m, nil
	}
	id := m.entries[m.active].id
	next, cmd := s.Update(msg)
	m.screens[id] = next
	return m, route(id, cmd)
}

// jumpToArea moves the sidebar cursor to the first tool of the nth area (0 = Home).
func (m model) jumpToArea(n int) (tea.Model, tea.Cmd) {
	if n == 0 {
		m.cursor, m.focus = 0, focusSidebar
		return m, nil
	}
	var present []catalog.Area
	for _, area := range areaOrder {
		for _, e := range m.entries {
			if e.area == area {
				present = append(present, area)
				break
			}
		}
	}
	if n-1 >= len(present) {
		return m, nil
	}
	for i, e := range m.entries {
		if e.area == present[n-1] {
			m.cursor, m.focus = i, focusSidebar
			break
		}
	}
	return m, nil
}

// ---- rendering ----

func (m model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m model) render() string {
	if m.width < minWidth || m.height < minHeight {
		return strong("Terminal too small") + "\n" + muted("Wintonicz needs at least 60×16. Resize the window, or press q to quit.")
	}
	bodyH := m.height - 2
	var body string
	switch {
	case m.flow != nil:
		body = m.modal(m.flow.tool.Title+" · action", m.flow.View, bodyH)
	case m.palette != nil:
		body = m.modal("Search tools", m.palette.View, bodyH)
	case m.help:
		body = m.modal("Keys and symbols", func(w, h int) string { return helpText() }, bodyH)
	default:
		body = m.layout(bodyH)
	}
	return strings.Join([]string{m.header(), body, m.statusBar()}, "\n")
}

func (m model) modal(title string, content func(w, h int) string, bodyH int) string {
	w := min(m.width-4, 80)
	h := min(bodyH, 24)
	return lipgloss.Place(m.width, bodyH, lipgloss.Center, lipgloss.Center, box(title, content(w-4, h-2), w, h, true))
}

func (m model) layout(bodyH int) string {
	sideW := 26
	if m.width < 80 {
		sideW = 20
	}
	detW := 0
	if m.width >= 112 {
		detW = 34
	}
	contentW := m.width - sideW - detW
	detH := 0
	if detW == 0 && m.height >= 28 {
		detH = 8
	}
	contentH := bodyH - detH

	sidebar := box("TOOLS", m.sidebarBody(sideW-4, bodyH-2), sideW, bodyH, m.focus == focusSidebar)
	title, body := "WINTONICZ", ""
	if s := m.activeScreen(); s != nil {
		title, body = s.Title(), s.View(contentW-4, contentH-2)
	}
	content := box(title, body, contentW, contentH, m.focus == focusContent)
	switch {
	case detW > 0:
		details := box("DETAILS", m.detailsBody(detW-4), detW, bodyH, false)
		return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content, details)
	case detH > 0:
		right := lipgloss.JoinVertical(lipgloss.Left, content, box("DETAILS", m.detailsBody(contentW-4), contentW, detH, false))
		return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
}

// sidebarBody draws Home plus one heading per area. Only the area holding the
// selected tool is expanded, so the list stays short however many tools exist.
func (m model) sidebarBody(w, h int) string {
	idx := m.cursor
	if m.focus == focusContent {
		idx = m.active
	}
	var open catalog.Area
	if idx >= 0 && idx < len(m.entries) {
		open = m.entries[idx].area
	}
	counts := map[catalog.Area]int{}
	for _, e := range m.entries {
		counts[e.area]++
	}
	var lines []string
	cursorLine := 0
	var lastArea catalog.Area
	for i, e := range m.entries {
		if e.area != "" && e.area != lastArea {
			lastArea = e.area
			title := fmt.Sprintf("%s %s", map[bool]string{true: "▾", false: "▸"}[e.area == open], strings.ToUpper(string(e.area)))
			count := fmt.Sprintf(" %d", counts[e.area])
			if e.area == open {
				lines = append(lines, accent(fit(title, w-len(count)))+muted(count))
			} else {
				lines = append(lines, muted(fit(title, w-len(count))+count))
			}
		}
		if e.area != "" && e.area != open {
			continue
		}
		badge := "    "
		if e.tool != nil {
			badge = toolBadge(*e.tool)
		}
		label := fit(e.title, max(1, w-7)) + " " + badge
		switch {
		case i == m.cursor && m.focus == focusSidebar:
			cursorLine = len(lines)
			lines = append(lines, selected(ansi.Strip(label), w))
		case i == m.active:
			cursorLine = max(cursorLine, len(lines))
			lines = append(lines, accent("▸ ")+fit(label, w-2))
		default:
			lines = append(lines, "  "+muted2(label))
		}
	}
	start := 0
	if cursorLine >= h {
		start = cursorLine - h + 1
	}
	if start > len(lines) {
		start = 0
	}
	return strings.Join(lines[start:], "\n")
}

func muted2(s string) string { return styleBase.Render(s) }

// toolBadge is the compact, color-independent risk/admin marker: [R] read, [L] launch,
// [C] change, [!] destructive, plus A for administrator.
func toolBadge(t catalog.Tool) string {
	risk := map[catalog.Risk]string{catalog.RiskRead: "R", catalog.RiskLaunch: "L", catalog.RiskChange: "C", catalog.RiskDestructive: "!"}[t.Risk]
	admin := " "
	if t.Admin == catalog.AdminRequired {
		admin = "A"
	}
	return "[" + risk + admin + "]"
}

func (m model) detailsBody(w int) string {
	idx := m.active
	if m.focus == focusSidebar {
		idx = m.cursor
	}
	if idx < 0 || idx >= len(m.entries) {
		return ""
	}
	e := m.entries[idx]
	var parts []string
	if e.tool != nil {
		parts = append(parts, strong(e.tool.Title), wrap(e.tool.Summary, w), "",
			wrap(muted("risk   ")+badges(*e.tool), w), wrap(muted("admin  ")+adminText(*e.tool, m.elevated), w))
	} else {
		parts = append(parts, strong("Home"), wrap("A quick look at this PC: system, resources, storage, and anything that needs attention.", w))
	}
	if m.focus == focusContent {
		if s := m.activeScreen(); s != nil {
			if d := s.Details(w); d != "" {
				parts = append(parts, "", strings.Repeat("─", min(w, 30)), d)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func adminText(t catalog.Tool, elevated bool) string {
	switch t.Admin {
	case catalog.AdminRequired:
		if elevated {
			return "required · you have it"
		}
		return "required · press a to restart as administrator"
	case catalog.AdminOptional:
		if elevated {
			return "optional · you have it"
		}
		return "optional · more data as administrator"
	}
	return "not needed"
}

func (m model) header() string {
	left := accent("Wintonicz") + muted(" "+m.version)
	crumb := "Home"
	if m.active >= 0 && m.active < len(m.entries) {
		e := m.entries[m.active]
		crumb = e.title
		if e.area != "" {
			crumb = strings.ToUpper(string(e.area)[:1]) + string(e.area)[1:] + " › " + e.title
		}
	}
	left += muted("  ·  ") + strong(crumb)
	right := muted("○ standard user")
	if m.elevated {
		right = okText("● administrator")
	}
	right += muted("   ^K search   ? help")
	gap := m.width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return ansi.Truncate(left, m.width, "…")
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m model) statusBar() string {
	if m.notice != "" {
		text := m.notice
		if m.noticeEr {
			return fit(danger("✗ "+text), m.width)
		}
		return fit(accent("● ")+text, m.width)
	}
	var hints []hint
	switch {
	case m.flow != nil:
		hints = m.flow.hints()
	case m.palette != nil:
		hints = []hint{{"↑↓", "move"}, {"enter", "open"}, {"esc", "close"}}
	case m.help:
		hints = []hint{{"any key", "close"}}
	case m.focus == focusSidebar:
		hints = []hint{{"↑↓", "move"}, {"enter", "open"}, {"1-5", "area"}, {"^K", "search"}, {"?", "help"}}
		if !m.elevated {
			hints = append(hints, hint{"a", "admin"})
		}
		hints = append(hints, hint{"q", "quit"})
	default:
		if s := m.activeScreen(); s != nil {
			hints = s.Hints()
		}
		if !m.editing() {
			hints = append(hints, hint{"esc", "tools"}, hint{"?", "help"}, hint{"q", "quit"})
		}
	}
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = strong(h.key) + " " + muted(h.desc)
	}
	return fit(strings.Join(parts, muted("  ·  ")), m.width)
}

func helpText() string {
	return strings.Join([]string{
		strong("Move"), "  ↑↓ or j k   move        enter or → open        esc or ← back to tools",
		"  1–5         jump to an area        tab  switch between tools and content", "",
		strong("Anywhere"), "  ^K  search every tool (try \"no internet\")    ?  this help",
		"  a   restart as administrator (one UAC prompt)    q  quit", "",
		strong("In lists"), "  /  filter    r  refresh    esc  clear filter", "",
		strong("Symbols next to each tool"),
		"  [R ] reads only          [L ] opens a Windows window",
		"  [C ] changes the system  [! ] destructive or hard to undo",
		"  A in the second slot = needs administrator", "",
		muted("Actions always show their exact command first and ask before they run."),
	}, "\n")
}

func loadConfig() config.Config {
	cfg, _ := config.Load() // a damaged file falls back to defaults
	return cfg
}
