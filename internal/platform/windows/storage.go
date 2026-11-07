//go:build windows

package windows

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func windowsDriveSpace() (string, uint64, uint64, error) {
	directory, err := windows.GetWindowsDirectory()
	if err != nil {
		return "", 0, 0, fmt.Errorf("find Windows directory: %w", err)
	}

	drive := filepath.VolumeName(directory)
	if drive == "" {
		return "", 0, 0, fmt.Errorf("Windows directory %q has no drive name", directory)
	}
	directoryPointer, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return "", 0, 0, fmt.Errorf("encode Windows directory: %w", err)
	}

	var freeAvailableToCaller, totalAvailableToCaller uint64
	if err := windows.GetDiskFreeSpaceEx(directoryPointer, &freeAvailableToCaller, &totalAvailableToCaller, nil); err != nil {
		return "", 0, 0, fmt.Errorf("query %s: %w", drive, err)
	}
	return drive, totalAvailableToCaller, freeAvailableToCaller, nil
}
