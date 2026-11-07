//go:build windows

package windows

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/MatheusCampagnolo/wintonicz/internal/services"
	"golang.org/x/sys/windows"
)

func (Collector) ListServices() ([]services.Entry, error) {
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return nil, fmt.Errorf("connect to Windows service manager: %w", err)
	}
	defer windows.CloseServiceHandle(manager)

	var size, count uint32
	var buffer []byte
	for {
		var data *byte
		if len(buffer) > 0 {
			data = &buffer[0]
		}
		err = windows.EnumServicesStatusEx(manager, windows.SC_ENUM_PROCESS_INFO, windows.SERVICE_WIN32, windows.SERVICE_STATE_ALL, data, uint32(len(buffer)), &size, &count, nil, nil)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.ERROR_MORE_DATA) || size <= uint32(len(buffer)) {
			return nil, fmt.Errorf("enumerate Windows services: %w", err)
		}
		buffer = make([]byte, size)
	}

	if count == 0 {
		return []services.Entry{}, nil
	}
	if uintptr(count) > uintptr(len(buffer))/unsafe.Sizeof(windows.ENUM_SERVICE_STATUS_PROCESS{}) {
		return nil, errors.New("Windows returned an invalid service count")
	}
	rows := unsafe.Slice((*windows.ENUM_SERVICE_STATUS_PROCESS)(unsafe.Pointer(&buffer[0])), int(count))
	entries := make([]services.Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, services.Entry{
			Name:        windows.UTF16PtrToString(row.ServiceName),
			DisplayName: windows.UTF16PtrToString(row.DisplayName),
			State:       serviceState(row.ServiceStatusProcess.CurrentState),
			PID:         row.ServiceStatusProcess.ProcessId,
		})
	}
	return entries, nil
}

func serviceState(state uint32) string {
	switch state {
	case windows.SERVICE_STOPPED:
		return "Stopped"
	case windows.SERVICE_START_PENDING:
		return "Start pending"
	case windows.SERVICE_STOP_PENDING:
		return "Stop pending"
	case windows.SERVICE_RUNNING:
		return "Running"
	case windows.SERVICE_CONTINUE_PENDING:
		return "Continue pending"
	case windows.SERVICE_PAUSE_PENDING:
		return "Pause pending"
	case windows.SERVICE_PAUSED:
		return "Paused"
	default:
		return "Unknown"
	}
}
