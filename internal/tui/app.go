package tui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/panels"
	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/system"
)

type page int

const (
	homePage page = iota
	diagnosticsMenuPage
	overviewPage
	processesPage
	portsPage
	networkPage
	panelsPage
)

type menuItem struct {
	title       string
	description string
	destination page
}

type keyMap struct {
	quit         key.Binding
	help         key.Binding
	refresh      key.Binding
	search       key.Binding
	connectivity key.Binding
	navigate     key.Binding
	switchTab    key.Binding
	selectTab    key.Binding
	open         key.Binding
	back         key.Binding
	erase        key.Binding
	menu         bool
	editing      bool
	page         page
}

func (k keyMap) ShortHelp() []key.Binding {
	if k.menu {
		bindings := []key.Binding{k.navigate, k.open, k.back}
		if k.page == homePage {
			bindings = append(bindings, k.switchTab, k.selectTab)
		}
		return append(bindings, k.help, k.quit)
	}
	if k.editing {
		return []key.Binding{k.erase, k.open, k.back, k.help, k.quit}
	}
	bindings := []key.Binding{k.back, k.refresh, k.navigate}
	if k.page == processesPage {
		bindings = append(bindings, k.search)
	}
	if k.page == networkPage {
		bindings = append(bindings, k.connectivity)
	}
	if k.page == panelsPage {
		bindings = append(bindings, k.open)
	}
	return append(bindings, k.help, k.quit)
}

func (k keyMap) FullHelp() [][]key.Binding {
	if k.menu {
		if k.page != homePage {
			return [][]key.Binding{{k.navigate, k.open, k.back}, {k.help, k.quit}}
		}
		return [][]key.Binding{{k.switchTab, k.selectTab, k.navigate}, {k.open, k.back, k.help, k.quit}}
	}
	if k.editing {
		return [][]key.Binding{{k.erase, k.open, k.back}, {k.help, k.quit}}
	}
	contextual := []key.Binding{k.refresh}
	if k.page == processesPage {
		contextual = append(contextual, k.search)
	}
	if k.page == networkPage {
		contextual = append(contextual, k.connectivity)
	}
	if k.page == panelsPage {
		contextual = append(contextual, k.open)
	}
	return [][]key.Binding{{k.back, k.navigate}, contextual, {k.help, k.quit}}
}

type model struct {
	services            app.Services
	keys                keyMap
	help                help.Model
	table               table.Model
	page                page
	navigation          []page
	menuSelection       map[page]int
	processes           []processes.Entry
	processFilter       string
	processFilterDraft  string
	processFilterCursor int
	filtering           bool
	connectivityPrompt  bool
	connectivityDraft   string
	connectivityCursor  int
	connectivityRun     bool
	connectivityResult  *network.ConnectivityResult
	connectivityErr     error
	ports               []ports.Listener
	snapshot            *system.Snapshot
	adapters            []network.Adapter
	panelList           []panels.Entry
	width               int
	height              int
	loading             bool
	err                 error
	notice              string
}

type pageLoadedMsg struct {
	page      page
	processes []processes.Entry
	ports     []ports.Listener
	snapshot  *system.Snapshot
	adapters  []network.Adapter
	panelList []panels.Entry
	err       error
}

func newModel(services app.Services) model {
	var panelList []panels.Entry
	if services.Panels != nil {
		panelList = services.Panels.ListPanels()
	}
	return model{
		services: services,
		keys: keyMap{
			quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
			help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
			search:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter processes")),
			connectivity: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "check connectivity")),
			navigate:     key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓", "navigate")),
			switchTab:    key.NewBinding(key.WithKeys("tab", "shift+tab", "left", "right"), key.WithHelp("tab/←/→", "switch tab")),
			selectTab:    key.NewBinding(key.WithKeys("1", "2", "3"), key.WithHelp("1–3", "select tab")),
			open:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open/select")),
			back:         key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc/backspace", "back")),
			erase:        key.NewBinding(key.WithKeys("backspace", "delete"), key.WithHelp("backspace/delete", "edit text")),
		},
		help:          help.New(),
		table:         table.New(table.WithFocused(true), table.WithHeight(18)),
		page:          homePage,
		navigation:    []page{homePage},
		menuSelection: make(map[page]int),
		panelList:     panelList,
	}
}

