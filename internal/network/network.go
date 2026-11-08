package network

// Adapter is a read-only summary of a network interface and its address configuration.
type Adapter struct {
	Index       int      `json:"index"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	MTU         int      `json:"mtu,omitempty"`
	MAC         string   `json:"mac,omitempty"`
	Addresses   []string `json:"addresses"`
	Gateways    []string `json:"gateways"`
	DNSServers  []string `json:"dnsServers"`
	DHCPEnabled bool     `json:"dhcpEnabled"`
	DHCPServer  string   `json:"dhcpServer,omitempty"`
	DNSSuffix   string   `json:"dnsSuffix,omitempty"`
	SpeedMbps   uint64   `json:"speedMbps,omitempty"`
}

type Source interface {
	NetworkAdapters() ([]Adapter, error)
}
