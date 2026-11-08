// Package diskhealth reports the health the storage stack itself exposes for each
// physical disk. It replaces the removed CHKDSK check: read-only, no administrator.
package diskhealth

// Disk is one physical disk.
type Disk struct {
	Name        string `json:"name"`
	MediaType   string `json:"mediaType"`
	BusType     string `json:"busType"`
	Health      string `json:"health"`
	Operational string `json:"operational"`
	SizeBytes   uint64 `json:"sizeBytes"`
}

// Source provides physical disk health.
type Source interface {
	ListDiskHealth() ([]Disk, error)
}

// Verdict is a short, color-independent reading of Health.
func (d Disk) Verdict() string {
	switch d.Health {
	case "Healthy":
		return "✓ healthy"
	case "Warning":
		return "! warning"
	case "Unhealthy":
		return "✗ unhealthy"
	}
	return "? " + d.Health
}
