//go:build windows

package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SystemExe resolves a tool shipped in System32 to an absolute path. PATH is never searched.
func SystemExe(name string) (string, error) {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		return "", fmt.Errorf("find Windows system directory: %w", err)
	}
	return filepath.Join(dir, name), nil
}

// WingetExe resolves the App Installer alias, the only non-System32 executable the
// runner may start. It returns an error when winget is not installed for this user.
func WingetExe() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("LOCALAPPDATA is not set")
	}
	path := filepath.Join(base, "Microsoft", "WindowsApps", "winget.exe")
	if _, err := os.Stat(path); err != nil {
		return "", errors.New("winget (App Installer) is not installed for this user")
	}
	return path, nil
}

func validateExe(exe string) error {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	if !filepath.IsAbs(exe) {
		return fmt.Errorf("refusing to run %q: path must be absolute", exe)
	}
	parent := filepath.Dir(exe)
	if strings.EqualFold(parent, dir) || strings.EqualFold(parent, filepath.Join(dir, "WindowsPowerShell", "v1.0")) {
		return nil
	}
	if winget, err := WingetExe(); err == nil && strings.EqualFold(exe, winget) {
		return nil
	}
	return fmt.Errorf("refusing to run %q: only executables in %s (and winget) are allowed", exe, dir)
}

var (
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGetOEMCP        = kernel32.NewProc("GetOEMCP")
	procMultiByteToWide = kernel32.NewProc("MultiByteToWideChar")
)

// decodeOEM converts console output in the OEM code page to UTF-8.
func decodeOEM(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	cp, _, _ := procGetOEMCP.Call()
	n, _, _ := procMultiByteToWide.Call(cp, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, 0)
	if n == 0 {
		return string(b)
	}
	wide := make([]uint16, n)
	procMultiByteToWide.Call(cp, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&wide[0])), n)
	return string(utf16.Decode(wide))
}
