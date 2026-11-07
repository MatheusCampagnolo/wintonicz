//go:build windows

package windows

import (
	"fmt"
	"net"
	"sort"

	"github.com/MatheusCampagnolo/wintonicz/internal/network"
)

func (Collector) NetworkAdapters() ([]network.Adapter, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	adapterDetails, err := collectAdapterAddresses()
	if err != nil {
		return nil, fmt.Errorf("read adapter gateway and DNS addresses: %w", err)
	}

	result := make([]network.Adapter, 0, len(interfaces))
	for _, adapter := range interfaces {
		addresses, addressErr := adapter.Addrs()
		if addressErr != nil {
			return nil, addressErr
		}
		status := "down"
		if adapter.Flags&net.FlagUp != 0 {
			status = "up"
		}
		items := make([]string, 0, len(addresses))
		for _, address := range addresses {
			items = append(items, address.String())
		}
		sort.Strings(items)
		details := adapterDetails[adapter.Index]
		gateways := append([]string{}, details.gateways...)
		dnsServers := append([]string{}, details.dnsServers...)
		result = append(result, network.Adapter{
			Index:      adapter.Index,
			Name:       adapter.Name,
			Status:     status,
			MTU:        max(0, adapter.MTU),
			MAC:        adapter.HardwareAddr.String(),
			Addresses:  items,
			Gateways:   gateways,
			DNSServers: dnsServers,
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Index < result[j].Index })
	return result, nil
}
