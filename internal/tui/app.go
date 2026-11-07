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
	overviewPage page = iota
	processesPage
	portsPage
	networkPage
	panelsPage
)

type keyMap struct {
	quit         key.Binding
	help         key.Binding
	refresh      key.Binding
	search       key.Binding
	connectivity key.Binding
	overview     key.Binding
	processes    key.Binding
	ports        key.Binding
	network      key.Binding
	panels       key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.overview, k.processes, k.ports, k.network, k.panels, k.refresh, k.help, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.overview, k.processes, k.ports, k.network, k.panels}, {k.search, k.connectivity, k.refresh, k.help, k.quit}}
}

type model struct {
	services           app.Services
	keys               keyMap
	help               help.Model
	table              table.Model
	page               page
	processes          []processes.Entry
	processFilter      string
	processFilterDraft string
	filtering          bool
	connectivityPrompt bool
	connectivityDraft  string
	connectivityRun    bool
	connectivityResult *network.ConnectivityResult
	connectivityErr    error
	ports              []ports.Listener
	snapshot           *system.Snapshot
	adapters           []network.Adapter
	panelList          []panels.Entry
	width              int
	height             int
	loading            bool
	err                error
	notice             string
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
	return model{
		services: services,
		keys: keyMap{
			quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
			help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
			search:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter processes")),
			connectivity: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "check connectivity")),
			overview:     key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "overview")),
			processes:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "processes")),
			ports:        key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "ports")),
			network:      key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "network")),
			panels:       key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "panels")),
		},
		help:    help.New(),
		table:   table.New(table.WithFocused(true), table.WithHeight(18)),
		loading: true,
	}
}

// Run starts the interactive diagnostic browser.
func Run(services app.Services) error {
	program := tea.NewProgram(newModel(services))
	_, err := program.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return m.loadPage()
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
		m.table.SetWidth(max(1, message.Width-2))
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
				return m, nil
			default:
				m.connectivityDraft = editTextDraft(m.connectivityDraft, message, 255)
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
				m.filtering = false
				m.setTableRows()
				return m, nil
			case "esc":
				m.processFilterDraft = m.processFilter
				m.filtering = false
				return m, nil
			case "backspace":
			default:
				m.processFilterDraft = editTextDraft(m.processFilterDraft, message, 80)
			}
			return m, nil
		}

		switch message.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.help.ShowAll = !m.help.ShowAll
		case "1":
			return m.switchPage(overviewPage)
		case "2":
			return m.switchPage(processesPage)
		case "3":
			return m.switchPage(portsPage)
		case "4":
			return m.switchPage(networkPage)
		case "5":
			return m.switchPage(panelsPage)
		case "enter":
			if m.page == panelsPage {
				return m, m.openSelectedPanel()
			}
		case "tab":
			return m.switchPage((m.page + 1) % 5)
		case "shift+tab":
			return m.switchPage((m.page + 4) % 5)
		case "r":
			m.loading = true
			m.err = nil
			return m, m.loadPage()
		case "/":
			if m.page == processesPage {
				m.filtering = true
				m.processFilterDraft = m.processFilter
			}
		case "c":
			if m.page == networkPage {
				m.connectivityPrompt = true
				m.connectivityDraft = ""
			}
		default:
			updated, command := m.table.Update(message)
			m.table = updated
			return m, command
		}
	}

	return m, nil
}

func (m model) switchPage(next page) (tea.Model, tea.Cmd) {
	if m.page == next {
		return m, nil
	}
	m.page = next
	m.loading = true
	m.err = nil
	m.notice = ""
	return m, m.loadPage()
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
	} else if m.page == panelsPage {
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
		table.WithWidth(max(1, m.width-2)),
		table.WithHeight(m.tableHeight()),
	)
}

func (m model) tableHeight() int {
	height := max(1, m.height-10)
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
