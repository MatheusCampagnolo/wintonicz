package ports

// Listener describes a local TCP endpoint in the LISTEN state.
type Listener struct {
	Address     string `json:"address"`
	Port        uint16 `json:"port"`
	PID         uint32 `json:"pid"`
	ProcessName string `json:"processName,omitempty"`
}

// Source provides TCP listener snapshots from the host operating system.
type Source interface {
	ListListeners() ([]Listener, error)
}
