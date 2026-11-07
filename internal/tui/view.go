package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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
		visible := m.visibleProcesses()
		filterStatus := fmt.Sprintf("%d processes", len(m.processes))
		if m.processFilter != "" {
			filterStatus = fmt.Sprintf("Filter: %s (%d matches)", m.processFilter, len(visible))
		} else {
			filterStatus += " · Press / to filter by process name or PID"
		}
		if m.filtering {
			filterStatus = "Type a name or PID, then press Enter to apply or Esc to cancel."
		}
		if len(visible) == 0 {
			return filterStatus + "\n" + m.filterView() + "No processes match this filter."
		}
		return filterStatus + "\n" + m.filterView() + m.table.View() + "\n" + m.selectedProcessDetails()
	}
	if len(m.ports) == 0 && m.page == portsPage {
		return "No TCP listeners were found."
	}
	if m.page == networkPage {
		if len(m.adapters) == 0 {
			return "No network adapters were found.\n\n" + m.networkConnectivityView()
		}
		return m.table.View() + "\n" + m.selectedNetworkDetails() + "\n\n" + m.networkConnectivityView()
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
	driveSummary := "unavailable"
	if snapshot.WindowsDrive != "" {
		driveSummary = fmt.Sprintf("%s (%s free of %s)", snapshot.WindowsDrive, formatBytes(snapshot.DriveFreeBytes), formatBytes(snapshot.DriveTotalBytes))
	}
	return fmt.Sprintf("Host: %s\nWindows: %s (%s, build %s)\nLogical processors: %d\nMemory: %s total, %s available\nWindows drive: %s\nUptime: %s\n\nUse the number keys or Tab to browse diagnostic pages.",
		snapshot.Hostname,
		snapshot.ProductName,
		valueOrUnavailable(snapshot.DisplayVersion),
		valueOrUnavailable(snapshot.Build),
		snapshot.LogicalProcessors,
		formatBytes(snapshot.TotalMemoryBytes),
		formatBytes(snapshot.AvailableMemoryBytes),
		driveSummary,
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
		return lipgloss.NewStyle().Width(max(32, m.width-2)).Render(fmt.Sprintf("MTU: %s · MAC: %s\nGateway: %s\nDNS servers: %s",
			mtu,
			valueOrUnavailable(adapter.MAC),
			joinOrUnavailable(adapter.Gateways),
			joinOrUnavailable(adapter.DNSServers),
		))
	}
	return "Selected network adapter details are unavailable."
}

func (m model) networkConnectivityView() string {
	if m.connectivityPrompt {
		prompt := "Connectivity check — enter host:port (for example, example.com:443); Enter runs it, Esc cancels.\nTarget: " + m.connectivityDraft + "▏"
		return lipgloss.NewStyle().Width(max(32, m.width-2)).Render(prompt)
	}
	if m.connectivityRun {
		return "Checking DNS resolution and TCP reachability..."
	}
	if m.connectivityErr != nil {
		message := "Connectivity check could not run: " + m.connectivityErr.Error()
		return lipgloss.NewStyle().Width(max(32, m.width-2)).Render(message)
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
	return lipgloss.NewStyle().Width(max(32, m.width-2)).Render(message)
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
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA")).Render("Path: " + path)
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
	return "Filter: " + m.processFilterDraft + "▏\n"
}
