//go:build windows

package windows

import (
	"net"
	"sort"

	"github.com/MatheusCampagnolo/wintonicz/internal/network"
)

func (Collector) NetworkAdapters() ([]network.Adapter, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
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
		result = append(result, network.Adapter{
			Index:     adapter.Index,
			Name:      adapter.Name,
			Status:    status,
			MTU:       max(0, adapter.MTU),
			MAC:       adapter.HardwareAddr.String(),
			Addresses: items,
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Index < result[j].Index })
	return result, nil
}
