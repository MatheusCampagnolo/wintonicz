package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	accentColor       = lipgloss.Color("#7DD3FC")
	accentBorderColor = lipgloss.Color("#38BDF8")
	activeBackground  = lipgloss.Color("#102A3E")
	primaryTextColor  = lipgloss.Color("#CBD5E1")
	strongTextColor   = lipgloss.Color("#E2E8F0")
	mutedTextColor    = lipgloss.Color("#94A3B8")
	borderColor       = lipgloss.Color("#334155")
	errorTextColor    = lipgloss.Color("#FCA5A5")
)

func (m model) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("Wintonicz")
	muted := lipgloss.NewStyle().Foreground(mutedTextColor)
	header := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, title, "   ", m.breadcrumbView()),
		muted.Render("Windows support toolbox"),
	)
	if m.width > 3 {
		divider := lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", min(m.width-2, 72)))
		header = lipgloss.JoinVertical(lipgloss.Left, header, divider)
	}
	body := m.pageView()
	footer := m.help.View(m.activeKeys())
	content := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m model) breadcrumbView() string {
	muted := lipgloss.NewStyle().Foreground(mutedTextColor)
	active := lipgloss.NewStyle().Bold(true).Foreground(strongTextColor)
	parts := make([]string, 0, len(m.navigation)*2)
	for index, item := range m.navigation {
		if index > 0 {
			parts = append(parts, muted.Render("›"))
		}
		label := pageTitle(item)
		if index == len(m.navigation)-1 {
			parts = append(parts, active.Render(label))
		} else {
			parts = append(parts, muted.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m model) pageView() string {
	if m.width < 72 || m.height < 10 {
		return "Resize the terminal to at least 72 columns by 10 rows."
	}
	if m.loading && !isMenuPage(m.page) {
		return "Collecting a read-only snapshot..."
	}
	if m.err != nil {
		message := "Could not collect this snapshot: " + m.err.Error()
		return lipgloss.NewStyle().Width(max(32, m.width-2)).Foreground(errorTextColor).Render(message)
	}
	if isMenuPage(m.page) {
		return m.menuView()
	}
	if m.page == overviewPage {
		return m.overviewView()
	}
	if m.page == processesPage {
		if len(m.processes) == 0 {
			return "No processes were returned by Windows."
		}
		visible := m.visibleProcesses()
		filterStatus := fmt.Sprintf("%d processes", len(m.processes))
		if m.processFilter != "" {
			filterStatus = fmt.Sprintf("Filter: %s (%d matches)", m.processFilter, len(visible))
		} else {
			filterStatus += " · Press / to filter by process name or PID"
		}
		if m.filtering {
			filterStatus = "Edit the filter, then press Enter to apply or Esc to cancel."
		}
		if len(visible) == 0 {
			main := lipgloss.JoinVertical(lipgloss.Left, pageSectionTitle("PROCESS SNAPSHOT"), filterStatus, m.filterView(), "No processes match this filter.")
			return m.detailLayout(main, "FILTER", "Change the filter or clear it to show all processes.")
		}
		main := lipgloss.JoinVertical(lipgloss.Left, pageSectionTitle("PROCESS SNAPSHOT"), filterStatus, m.filterView(), m.table.View())
		return m.detailLayout(main, "SELECTED PROCESS", m.selectedProcessDetails())
	}
	if len(m.ports) == 0 && m.page == portsPage {
		return "No TCP listeners were found."
	}
	if m.page == portsPage {
		main := lipgloss.JoinVertical(lipgloss.Left, pageSectionTitle("TCP LISTENERS"), m.table.View())
		return m.detailLayout(main, "SELECTED LISTENER", m.selectedPortDetails())
	}
	if m.page == networkPage {
		if len(m.adapters) == 0 {
			main := pageSectionTitle("NETWORK ADAPTERS") + "\nNo network adapters were found."
			return m.detailLayout(main, "CONNECTIVITY CHECK", m.networkConnectivityView())
		}
		main := lipgloss.JoinVertical(lipgloss.Left, pageSectionTitle("NETWORK ADAPTERS"), m.table.View())
		details := strings.Join([]string{m.selectedNetworkDetails(), "", m.networkConnectivityView()}, "\n")
		return m.detailLayout(main, "ADAPTER DETAILS & CONNECTIVITY", details)
	}
	if m.page == servicesPage {
		if len(m.serviceList) == 0 {
			return "No Windows services were found."
		}
		main := lipgloss.JoinVertical(lipgloss.Left, pageSectionTitle("WINDOWS SERVICES · READ ONLY"), m.table.View())
		return m.detailLayout(main, "SELECTED SERVICE", m.selectedServiceDetails())
	}
	if m.page == usbPage {
		if len(m.usbDevices) == 0 {
			return "No connected USB devices were found."
		}
		main := lipgloss.JoinVertical(lipgloss.Left, pageSectionTitle("CONNECTED USB DEVICES · READ ONLY"), m.table.View())
		return m.detailLayout(main, "SELECTED DEVICE", m.selectedUSBDetails())
	}
	if m.page == panelsPage {
		if len(m.panelList) == 0 {
			return "No Windows panels are available."
		}
		detail := m.selectedPanelDetails()
		if m.notice != "" {
			detail += "\n" + m.notice
		}
		main := lipgloss.JoinVertical(lipgloss.Left, pageSectionTitle("AVAILABLE WINDOWS PANELS"), m.table.View())
		return m.detailLayout(main, "PANEL DETAILS", detail)
	}
	return m.table.View()
}

func pageSectionTitle(value string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(value)
}

func (m model) detailLayout(main, title, details string) string {
	if m.width >= 136 {
		sidebarWidth := max(32, m.width-m.tableViewWidth()-4)
		return lipgloss.JoinHorizontal(lipgloss.Top, main, " ", renderPanel(title, details, sidebarWidth, true))
	}
	return lipgloss.JoinVertical(lipgloss.Left, main, "", renderPanel(title, details, max(40, m.width-2), true))
}

func (m model) menuView() string {
	items := homeSections()
	contentWidth := min(max(64, m.width-4), 112)
	selectedIndex := m.menuSelection[m.page]
	muted := lipgloss.NewStyle().Foreground(mutedTextColor)
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(strongTextColor)

	tabs := make([]string, 0, len(items)*2-1)
	tabWidth := (contentWidth - len(items) + 1) / len(items)
	for index, item := range items {
		style := lipgloss.NewStyle().Width(tabWidth).Padding(0, 1).Border(lipgloss.RoundedBorder())
		if index == selectedIndex {
			style = style.Bold(true).Foreground(accentColor).Background(activeBackground).BorderForeground(accentBorderColor)
		} else {
			style = style.Foreground(primaryTextColor).BorderForeground(borderColor)
		}
		tabs = append(tabs, style.Render(fmt.Sprintf("%d  %s", index+1, menuTabLabel(item))))
		if index < len(items)-1 {
			tabs = append(tabs, " ")
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	section := m.homeSectionPage()
	if section == panelsPage {
		if len(m.panelList) == 0 {
			return lipgloss.JoinVertical(lipgloss.Left, tabBar, "", "No Windows panels are available.")
		}
		details := m.selectedPanelDetails()
		if m.notice != "" {
			details += "\n" + m.notice
		}
		main := lipgloss.JoinVertical(lipgloss.Left, pageSectionTitle("WINDOWS PANELS"), m.table.View())
		return lipgloss.JoinVertical(lipgloss.Left,
			muted.Render("Tab changes area · ↑/↓ selects a panel · Enter opens it."),
			tabBar,
			"",
			m.detailLayout(main, "PANEL DETAILS", details),
		)
	}

	features := menuItems(section)
	if len(features) == 0 {
		return "No tools are available in this section."
	}
	selectedIndex = m.menuSelection[section]
	if selectedIndex < 0 || selectedIndex >= len(features) {
		selectedIndex = 0
	}
	selectedItem := features[selectedIndex]
	labels := make([]string, 0, len(features))
	for _, item := range features {
		labels = append(labels, item.title)
	}
	leftWidth := (contentWidth - 1) / 2
	rightWidth := contentWidth - leftWidth - 1
	details := strings.Join([]string{
		selectedItem.description,
		"",
		strings.Join(m.menuPreview(selectedItem), "\n"),
		"",
		muted.Render("Press Enter to open this tool."),
	}, "\n")
	leftPanel := renderPanel("TOOLS", renderSelectableList(labels, selectedIndex, leftWidth-4), leftWidth, true)
	rightPanel := renderPanel(selectedItem.title, details, rightWidth, false)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)
	if m.width < 96 {
		panels = lipgloss.JoinVertical(lipgloss.Left, leftPanel, "", rightPanel)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		sectionTitle.Render(strings.ToUpper(pageTitle(section))),
		muted.Render("Tab changes area · ↑/↓ selects a tool · Enter opens it."),
		tabBar,
		"",
		panels,
	)
}

func renderSelectableList(labels []string, selected, width int) string {
	rows := make([]string, 0, len(labels))
	for index, label := range labels {
		prefix := "  "
		style := lipgloss.NewStyle().Width(max(1, width)).Padding(0, 1).Foreground(primaryTextColor)
		if index == selected {
			prefix = "› "
			style = style.Bold(true).Foreground(accentColor).Background(activeBackground)
		}
		rows = append(rows, style.Render(prefix+label))
	}
	return strings.Join(rows, "\n")
}

func renderPanel(title, content string, width int, highlighted bool) string {
	panelBorderColor := borderColor
	titleColor := mutedTextColor
	if highlighted {
		panelBorderColor = accentBorderColor
		titleColor = accentColor
	}
	label := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(title)
	style := lipgloss.NewStyle().Width(width).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(panelBorderColor)
	return style.Render(label + "\n" + content)
}

func menuTabLabel(item menuItem) string {
	switch item.destination {
	case overviewPage:
		return "Overview"
	case diagnosticsMenuPage:
		return "Diagnostics"
	case processesPage:
		return "Processes"
	case portsPage:
		return "TCP ports"
	case networkPage:
		return "Network"
	case servicesPage:
		return "Services"
	case usbPage:
		return "USB devices"
	case panelsPage:
		return "Windows tools"
	default:
		return item.title
	}
}

func (m model) menuPreview(item menuItem) []string {
	var entries []string
	switch item.destination {
	case overviewPage:
		entries = []string{"Windows edition and build", "Processor and memory summary", "Uptime and system drive capacity"}
	case diagnosticsMenuPage:
		entries = []string{"Process snapshots and parent PIDs", "TCP listeners and process owners", "Adapters, gateway, DNS, and connectivity", "Windows service state and process IDs", "Currently present USB devices"}
	case panelsPage:
		entries = []string{"Computer and device management", "Network, Power, Sound, and Printers", "Security, Firewall, and System Restore"}
	case processesPage:
		entries = []string{"Filter by process name or PID", "Inspect parent PID and executable path", "Read-only process snapshot"}
	case portsPage:
		entries = []string{"Find TCP listening endpoints", "See PID and process owner", "Search from a port or a process"}
	case networkPage:
		entries = []string{"Inspect addresses, gateways, and DNS", "Check an explicit host:port", "No connection is made until requested"}
	case servicesPage:
		entries = []string{"Browse service names and display names", "Inspect current service state and PID", "Read-only inventory; no service controls"}
	case usbPage:
		entries = []string{"Show currently present USB devices", "Display name, manufacturer, class, and location", "No resident monitoring or device changes"}
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, "•  "+entry)
	}
	return lines
}

func pageTitle(current page) string {
	switch current {
	case homePage:
		return "Home"
	case diagnosticsMenuPage:
		return "Diagnostics"
	case overviewPage:
		return "System overview"
	case processesPage:
		return "Processes"
	case portsPage:
		return "TCP ports"
	case networkPage:
		return "Network"
	case servicesPage:
		return "Services"
	case panelsPage:
		return "Windows tools"
	default:
		return "Wintonicz"
	}
}

func (m model) overviewView() string {
	if m.snapshot == nil {
		return "Windows system information is unavailable."
	}
	snapshot := m.snapshot
	driveSummary := "unavailable"
	if snapshot.WindowsDrive != "" {
		driveSummary = fmt.Sprintf("%s (%s free of %s)", snapshot.WindowsDrive, formatBytes(snapshot.DriveFreeBytes), formatBytes(snapshot.DriveTotalBytes))
	}
	windows := strings.Join([]string{
		"Product: " + valueOrUnavailable(snapshot.ProductName),
		"Version: " + valueOrUnavailable(snapshot.DisplayVersion),
		"Build: " + valueOrUnavailable(snapshot.Build),
		"Host: " + snapshot.Hostname,
	}, "\n")
	resources := strings.Join([]string{
		fmt.Sprintf("Logical processors: %d", snapshot.LogicalProcessors),
		fmt.Sprintf("Memory: %s total · %s available", formatBytes(snapshot.TotalMemoryBytes), formatBytes(snapshot.AvailableMemoryBytes)),
		"Windows drive: " + driveSummary,
		"Uptime: " + snapshot.Uptime,
	}, "\n")
	width := max(32, (m.width-5)/2)
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			renderPanel("WINDOWS", windows, width, true),
			" ",
			renderPanel("SYSTEM RESOURCES", resources, width, false),
		),
		"",
		lipgloss.NewStyle().Foreground(mutedTextColor).Render("Read-only system snapshot · Press Esc or Backspace to return."),
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

func (m model) selectedServiceDetails() string {
	selected := m.table.SelectedRow()
	if len(selected) < 4 {
		return "Select a service to inspect its current state and process ID."
	}
	return strings.Join([]string{
		"Name: " + selected[0],
		"Display name: " + selected[1],
		"State: " + selected[2],
		"PID: " + selected[3],
		"Windows may omit services this account cannot query.",
	}, "\n")
}

func (m model) selectedUSBDetails() string {
	selected := m.table.SelectedRow()
	if len(selected) < 4 {
		return "No connected USB device is selected."
	}
	return strings.Join([]string{
		"Device: " + selected[0],
		"Manufacturer: " + selected[1],
		"Class: " + selected[2],
		"Location: " + selected[3],
		"Only currently present devices are listed.",
	}, "\n")
}

func (m model) selectedNetworkDetails() string {
	selected := m.table.SelectedRow()
	if len(selected) == 0 {
		return "Select a network adapter to see its gateway and DNS servers."
	}
	index, err := strconv.Atoi(selected[0])
	if err != nil {
		return "Selected network adapter details are unavailable."
	}
	for _, adapter := range m.adapters {
		if adapter.Index != index {
			continue
		}
		mtu := "unavailable"
		if adapter.MTU > 0 {
			mtu = strconv.Itoa(adapter.MTU)
		}
		return fmt.Sprintf("MTU: %s · MAC: %s\nGateway: %s\nDNS servers: %s",
			mtu,
			valueOrUnavailable(adapter.MAC),
			joinOrUnavailable(adapter.Gateways),
			joinOrUnavailable(adapter.DNSServers),
		)
	}
	return "Selected network adapter details are unavailable."
}

func (m model) networkConnectivityView() string {
	if m.connectivityPrompt {
		return "Enter a host:port (for example, example.com:443).\nTarget: " + renderTextDraft(m.connectivityDraft, m.connectivityCursor)
	}
	if m.connectivityRun {
		return "Checking DNS resolution and TCP reachability..."
	}
	if m.connectivityErr != nil {
		message := "Connectivity check could not run: " + m.connectivityErr.Error()
		return message
	}
	if m.connectivityResult == nil {
		return "Press c to check connectivity to an explicit host:port."
	}

	result := m.connectivityResult
	resolution := "failed"
	if result.TargetResolved {
		resolution = "resolved to " + strings.Join(result.ResolvedAddresses, ", ")
	}
	tcpStatus := "unreachable"
	if result.TCPConnected {
		tcpStatus = "reachable"
	}
	message := strings.Join([]string{
		"Connectivity check",
		"Target: " + result.Target,
		"Resolution: " + resolution,
		fmt.Sprintf("TCP connection: %s (%d ms)", tcpStatus, result.DurationMillis),
	}, "\n")
	if result.FailureReason != "" {
		message += "\nDetails: " + result.FailureReason
	}
	return message
}

func (m model) selectedPortDetails() string {
	selected := m.table.SelectedRow()
	if len(selected) < 4 {
		return "Select a TCP listener to inspect its address and owning process."
	}
	return strings.Join([]string{
		"Address: " + selected[0],
		"Port: " + selected[1],
		"Process: " + selected[3],
		"PID: " + selected[2],
	}, "\n")
}

func joinOrUnavailable(values []string) string {
	if len(values) == 0 {
		return "not configured"
	}
	return strings.Join(values, ", ")
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
		return lipgloss.NewStyle().Foreground(mutedTextColor).Render("Path: " + path)
	}
	return fmt.Sprintf("PID %d is no longer in the process snapshot.", pid)
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

func (m model) filterView() string {
	if !m.filtering {
		return ""
	}
	return "Filter: " + renderTextDraft(m.processFilterDraft, m.processFilterCursor)
}

func renderTextDraft(value string, cursor int) string {
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	if cursor == len(runes) {
		return string(runes) + cursorStyle.Render("▏")
	}
	cursorStyle = cursorStyle.Foreground(strongTextColor).Background(accentColor)
	return string(runes[:cursor]) + cursorStyle.Render(string(runes[cursor])) + string(runes[cursor+1:])
}
