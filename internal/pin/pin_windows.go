//go:build windows

package pin

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Dir is the folder the executable is copied to.
func Dir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return ""
	}
	return filepath.Join(base, "Wintonicz", "bin")
}

// Target is the pinned executable path.
func Target() string { return filepath.Join(Dir(), "wintonicz.exe") }

func userPath() (value string, valueType uint32, err error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return "", registry.EXPAND_SZ, err
	}
	defer key.Close()
	value, valueType, err = key.GetStringValue("Path")
	if errors.Is(err, registry.ErrNotExist) {
		return "", registry.EXPAND_SZ, nil
	}
	return value, valueType, err
}

func setUserPath(value string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	// Keep REG_EXPAND_SZ so entries such as %USERPROFILE%\bin keep working.
	if err := key.SetExpandStringValue("Path", value); err != nil {
		return err
	}
	broadcastEnvironmentChange()
	return nil
}

func broadcastEnvironmentChange() {
	user32 := windows.NewLazySystemDLL("user32.dll")
	proc := user32.NewProc("SendMessageTimeoutW")
	env, _ := syscall.UTF16PtrFromString("Environment")
	var result uintptr
	// HWND_BROADCAST, WM_SETTINGCHANGE, SMTO_ABORTIFHUNG, 2 s
	proc.Call(0xFFFF, 0x001A, 0, uintptr(unsafe.Pointer(env)), 0x2, 2000, uintptr(unsafe.Pointer(&result)))
}

// Pinned reports whether the user PATH already contains the pin folder.
func Pinned() (bool, error) {
	current, _, err := userPath()
	if err != nil {
		return false, err
	}
	_, would := AddPath(current, Dir())
	return !would, nil
}

// Pin copies the running executable into Dir and adds Dir to the user PATH.
func Pin() error {
	dir := Dir()
	if dir == "" {
		return errors.New("LOCALAPPDATA is not set")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(self), filepath.Clean(Target())) {
		if err := copyFile(self, Target()); err != nil {
			return fmt.Errorf("copy executable: %w", err)
		}
	}
	current, _, err := userPath()
	if err != nil {
		return fmt.Errorf("read user PATH: %w", err)
	}
	if updated, changed := AddPath(current, dir); changed {
		if err := setUserPath(updated); err != nil {
			return fmt.Errorf("update user PATH: %w", err)
		}
	}
	return nil
}

// Unpin removes Dir from the user PATH and deletes the pinned copy when possible.
// It returns a note when the file could not be deleted (for example, it is running).
func Unpin() (note string, err error) {
	current, _, err := userPath()
	if err != nil {
		return "", fmt.Errorf("read user PATH: %w", err)
	}
	if updated, changed := RemovePath(current, Dir()); changed {
		if err := setUserPath(updated); err != nil {
			return "", fmt.Errorf("update user PATH: %w", err)
		}
	}
	if err := os.Remove(Target()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "The PATH entry was removed, but " + Target() + " could not be deleted (is it running?). Delete it by hand.", nil
	}
	_ = os.Remove(Dir())
	return "", nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := to + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, to)
}
