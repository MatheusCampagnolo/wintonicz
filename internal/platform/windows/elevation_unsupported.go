//go:build !windows

package windows

func EnsureElevated() (bool, error) { return false, nil }
