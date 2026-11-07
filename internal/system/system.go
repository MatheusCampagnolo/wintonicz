package system

// Snapshot contains basic Windows information gathered without changing state.
type Snapshot struct {
	Hostname             string `json:"hostname"`
	ProductName          string `json:"productName"`
	DisplayVersion       string `json:"displayVersion,omitempty"`
	Build                string `json:"build,omitempty"`
	LogicalProcessors    int    `json:"logicalProcessors"`
	TotalMemoryBytes     uint64 `json:"totalMemoryBytes"`
	AvailableMemoryBytes uint64 `json:"availableMemoryBytes"`
	Uptime               string `json:"uptime"`
}

type Source interface {
	SystemSnapshot() (Snapshot, error)
}
