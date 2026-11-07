package usb

// Device is a connected USB device visible to the current Windows account.
type Device struct {
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Class        string `json:"class,omitempty"`
	Location     string `json:"location,omitempty"`
}

// Source provides on-demand USB device snapshots.
type Source interface {
	ListConnectedDevices() ([]Device, error)
}
