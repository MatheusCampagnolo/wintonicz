//go:build windows

package windows

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/MatheusCampagnolo/wintonicz/internal/support"
	"golang.org/x/sys/windows"
)

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
)

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

var shellExecuteEx = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

func runElevatedDiskCheck(ctx context.Context, drive string) (support.DiskCheckResult, error) {
	if err := ctx.Err(); err != nil {
		return support.DiskCheckResult{}, err
	}
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return support.DiskCheckResult{}, fmt.Errorf("find Windows system directory: %w", err)
	}
	executable := filepath.Join(systemDirectory, "WindowsPowerShell", "v1.0", "powershell.exe")
	file, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return support.DiskCheckResult{}, err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	workingDirectory, err := windows.UTF16PtrFromString(systemDirectory)
	if err != nil {
		return support.DiskCheckResult{}, err
	}
	script := fmt.Sprintf("$p = Start-Process -FilePath $env:ComSpec -ArgumentList @('/d','/c','chkdsk %s') -NoNewWindow -Wait -PassThru; exit $p.ExitCode", drive[:2])
	encodedScript := encodePowerShellCommand(script)
	parameters, err := windows.UTF16PtrFromString("-NoProfile -NonInteractive -EncodedCommand " + encodedScript)
	if err != nil {
		return support.DiskCheckResult{}, err
	}
	started := time.Now()
	info := shellExecuteInfo{
		size:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		mask:       seeMaskNoCloseProcess | seeMaskNoAsync,
		verb:       verb,
		file:       file,
		parameters: parameters,
		directory:  workingDirectory,
		show:       windows.SW_SHOWNORMAL,
	}
	ok, _, callErr := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errors.Is(callErr, windows.ERROR_CANCELLED) {
			return support.DiskCheckResult{}, support.ErrElevationCancelled
		}
		if callErr == nil {
			return support.DiskCheckResult{}, fmt.Errorf("request administrator consent failed")
		}
		return support.DiskCheckResult{}, fmt.Errorf("request administrator consent: %w", callErr)
	}
	if info.process == 0 {
		return support.DiskCheckResult{}, fmt.Errorf("Windows did not return a CHKDSK process handle")
	}
	defer windows.CloseHandle(info.process)
	if _, err := windows.WaitForSingleObject(info.process, windows.INFINITE); err != nil {
		return support.DiskCheckResult{}, fmt.Errorf("wait for CHKDSK: %w", err)
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.process, &exitCode); err != nil {
		return support.DiskCheckResult{}, fmt.Errorf("read CHKDSK exit status: %w", err)
	}
	return support.DiskCheckResult{
		Drive:      drive,
		Command:    support.DiskCheckCommand(drive),
		ExitCode:   int(exitCode),
		DurationMS: time.Since(started).Milliseconds(),
		Output:     "CHKDSK output was shown in the temporary elevated PowerShell console.",
	}, nil
}

func encodePowerShellCommand(command string) string {
	units := utf16.Encode([]rune(command))
	encoded := make([]byte, len(units)*2)
	for index, unit := range units {
		binary.LittleEndian.PutUint16(encoded[index*2:], unit)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}
