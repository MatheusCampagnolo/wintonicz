package processes

// Entry is the read-only process information Wintonicz can collect without
// requiring administrator access.
type Entry struct {
	PID      uint32 `json:"pid"`
	ParentID uint32 `json:"parentPid"`
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
}

// Source provides process snapshots from the host operating system.
type Source interface {
	ListProcesses() ([]Entry, error)
}
