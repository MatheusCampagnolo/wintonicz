//go:build !windows

package privilege

import "errors"

func IsElevated() bool { return false }

func RelaunchElevated([]string) (int, error) {
	return 1, errors.New("elevation is available only on Windows")
}

func StartElevated([]string) error { return errors.New("elevation is available only on Windows") }