// Run starts the interactive diagnostic browser.
func Run(services app.Services) error {
	program := tea.NewProgram(newModel(services))
	_, err := program.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case pageLoadedMsg:
		if message.page != m.page {
			return m, nil
		}
		m.loading = false
		m.err = message.err
		if message.err == nil {
			m.processes = message.processes
			m.ports = message.ports
			m.snapshot = message.snapshot
			m.adapters = message.adapters
			m.panelList = message.panelList
			m.setTableRows()
		}
	case panelOpenedMsg:
		if message.err != nil {
			m.notice = "Could not open " + message.key + ": " + message.err.Error()
		} else {
			m.notice = "Opened " + message.key + "."
		}
	case connectivityCheckedMsg:
		m.connectivityRun = false
		m.connectivityErr = message.err
		if message.err == nil {
			m.connectivityResult = &message.result
		}
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		m.help.SetWidth(message.Width)
		m.table.SetWidth(m.tableViewWidth())
		m.table.SetHeight(m.tableHeight())
	case tea.KeyPressMsg:
		if m.connectivityPrompt {
			switch message.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "enter":
				target := strings.TrimSpace(m.connectivityDraft)
				m.connectivityPrompt = false
				m.connectivityRun = true
				m.connectivityResult = nil
				m.connectivityErr = nil
				return m, m.checkConnectivity(target)
			case "esc":
				m.connectivityPrompt = false
				m.connectivityDraft = ""
				m.connectivityCursor = 0
				return m, nil
			default:
				m.connectivityDraft, m.connectivityCursor = editTextDraft(m.connectivityDraft, m.connectivityCursor, message, 255)
				return m, nil
			}
		}

		if m.filtering {
			switch message.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "enter":
				m.processFilter = strings.TrimSpace(m.processFilterDraft)
				m.processFilterDraft = m.processFilter
				m.processFilterCursor = len([]rune(m.processFilterDraft))
				m.filtering = false
				m.setTableRows()
				return m, nil
			case "esc":
				m.processFilterDraft = m.processFilter
				m.processFilterCursor = len([]rune(m.processFilterDraft))
				m.filtering = false
				return m, nil
			default:
				m.processFilterDraft, m.processFilterCursor = editTextDraft(m.processFilterDraft, m.processFilterCursor, message, 80)
				return m, nil
			}
		}

		if isBackNavigationKey(message) {
			return m.goBack()
		}

		switch message.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.help.ShowAll = !m.help.ShowAll
		case "enter":
			if m.page == homePage {
				if m.homeSectionPage() == panelsPage {
					return m, m.openSelectedPanel()
				}
				return m.openSelectedHomeItem()
			}
			if m.page == diagnosticsMenuPage {
				return m.openSelectedMenuItem()
			}
			if m.page == panelsPage {
				return m, m.openSelectedPanel()
			}
		case "r":
			if !isMenuPage(m.page) {
				m.loading = true
				m.err = nil
				return m, m.loadPage()
			}
		case "/":
			if m.page == processesPage {
				m.filtering = true
				m.processFilterDraft = m.processFilter
				m.processFilterCursor = len([]rune(m.processFilterDraft))
			}
		case "c":
			if m.page == networkPage {
				m.connectivityPrompt = true
				m.connectivityDraft = ""
				m.connectivityCursor = 0
			}
		case "tab", "right":
			if m.page == homePage {
				m.moveMenuSelection(1)
				if m.homeSectionPage() == panelsPage {
					m.setTableRows()
				}
				return m, nil
			}
		case "shift+tab", "left":
			if m.page == homePage {
				m.moveMenuSelection(-1)
				if m.homeSectionPage() == panelsPage {
					m.setTableRows()
				}
				return m, nil
			}
		case "1", "2", "3":
			if m.page == homePage {
				selection, _ := strconv.Atoi(message.String())
				m.selectMenuItem(selection - 1)
				if m.homeSectionPage() == panelsPage {
					m.setTableRows()
				}
				return m, nil
			}
		case "up", "k":
			if m.page == homePage && m.homeSectionPage() != panelsPage {
				m.moveMenuSelectionFor(m.homeSectionPage(), -1)
				return m, nil
			}
			if m.page == diagnosticsMenuPage {
				m.moveMenuSelection(-1)
				return m, nil
			}
			updated, command := m.table.Update(message)
			m.table = updated
			return m, command
		case "down", "j":
			if m.page == homePage && m.homeSectionPage() != panelsPage {
				m.moveMenuSelectionFor(m.homeSectionPage(), 1)
				return m, nil
			}
			if m.page == diagnosticsMenuPage {
				m.moveMenuSelection(1)
				return m, nil
			}
			updated, command := m.table.Update(message)
			m.table = updated
			return m, command
		default:
			updated, command := m.table.Update(message)
			m.table = updated
			return m, command
		}
	}

	return m, nil
}

