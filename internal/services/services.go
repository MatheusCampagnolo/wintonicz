package services

// Entry is a read-only snapshot of a Windows service.
type Entry struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	PID         uint32 `json:"pid,omitempty"`
}

// Source provides Windows service snapshots.
type Source interface {
	ListServices() ([]Entry, error)
}
