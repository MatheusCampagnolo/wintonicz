//go:build !windows

package maint

import "errors"

var errWindowsOnly = errors.New("this action is available only on Windows")

func recycleBinInfo() (int64, int64, error) { return 0, 0, errWindowsOnly }
func emptyRecycleBin() error                { return errWindowsOnly }
func terminateProcess(uint32) error         { return errWindowsOnly }
