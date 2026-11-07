//go:build windows

package windows

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"syscall"
	"unsafe"

	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"golang.org/x/sys/windows"
)

const (
	addressFamilyIPv4        = 2
	addressFamilyIPv6        = 23
	tcpTableOwnerPIDListener = 3
	tcpStateListen           = 2
)

var getExtendedTCPTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

type Collector struct{}

func (Collector) ListProcesses() ([]processes.Entry, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("create process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return []processes.Entry{}, nil
		}
		return nil, fmt.Errorf("read process snapshot: %w", err)
	}

	result := make([]processes.Entry, 0, 64)
	for {
		result = append(result, processes.Entry{
			PID:      entry.ProcessID,
			ParentID: entry.ParentProcessID,
			Name:     windows.UTF16ToString(entry.ExeFile[:]),
			Path:     processPath(entry.ProcessID),
		})

		err := windows.Process32Next(snapshot, &entry)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("continue process snapshot: %w", err)
		}
	}

	return result, nil
}

func processPath(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buffer[:size])
}

func (Collector) ListListeners() ([]ports.Listener, error) {
	processNames := make(map[uint32]string)
	if entries, err := (Collector{}).ListProcesses(); err == nil {
		for _, entry := range entries {
			processNames[entry.PID] = entry.Name
		}
	}

	listeners := make([]ports.Listener, 0)
	for _, family := range []uint32{addressFamilyIPv4, addressFamilyIPv6} {
		rows, err := tcpListeners(family)
		if err != nil {
			return nil, fmt.Errorf("read TCP listeners for address family %d: %w", family, err)
		}
		for _, row := range rows {
			row.ProcessName = processNames[row.PID]
			listeners = append(listeners, row)
		}
	}

	return listeners, nil
}

func tcpListeners(family uint32) ([]ports.Listener, error) {
	var size uint32
	status, _, _ := getExtendedTCPTable.Call(
		0,
		uintptr(unsafe.Pointer(&size)),
		0,
		uintptr(family),
		tcpTableOwnerPIDListener,
		0,
	)
	if status != uintptr(syscall.Errno(122)) && status != 0 {
		return nil, syscall.Errno(status)
	}
	if size < 4 {
		return []ports.Listener{}, nil
	}

	for attempt := 0; attempt < 3; attempt++ {
		buffer := make([]byte, size)
		status, _, _ = getExtendedTCPTable.Call(
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(unsafe.Pointer(&size)),
			0,
			uintptr(family),
			tcpTableOwnerPIDListener,
			0,
		)
		if status == uintptr(syscall.Errno(122)) {
			continue
		}
		if status != 0 {
			return nil, syscall.Errno(status)
		}
		if len(buffer) < 4 {
			return nil, errors.New("Windows returned an incomplete TCP table")
		}

		count := int(binary.LittleEndian.Uint32(buffer[:4]))
		if family == addressFamilyIPv4 {
			return parseIPv4Listeners(buffer[4:], count), nil
		}
		return parseIPv6Listeners(buffer[4:], count), nil
	}

	return nil, errors.New("TCP listener table changed repeatedly while reading it")
}

func parseIPv4Listeners(buffer []byte, count int) []ports.Listener {
	const rowSize = 24
	if count > len(buffer)/rowSize {
		count = len(buffer) / rowSize
	}
	rows := make([]ports.Listener, 0, count)
	for i := 0; i < count; i++ {
		row := buffer[i*rowSize : (i+1)*rowSize]
		if binary.LittleEndian.Uint32(row[0:4]) != tcpStateListen {
			continue
		}
		addr := binary.LittleEndian.Uint32(row[4:8])
		ip := [4]byte{byte(addr), byte(addr >> 8), byte(addr >> 16), byte(addr >> 24)}
		rows = append(rows, ports.Listener{
			Address: netip.AddrFrom4(ip).String(),
			Port:    windows.Ntohs(uint16(binary.LittleEndian.Uint32(row[8:12]))),
			PID:     binary.LittleEndian.Uint32(row[20:24]),
		})
	}
	return rows
}

func parseIPv6Listeners(buffer []byte, count int) []ports.Listener {
	const rowSize = 56
	if count > len(buffer)/rowSize {
		count = len(buffer) / rowSize
	}
	rows := make([]ports.Listener, 0, count)
	for i := 0; i < count; i++ {
		row := buffer[i*rowSize : (i+1)*rowSize]
		if binary.LittleEndian.Uint32(row[48:52]) != tcpStateListen {
			continue
		}
		var ip [16]byte
		copy(ip[:], row[:16])
		address := netip.AddrFrom16(ip)
		scopeID := binary.LittleEndian.Uint32(row[16:20])
		if scopeID != 0 {
			address = address.WithZone(fmt.Sprint(scopeID))
		}
		rows = append(rows, ports.Listener{
			Address: address.String(),
			Port:    windows.Ntohs(uint16(binary.LittleEndian.Uint32(row[20:24]))),
			PID:     binary.LittleEndian.Uint32(row[52:56]),
		})
	}
	return rows
}
