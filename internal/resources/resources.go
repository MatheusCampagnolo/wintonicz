package resources

// Snapshot contains CPU utilization measured across a short sample window and
// physical memory usage observed at the end of that window.
type Snapshot struct {
	CPUPercent           float64 `json:"cpuPercent"`
	SampleWindowMS       int64   `json:"sampleWindowMs"`
	MemoryTotalBytes     uint64  `json:"memoryTotalBytes"`
	MemoryUsedBytes      uint64  `json:"memoryUsedBytes"`
	MemoryAvailableBytes uint64  `json:"memoryAvailableBytes"`
	MemoryPercent        float64 `json:"memoryPercent"`
}

// Source provides on-demand resource snapshots.
type Source interface {
	ResourceSnapshot() (Snapshot, error)
}
