//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/support"
	"golang.org/x/sys/windows"
)

func (Collector) RunDiskCheck(ctx context.Context, drive string) (support.DiskCheckResult, error) {
	drive, err := support.NormalizeDrive(drive)
	if err != nil {
		return support.DiskCheckResult{}, err
	}
	volumes, err := (Collector{}).ListVolumes()
	if err != nil {
		return support.DiskCheckResult{}, fmt.Errorf("validate disk-check drive: %w", err)
	}
	checkable := false
	for _, volume := range volumes {
		if strings.EqualFold(volume.Drive, drive) && support.IsDiskCheckVolume(volume) {
			checkable = true
			break
		}
	}
	if !checkable {
		return support.DiskCheckResult{}, fmt.Errorf("%s is not a recognized fixed or removable drive", strings.TrimSuffix(drive, `\`))
	}

	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return support.DiskCheckResult{}, fmt.Errorf("find Windows system directory: %w", err)
	}
	command := support.DiskCheckCommand(drive)
	started := time.Now()
	output, runErr := exec.CommandContext(ctx, filepath.Join(systemDirectory, "chkdsk.exe"), strings.TrimSuffix(drive, `\`)).CombinedOutput()
	result := support.DiskCheckResult{
		Drive:      drive,
		Command:    command,
		DurationMS: time.Since(started).Milliseconds(),
		Output:     string(output),
	}
	if runErr == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, fmt.Errorf("run %s: %w", command, runErr)
}
