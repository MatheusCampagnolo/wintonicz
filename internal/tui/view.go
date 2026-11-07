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
