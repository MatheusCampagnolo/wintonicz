package storage

// Volume contains capacity information for a Windows logical drive.
type Volume struct {
	Drive     string `json:"drive"`
	Type      string `json:"type"`
	Available bool   `json:"available"`
	Total     uint64 `json:"totalBytes,omitempty"`
	Free      uint64 `json:"freeBytes,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Source provides on-demand volume capacity snapshots.
type Source interface {
	ListVolumes() ([]Volume, error)
}
