//go:build windows

package windows

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/MatheusCampagnolo/wintonicz/internal/system"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
var getTickCount64 = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetTickCount64")

func (Collector) SystemSnapshot() (system.Snapshot, error) {
	const currentVersionKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, currentVersionKey, registry.QUERY_VALUE)
	if err != nil {
		return system.Snapshot{}, fmt.Errorf("read Windows version from registry: %w", err)
	}
	defer key.Close()

	productName, _, err := key.GetStringValue("ProductName")
	if err != nil {
		return system.Snapshot{}, fmt.Errorf("read Windows product name: %w", err)
	}
	displayVersion, _, _ := key.GetStringValue("DisplayVersion")
	build, _, _ := key.GetStringValue("CurrentBuildNumber")
	if buildNumber, parseErr := strconv.Atoi(build); parseErr == nil && buildNumber >= 22000 && strings.HasPrefix(productName, "Windows 10") {
		productName = strings.Replace(productName, "Windows 10", "Windows 11", 1)
	}
	hostname, _ := os.Hostname()

	memory := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	result, _, callErr := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memory)))
	if result == 0 {
		return system.Snapshot{}, fmt.Errorf("read physical memory status: %w", callErr)
	}

	uptimeMilliseconds, _, callErr := getTickCount64.Call()
	if callErr != nil && callErr != windows.ERROR_SUCCESS {
		return system.Snapshot{}, fmt.Errorf("read system uptime: %w", callErr)
	}
	drive, driveTotal, driveFree, err := windowsDriveSpace()
	if err != nil {
		return system.Snapshot{}, fmt.Errorf("read Windows drive space: %w", err)
	}

	return system.Snapshot{
		Hostname:             hostname,
		ProductName:          productName,
		DisplayVersion:       displayVersion,
		Build:                build,
		LogicalProcessors:    runtime.NumCPU(),
		TotalMemoryBytes:     memory.TotalPhys,
		AvailableMemoryBytes: memory.AvailPhys,
		WindowsDrive:         drive,
		DriveTotalBytes:      driveTotal,
		DriveFreeBytes:       driveFree,
		Uptime:               (time.Duration(uptimeMilliseconds) * time.Millisecond).Round(time.Second).String(),
	}, nil
}
