package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	quit      key.Binding
	help      key.Binding
	refresh   key.Binding
	overview  key.Binding
	processes key.Binding
	ports     key.Binding
	network   key.Binding
	panels    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.overview, k.processes, k.ports, k.network, k.panels, k.refresh, k.help, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.overview, k.processes, k.ports, k.network, k.panels}, {k.refresh, k.help, k.quit}}
}

type model struct {
	services  app.Services
	keys      keyMap
	help      help.Model
	table     table.Model
	page      page
	processes []processes.Entry
	ports     []ports.Listener
	snapshot  *system.Snapshot
	adapters  []network.Adapter
	panelList []panels.Entry
	width     int
	height    int
	loading   bool
	err       error
	notice    string
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
			quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
			help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
			overview:  key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "overview")),
			processes: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "processes")),
			ports:     key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "ports")),
			network:   key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "network")),
			panels:    key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "panels")),
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
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		m.help.SetWidth(message.Width)
		m.table.SetWidth(max(1, message.Width-2))
		m.table.SetHeight(max(1, message.Height-10))
	case tea.KeyPressMsg:
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
		default:
			updated, command := m.table.Update(message)
			m.table = updated
			return m, command
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7DD3FC")).Render("Wintonicz")
	active := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7DD3FC"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA"))
	labels := []string{"1 Overview", "2 Processes", "3 TCP Ports", "4 Network", "5 Panels"}
	for i := range labels {
		if page(i) == m.page {
			labels[i] = active.Render(labels[i])
		} else {
			labels[i] = muted.Render(labels[i])
		}
	}
	header := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, title, "   ", strings.Join(labels, "   ")),
		muted.Render("Read-only Windows diagnostics"),
	)
	body := m.pageView()
	footer := m.help.View(m.keys)
	content := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)

	view := tea.NewView(content)
	view.AltScreen = true
	return view
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
		rows = make([]table.Row, 0, len(m.processes))
		for _, entry := range m.processes {
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
		table.WithHeight(max(1, m.height-10)),
	)
}

func (m model) pageView() string {
	if m.width < 72 || m.height < 10 {
		return "Resize the terminal to at least 72 columns by 10 rows."
	}
	if m.loading {
		return "Collecting a read-only snapshot..."
	}
	if m.err != nil {
		message := "Could not collect this snapshot: " + m.err.Error()
		return lipgloss.NewStyle().Width(max(32, m.width-2)).Foreground(lipgloss.Color("#FCA5A5")).Render(message)
	}
	if m.page == overviewPage {
		return m.overviewView()
	}
	if m.page == processesPage {
		if len(m.processes) == 0 {
			return "No processes were returned by Windows."
		}
		return m.table.View() + "\n" + m.selectedProcessDetails()
	}
	if len(m.ports) == 0 {
		if m.page == portsPage {
			return "No TCP listeners were found."
		}
	}
	if m.page == networkPage {
		if len(m.adapters) == 0 {
			return "No network adapters were found."
		}
		return m.table.View()
	}
	if m.page == panelsPage {
		if len(m.panelList) == 0 {
			return "No Windows panels are available."
		}
		detail := m.selectedPanelDetails()
		if m.notice != "" {
			detail += "\n" + m.notice
		}
		return m.table.View() + "\n" + detail
	}
	return m.table.View()
}

func (m model) overviewView() string {
	if m.snapshot == nil {
		return "Windows system information is unavailable."
	}
	snapshot := m.snapshot
	return fmt.Sprintf("Host: %s\nWindows: %s (%s, build %s)\nLogical processors: %d\nMemory: %s total, %s available\nUptime: %s\n\nUse the number keys or Tab to browse diagnostic pages.",
		snapshot.Hostname,
		snapshot.ProductName,
		valueOrUnavailable(snapshot.DisplayVersion),
		valueOrUnavailable(snapshot.Build),
		snapshot.LogicalProcessors,
		formatBytes(snapshot.TotalMemoryBytes),
		formatBytes(snapshot.AvailableMemoryBytes),
		snapshot.Uptime,
	)
}

func (m model) selectedPanelDetails() string {
	selected := m.table.SelectedRow()
	if len(selected) == 0 {
		return "Select a panel and press Enter to open it."
	}
	for _, item := range m.panelList {
		if item.Key == selected[0] {
			return item.Description + " Press Enter to open."
		}
	}
	return "Select a panel and press Enter to open it."
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

type panelOpenedMsg struct {
	key string
	err error
}

func valueOrUnavailable(value string) string {
	if value == "" {
		return "unavailable"
	}
	return value
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

func (m model) selectedProcessDetails() string {
	selected := m.table.SelectedRow()
	if len(selected) == 0 {
		return ""
	}
	pid, err := strconv.ParseUint(selected[0], 10, 32)
	if err != nil {
		return ""
	}
	for _, entry := range m.processes {
		if uint64(entry.PID) != pid {
			continue
		}
		path := entry.Path
		if path == "" {
			path = "Executable path unavailable for this process."
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA")).Render("Path: " + path)
	}
	return strings.TrimSpace(fmt.Sprintf("PID %d is no longer in the process snapshot.", pid))
}
