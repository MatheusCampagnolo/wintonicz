//go:build windows

package windows

import (
	"context"
	"fmt"
	"strings"

	"github.com/MatheusCampagnolo/wintonicz/internal/support"
)

func (Collector) RunDiskCheck(ctx context.Context, drive string) (support.DiskCheckResult, error) {
	if err := ctx.Err(); err != nil {
		return support.DiskCheckResult{}, err
	}
	drive, err := validateDiskCheckDrive(drive)
	if err != nil {
		return support.DiskCheckResult{}, err
	}
	return runElevatedDiskCheck(ctx, drive)
}

func validateDiskCheckDrive(drive string) (string, error) {
	drive, err := support.NormalizeDrive(drive)
	if err != nil {
		return "", err
	}
	volumes, err := (Collector{}).ListVolumes()
	if err != nil {
		return "", fmt.Errorf("validate disk-check drive: %w", err)
	}
	checkable := false
	for _, volume := range volumes {
		if strings.EqualFold(volume.Drive, drive) && support.IsDiskCheckVolume(volume) {
			checkable = true
			break
		}
	}
	if !checkable {
		return "", fmt.Errorf("%s is not a recognized fixed or removable drive", strings.TrimSuffix(drive, `\`))
	}
	return drive, nil
}
