// Package security reports antivirus and firewall state as Windows Security Center
// exposes it. Third-party products and managed devices can make this stale or
// missing, so every field can be "unknown".
package security

import "fmt"

// Product is a registered antivirus product.
type Product struct {
	Name        string `json:"name"`
	State       string `json:"state"`      // on, off, snoozed, unknown
	Signatures  string `json:"signatures"` // up to date, out of date, unknown
	RawStateHex string `json:"rawState"`
}

// Firewall is one Windows firewall profile.
type Firewall struct {
	Profile string `json:"profile"`
	Enabled bool   `json:"enabled"`
}

// Status is the whole picture.
type Status struct {
	Antivirus []Product  `json:"antivirus"`
	Firewall  []Firewall `json:"firewall"`
}

// Source provides security status.
type Source interface {
	SecurityStatus() (Status, error)
}

// DecodeProductState decodes the Security Center productState value. The layout is
// 0xAABBCC: BB is 0x10/0x11 when the product is on, 0x00/0x01 off, 0x20/0x21 snoozed;
// CC is 0x00 when signatures are current and 0x10 when out of date.
func DecodeProductState(state uint32) (running, signatures string) {
	switch (state >> 8) & 0xFF {
	case 0x10, 0x11:
		running = "on"
	case 0x00, 0x01:
		running = "off"
	case 0x20, 0x21:
		running = "snoozed"
	default:
		running = "unknown"
	}
	switch state & 0xFF {
	case 0x00:
		signatures = "up to date"
	case 0x10:
		signatures = "out of date"
	default:
		signatures = "unknown"
	}
	return running, signatures
}

// NewProduct builds a Product from the raw Security Center values.
func NewProduct(name string, state uint32) Product {
	running, signatures := DecodeProductState(state)
	return Product{Name: name, State: running, Signatures: signatures, RawStateHex: fmt.Sprintf("0x%06X", state)}
}
