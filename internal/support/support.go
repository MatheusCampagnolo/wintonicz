package support

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MatheusCampagnolo/wintonicz/internal/storage"
)

var ErrElevationCancelled = errors.New("administrator approval was cancelled")

type DiskCheckResult struct {
	Drive      string `json:"drive"`
	Command    string `json:"command"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
	Output     string `json:"output"`
}

type DiskCheckPreview struct {
	Drive               string `json:"drive"`
	Command             string `json:"command"`
	Purpose             string `json:"purpose"`
	Notes               string `json:"notes"`
	FilesystemChanges   bool   `json:"filesystemChanges"`
	AdministratorNeeded bool   `json:"administratorNeeded"`
	RestartExpected     bool   `json:"restartExpected"`
}

type DiskCheckExitError struct{ Code int }

func (e DiskCheckExitError) Error() string {
	return fmt.Sprintf("chkdsk exited with status %d", e.Code)
}
func (e DiskCheckExitError) ExitCode() int { return e.Code }

type Source interface {
	RunDiskCheck(context.Context, string) (DiskCheckResult, error)
}

func NormalizeDrive(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) == 2 && value[1] == ':' {
		value += `\`
	}
	if len(value) != 3 || value[1:] != `:\` || !isDriveLetter(value[0]) {
		return "", fmt.Errorf("drive must be a letter such as C:")
	}
	return strings.ToUpper(value), nil
}

func IsDiskCheckVolume(volume storage.Volume) bool {
	return volume.Type == "Fixed" || volume.Type == "Removable"
}

func DiskCheckCommand(drive string) string {
	return "chkdsk " + strings.TrimSuffix(drive, `\`)
}

func NewDiskCheckPreview(drive string) DiskCheckPreview {
	return DiskCheckPreview{
		Drive:               drive,
		Command:             DiskCheckCommand(drive),
		Purpose:             "Report filesystem status without requesting repairs.",
		Notes:               "An active volume may report inconclusive errors because it cannot be locked. Allow the check to finish for a complete report.",
		FilesystemChanges:   false,
		AdministratorNeeded: true,
		RestartExpected:     false,
	}
}

func isDriveLetter(value byte) bool {
	return (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z')
}
