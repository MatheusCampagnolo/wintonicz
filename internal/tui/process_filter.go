package tui

import (
	"strconv"
	"strings"

	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
)

func (m model) visibleProcesses() []processes.Entry {
	if m.processFilter == "" {
		return m.processes
	}

	query := strings.ToLower(m.processFilter)
	visible := make([]processes.Entry, 0, len(m.processes))
	for _, entry := range m.processes {
		pid := strconv.FormatUint(uint64(entry.PID), 10)
		if strings.Contains(strings.ToLower(entry.Name), query) || strings.Contains(pid, query) {
			visible = append(visible, entry)
		}
	}
	return visible
}
