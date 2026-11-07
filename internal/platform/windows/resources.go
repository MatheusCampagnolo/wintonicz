//go:build windows

package windows

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/MatheusCampagnolo/wintonicz/internal/resources"
	"golang.org/x/sys/windows"
)

var getSystemTimes = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemTimes")

func (Collector) ResourceSnapshot() (resources.Snapshot, error) {
	const sampleWindow = 500 * time.Millisecond
	idleBefore, kernelBefore, userBefore, err := systemTimes()
	if err != nil {
		return resources.Snapshot{}, err
	}
	started := time.Now()
	time.Sleep(sampleWindow)
	idleAfter, kernelAfter, userAfter, err := systemTimes()
	if err != nil {
		return resources.Snapshot{}, err
	}
	sampleWindowMS := time.Since(started).Milliseconds()

	totalBefore, totalAfter := kernelBefore+userBefore, kernelAfter+userAfter
	if totalAfter < totalBefore || idleAfter < idleBefore {
		return resources.Snapshot{}, fmt.Errorf("CPU counters changed unexpectedly during sample")
	}
	totalDelta, idleDelta := totalAfter-totalBefore, idleAfter-idleBefore
	if totalDelta == 0 || idleDelta > totalDelta {
		return resources.Snapshot{}, fmt.Errorf("Windows returned an invalid CPU sample")
	}

	memory, err := readMemoryStatus()
	if err != nil {
		return resources.Snapshot{}, err
	}
	if memory.AvailPhys > memory.TotalPhys {
		return resources.Snapshot{}, fmt.Errorf("Windows returned inconsistent physical memory counters")
	}
	used := memory.TotalPhys - memory.AvailPhys
	memoryPercent := float64(0)
	if memory.TotalPhys > 0 {
		memoryPercent = float64(used) * 100 / float64(memory.TotalPhys)
	}

	return resources.Snapshot{
		CPUPercent:           float64(totalDelta-idleDelta) * 100 / float64(totalDelta),
		SampleWindowMS:       sampleWindowMS,
		MemoryTotalBytes:     memory.TotalPhys,
		MemoryUsedBytes:      used,
		MemoryAvailableBytes: memory.AvailPhys,
		MemoryPercent:        memoryPercent,
	}, nil
}

func systemTimes() (idle, kernel, user uint64, err error) {
	var idleTime, kernelTime, userTime windows.Filetime
	result, _, callErr := getSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if result == 0 {
		return 0, 0, 0, fmt.Errorf("read Windows CPU times: %w", callErr)
	}
	return filetimeValue(idleTime), filetimeValue(kernelTime), filetimeValue(userTime), nil
}

func filetimeValue(value windows.Filetime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}