func (m model) activeKeys() keyMap {
	keys := m.keys
	keys.menu = isMenuPage(m.page)
	keys.editing = m.filtering || m.connectivityPrompt
	keys.page = m.page
	if keys.editing {
		keys.back = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
		if m.filtering {
			keys.open = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply filter"))
		} else {
			keys.open = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run check"))
		}
	}
	return keys
}

func (m model) pushPage(next page) (tea.Model, tea.Cmd) {
	if m.page == next {
		return m, nil
	}
	m.navigation = append(m.navigation, next)
	m.page = next
	m.loading = !isMenuPage(next)
	m.err = nil
	m.notice = ""
	if isMenuPage(next) {
		return m, nil
	}
	return m, m.loadPage()
}

func (m model) goBack() (tea.Model, tea.Cmd) {
	if len(m.navigation) <= 1 {
		return m, nil
	}
	m.navigation = m.navigation[:len(m.navigation)-1]
	m.page = m.navigation[len(m.navigation)-1]
	m.loading = false
	m.err = nil
	m.notice = ""
	return m, nil
}

func (m *model) moveMenuSelection(offset int) {
	m.moveMenuSelectionFor(m.page, offset)
}

func (m *model) moveMenuSelectionFor(current page, offset int) {
	items := menuItems(current)
	if current == homePage {
		items = homeSections()
	}
	if len(items) == 0 {
		return
	}
	m.selectMenuItemFor(current, m.menuSelection[current]+offset)
}

func (m *model) selectMenuItem(selection int) {
	m.selectMenuItemFor(m.page, selection)
}

func (m *model) selectMenuItemFor(current page, selection int) {
	items := menuItems(current)
	if current == homePage {
		items = homeSections()
	}
	if len(items) == 0 {
		return
	}
	selection = (selection%len(items) + len(items)) % len(items)
	m.menuSelection[current] = selection
}

func (m model) homeSectionPage() page {
	switch m.menuSelection[homePage] {
	case 1:
		return diagnosticsMenuPage
	case 2:
		return panelsPage
	default:
		return overviewPage
	}
}

func (m model) openSelectedHomeItem() (tea.Model, tea.Cmd) {
	items := menuItems(m.homeSectionPage())
	selection := m.menuSelection[m.homeSectionPage()]
	if selection < 0 || selection >= len(items) {
		return m, nil
	}
	return m.pushPage(items[selection].destination)
}

func (m model) openSelectedMenuItem() (tea.Model, tea.Cmd) {
	items := menuItems(m.page)
	selection := m.menuSelection[m.page]
	if selection < 0 || selection >= len(items) {
		return m, nil
	}
	return m.pushPage(items[selection].destination)
}

func isMenuPage(current page) bool {
	return current == homePage
}

func menuItems(current page) []menuItem {
	switch current {
	case overviewPage:
		return []menuItem{{title: "System overview", description: "Windows version, uptime, CPU, memory, and drive space.", destination: overviewPage}}
	case diagnosticsMenuPage:
		return []menuItem{
			{title: "Processes", description: "Browse running processes and filter by name or PID.", destination: processesPage},
			{title: "TCP ports", description: "Find listening ports and their owning processes.", destination: portsPage},
			{title: "Network", description: "Inspect adapters, gateways, DNS, and connectivity.", destination: networkPage},
		}
	default:
		return nil
	}
}

func homeSections() []menuItem {
	return []menuItem{
		{title: "Overview", description: "System information and resource summary.", destination: overviewPage},
		{title: "Diagnostics", description: "Inspect processes, TCP listeners, and network configuration.", destination: diagnosticsMenuPage},
		{title: "Windows tools", description: "Browse and open built-in Windows settings and management panels.", destination: panelsPage},
	}
}

