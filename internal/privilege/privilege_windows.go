//go:build windows

package privilege

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IsElevated reports whether the current process token is elevated.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

var shellExecuteEx = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

type shellExecuteInfo struct {
	size       uint32
	mask       uint32
	window     windows.Handle
	verb       *uint16
	file       *uint16
	parameters *uint16
	directory  *uint16
	show       int32
	instance   windows.Handle
	idList     uintptr
	class      *uint16
	classKey   windows.Handle
	hotKey     uint32
	icon       windows.Handle
	process    windows.Handle
}

const seeMaskNoCloseProcess = 0x00000040

// RelaunchElevated starts this same executable with the given arguments through a
// UAC prompt and waits for it. It returns the child's exit code. A declined prompt
// returns ErrCancelled.
func RelaunchElevated(args []string) (int, error) {
	process, code, err := startElevated(args)
	if err != nil {
		return code, err
	}
	defer windows.CloseHandle(process)
	if _, err := windows.WaitForSingleObject(process, windows.INFINITE); err != nil {
		return 1, err
	}
	var exit uint32
	if err := windows.GetExitCodeProcess(process, &exit); err != nil {
		return 1, err
	}
	return int(exit), nil
}

// StartElevated starts this executable elevated and returns without waiting. The
// caller normally exits afterwards. A declined prompt returns ErrCancelled.
func StartElevated(args []string) error {
	process, _, err := startElevated(args)
	if err != nil {
		return err
	}
	windows.CloseHandle(process)
	return nil
}

func composeCommandLine(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = syscall.EscapeArg(arg)
	}
	return strings.Join(parts, " ")
}

// startElevated launches this executable through UAC and returns the process
// handle (caller closes it). On failure the int is a suggested exit code.
func startElevated(args []string) (windows.Handle, int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, 1, err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, 1, err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	params, err := windows.UTF16PtrFromString(composeCommandLine(args))
	if err != nil {
		return 0, 1, err
	}
	info := shellExecuteInfo{mask: seeMaskNoCloseProcess, verb: verb, file: file, parameters: params, show: windows.SW_SHOWNORMAL}
	info.size = uint32(unsafe.Sizeof(info))
	ok, _, callErr := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errors.Is(callErr, windows.ERROR_CANCELLED) {
			return 0, 4, ErrCancelled
		}
		return 0, 1, callErr
	}
	if info.process == 0 {
		return 0, 1, errors.New("Windows did not return a process handle")
	}
	return info.process, 0, nil
}
