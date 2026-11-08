//go:build windows

package windows

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"

	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"golang.org/x/sys/windows"
)

var (
	iphlpapi         = windows.NewLazySystemDLL("iphlpapi.dll")
	procIcmpCreate   = iphlpapi.NewProc("IcmpCreateFile")
	procIcmpClose    = iphlpapi.NewProc("IcmpCloseHandle")
	procIcmpSendEcho = iphlpapi.NewProc("IcmpSendEcho")
)

var icmpStatus = map[uint32]string{
	0: "reply", 11003: "network unreachable", 11004: "host unreachable", 11010: "request timed out", 11013: "TTL expired in transit",
}

// Ping sends one ICMP echo through the Windows ICMP helper (no raw sockets, no administrator).
func (Collector) Ping(ip net.IP, timeout time.Duration) (network.PingResult, error) {
	v4 := ip.To4()
	if v4 == nil {
		return network.PingResult{}, fmt.Errorf("ping supports IPv4 only")
	}
	result := network.PingResult{Address: v4.String()}
	handle, _, err := procIcmpCreate.Call()
	if handle == 0 || handle == uintptr(windows.InvalidHandle) {
		return result, fmt.Errorf("IcmpCreateFile: %w", err)
	}
	defer procIcmpClose.Call(handle)

	payload := []byte("wintonicz-ping-0123456789")
	reply := make([]byte, 512)
	n, _, callErr := procIcmpSendEcho.Call(handle, uintptr(binary.LittleEndian.Uint32(v4)),
		uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0,
		uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), uintptr(timeout.Milliseconds()))
	if n == 0 {
		var code uint32
		if errno, ok := callErr.(syscall.Errno); ok {
			code = uint32(errno)
		}
		result.Status = icmpStatus[code]
		if result.Status == "" {
			result.Status = fmt.Sprintf("error %d", code)
		}
		return result, nil
	}
	// ICMP_ECHO_REPLY: Address u32, Status u32, RoundTripTime u32.
	status := binary.LittleEndian.Uint32(reply[4:8])
	result.RTTMillis = int64(binary.LittleEndian.Uint32(reply[8:12]))
	result.Status = icmpStatus[status]
	if result.Status == "" {
		result.Status = fmt.Sprintf("status %d", status)
	}
	result.OK = status == 0
	return result, nil
}