func (m model) loadPage() tea.Cmd {
	services := m.services
	current := m.page
	return func() tea.Msg {
		result := pageLoadedMsg{page: current}
		switch current {
		case overviewPage:
			var snapshot system.Snapshot
			snapshot, result.err = services.System.SystemSnapshot()
			result.snapshot = &snapshot
		case processesPage:
			result.processes, result.err = services.Processes.ListProcesses()
		case portsPage:
			result.ports, result.err = services.Ports.ListListeners()
		case networkPage:
			result.adapters, result.err = services.Network.NetworkAdapters()
		case panelsPage:
			result.panelList = services.Panels.ListPanels()
		}
		return result
	}
}

func (m *model) setTableRows() {
	var columns []table.Column
	var rows []table.Row

	if m.page == processesPage {
		columns = []table.Column{
			{Title: "PID", Width: 9},
			{Title: "PROCESS", Width: 26},
			{Title: "PARENT", Width: 9},
		}
		visibleProcesses := m.visibleProcesses()
		rows = make([]table.Row, 0, len(visibleProcesses))
		for _, entry := range visibleProcesses {
			rows = append(rows, table.Row{strconv.FormatUint(uint64(entry.PID), 10), entry.Name, strconv.FormatUint(uint64(entry.ParentID), 10)})
		}
	} else if m.page == networkPage {
		columns = []table.Column{
			{Title: "INDEX", Width: 8},
			{Title: "ADAPTER", Width: 24},
			{Title: "STATE", Width: 10},
			{Title: "ADDRESSES", Width: 48},
		}
		rows = make([]table.Row, 0, len(m.adapters))
		for _, adapter := range m.adapters {
			rows = append(rows, table.Row{strconv.Itoa(adapter.Index), adapter.Name, adapter.Status, strings.Join(adapter.Addresses, ", ")})
		}
	} else if m.page == panelsPage || (m.page == homePage && m.homeSectionPage() == panelsPage) {
		columns = []table.Column{
			{Title: "KEY", Width: 24},
			{Title: "WINDOWS PANEL", Width: 32},
		}
		rows = make([]table.Row, 0, len(m.panelList))
		for _, item := range m.panelList {
			rows = append(rows, table.Row{item.Key, item.Name})
		}
	} else if m.page == portsPage {
		columns = []table.Column{
			{Title: "ADDRESS", Width: 25},
			{Title: "PORT", Width: 8},
			{Title: "PID", Width: 9},
			{Title: "PROCESS", Width: 24},
		}
		rows = make([]table.Row, 0, len(m.ports))
		for _, listener := range m.ports {
			name := listener.ProcessName
			if name == "" {
				name = "<unknown>"
			}
			rows = append(rows, table.Row{listener.Address, strconv.Itoa(int(listener.Port)), strconv.FormatUint(uint64(listener.PID), 10), name})
		}
	} else {
		return
	}

	// A new table prevents old rows from being rendered against a new page's
	// column count while the component recalculates its viewport.
	m.table = table.New(
		table.WithFocused(true),
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithWidth(m.tableViewWidth()),
		table.WithHeight(m.tableHeight()),
	)
}

func (m model) tableViewWidth() int {
	if m.width >= 136 {
		return max(96, m.width*70/100)
	}
	return max(1, m.width-2)
}

func (m model) tableHeight() int {
	height := max(1, m.height-10)
	if m.page == homePage && m.homeSectionPage() == panelsPage && m.width < 136 {
		return max(3, height-7)
	}
	if m.page == networkPage {
		return min(height, max(1, len(m.adapters)+1))
	}
	return height
}

func (m model) openSelectedPanel() tea.Cmd {
	selected := m.table.SelectedRow()
	if len(selected) == 0 {
		return nil
	}
	services := m.services
	key := selected[0]
	return func() tea.Msg {
		return panelOpenedMsg{key: key, err: services.Panels.OpenPanel(key)}
	}
}

func (m model) checkConnectivity(target string) tea.Cmd {
	return func() tea.Msg {
		result, err := network.CheckConnectivity(context.Background(), target, 3*time.Second)
		return connectivityCheckedMsg{result: result, err: err}
	}
}

type connectivityCheckedMsg struct {
	result network.ConnectivityResult
	err    error
}

type panelOpenedMsg struct {
	key string
	err error
}
