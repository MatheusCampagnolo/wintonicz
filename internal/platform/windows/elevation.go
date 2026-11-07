//go:build windows

package windows

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func EnsureElevated() (bool, error) {
	if windows.GetCurrentProcessToken().IsElevated() {
		return false, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("locate Wintonicz executable: %w", err)
	}
	path, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return false, fmt.Errorf("encode Wintonicz executable path: %w", err)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	directory, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("get current directory: %w", err)
	}
	workingDirectory, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return false, fmt.Errorf("encode current directory: %w", err)
	}
	if err := windows.ShellExecute(0, verb, path, nil, workingDirectory, 1); err != nil {
		return false, fmt.Errorf("request elevation: %w", err)
	}
	return true, nil
}
