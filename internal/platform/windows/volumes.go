//go:build windows

package windows

import (
	"fmt"

	"github.com/MatheusCampagnolo/wintonicz/internal/storage"
	"golang.org/x/sys/windows"
)

func (Collector) ListVolumes() ([]storage.Volume, error) {
	drives, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, fmt.Errorf("list Windows logical drives: %w", err)
	}

	volumes := make([]storage.Volume, 0)
	for letter := byte('A'); letter <= 'Z'; letter++ {
		if drives&(1<<uint(letter-'A')) == 0 {
			continue
		}
		root := string(letter) + ":\\"
		rootPointer, err := windows.UTF16PtrFromString(root)
		if err != nil {
			return nil, fmt.Errorf("encode drive %s: %w", root, err)
		}
		driveType := windows.GetDriveType(rootPointer)
		volume := storage.Volume{Drive: root, Type: volumeType(driveType)}
		var available, total uint64
		if err := windows.GetDiskFreeSpaceEx(rootPointer, &available, &total, nil); err != nil {
			volume.Error = err.Error()
		} else {
			volume.Available = true
			volume.Free = available
			volume.Total = total
		}
		volumes = append(volumes, volume)
	}
	return volumes, nil
}

func volumeType(driveType uint32) string {
	switch driveType {
	case windows.DRIVE_REMOVABLE:
		return "Removable"
	case windows.DRIVE_FIXED:
		return "Fixed"
	case windows.DRIVE_REMOTE:
		return "Network"
	case windows.DRIVE_CDROM:
		return "Optical"
	case windows.DRIVE_RAMDISK:
		return "RAM disk"
	case windows.DRIVE_NO_ROOT_DIR:
		return "Unavailable"
	default:
		return "Unknown"
	}
}
