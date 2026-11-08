//go:build windows

package maint

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	procQueryRecycleBin = shell32.NewProc("SHQueryRecycleBinW")
	procEmptyRecycleBin = shell32.NewProc("SHEmptyRecycleBinW")
)

type recycleBinInfoStruct struct {
	cbSize    uint32
	sizeBytes int64
	numItems  int64
}

func recycleBinInfo() (items, size int64, err error) {
	info := recycleBinInfoStruct{}
	info.cbSize = uint32(unsafe.Sizeof(info))
	hr, _, _ := procQueryRecycleBin.Call(0, uintptr(unsafe.Pointer(&info)))
	if hr != 0 {
		return 0, 0, fmt.Errorf("SHQueryRecycleBin returned 0x%X", hr)
	}
	return info.numItems, info.sizeBytes, nil
}

func emptyRecycleBin() error {
	const noConfirmation, noProgressUI, noSound = 0x1, 0x2, 0x4
	hr, _, _ := procEmptyRecycleBin.Call(0, 0, noConfirmation|noProgressUI|noSound)
	// S_OK, or S_FALSE (0x1) when the bin was already empty.
	if hr != 0 && hr != 1 {
		return fmt.Errorf("SHEmptyRecycleBin returned 0x%X", hr)
	}
	return nil
}

func terminateProcess(pid uint32) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("open process %d (access may be denied): %w", pid, err)
	}
	defer windows.CloseHandle(handle)
	if err := windows.TerminateProcess(handle, 1); err != nil {
		return fmt.Errorf("end process %d: %w", pid, err)
	}
	return nil
}
