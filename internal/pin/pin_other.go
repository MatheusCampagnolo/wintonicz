//go:build !windows

package pin

import "errors"

var errWindowsOnly = errors.New("pinning is available only on Windows")

func Dir() string            { return "" }
func Target() string         { return "" }
func Pinned() (bool, error)  { return false, errWindowsOnly }
func Pin() error             { return errWindowsOnly }
func Unpin() (string, error) { return "", errWindowsOnly }
