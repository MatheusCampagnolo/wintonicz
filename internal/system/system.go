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
	WindowsDrive         string `json:"windowsDrive"`
	// DriveTotalBytes is the capacity available to the current user, respecting disk quotas.
	DriveTotalBytes uint64 `json:"driveTotalBytes"`
	// DriveFreeBytes is the free space available to the current user, respecting disk quotas.
	DriveFreeBytes uint64 `json:"driveFreeBytes"`
	Uptime         string `json:"uptime"`
}

type Source interface {
	SystemSnapshot() (Snapshot, error)
}
