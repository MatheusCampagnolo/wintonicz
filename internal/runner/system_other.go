//go:build !windows

package runner

import "errors"

var errWindowsOnly = errors.New("running actions is available only on Windows")

func SystemExe(string) (string, error) { return "", errWindowsOnly }

func validateExe(string) error { return errWindowsOnly }

func decodeOEM(b []byte) string { return string(b) }
