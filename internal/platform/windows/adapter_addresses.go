//go:build windows

package windows

import (
	"fmt"
	"runtime"
	"sort"
	"unsafe"

	"golang.org/x/sys/windows"
)

const adapterAddressBufferSize = 15 * 1024

type adapterAddressDetails struct {
	gateways   []string
	dnsServers []string
}

func collectAdapterAddresses() (map[int]adapterAddressDetails, error) {
	// Microsoft recommends a 15 KB initial buffer for typical adapter lists.
	bufferSize := uint32(adapterAddressBufferSize)
	for attempt := 0; attempt < 3; attempt++ {
		buffer := make([]byte, bufferSize)
		addresses := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		actualSize := uint32(len(buffer))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_GATEWAYS, 0, addresses, &actualSize)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			bufferSize = actualSize
			continue
		}
		if err == windows.ERROR_NO_DATA {
			runtime.KeepAlive(buffer)
			return map[int]adapterAddressDetails{}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("GetAdaptersAddresses: %w", err)
		}

		details := make(map[int]adapterAddressDetails)
		for current := addresses; current != nil; current = current.Next {
			gatewayAddresses := collectGatewayAddresses(current.FirstGatewayAddress)
			dnsAddresses := collectDNSServerAddresses(current.FirstDnsServerAddress)
			if current.IfIndex != 0 {
				details[int(current.IfIndex)] = adapterAddressDetails{gateways: gatewayAddresses, dnsServers: dnsAddresses}
			}
			if current.Ipv6IfIndex != 0 {
				details[int(current.Ipv6IfIndex)] = adapterAddressDetails{gateways: gatewayAddresses, dnsServers: dnsAddresses}
			}
		}
		runtime.KeepAlive(buffer)
		return details, nil
	}
	return nil, fmt.Errorf("adapter list changed during collection")
}

func collectGatewayAddresses(first *windows.IpAdapterGatewayAddress) []string {
	var result []string
	for current := first; current != nil; current = current.Next {
		if address := current.Address.IP(); address != nil {
			result = append(result, address.String())
		}
	}
	sort.Strings(result)
	return result
}

func collectDNSServerAddresses(first *windows.IpAdapterDnsServerAdapter) []string {
	var result []string
	for current := first; current != nil; current = current.Next {
		if address := current.Address.IP(); address != nil {
			result = append(result, address.String())
		}
	}
	sort.Strings(result)
	return result
}
