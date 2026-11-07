//go:build !windows

package windows

import (
	"context"
	"errors"

	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/panels"
	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/resources"
	"github.com/MatheusCampagnolo/wintonicz/internal/services"
	"github.com/MatheusCampagnolo/wintonicz/internal/storage"
	"github.com/MatheusCampagnolo/wintonicz/internal/support"
	"github.com/MatheusCampagnolo/wintonicz/internal/system"
	"github.com/MatheusCampagnolo/wintonicz/internal/usb"
)

var errWindowsOnly = errors.New("this diagnostic is currently available only on Windows")

type Collector struct{}

func (Collector) ListProcesses() ([]processes.Entry, error) {
	return nil, errWindowsOnly
}

func (Collector) ListListeners() ([]ports.Listener, error) {
	return nil, errWindowsOnly
}

func (Collector) SystemSnapshot() (system.Snapshot, error) {
	return system.Snapshot{}, errWindowsOnly
}

func (Collector) NetworkAdapters() ([]network.Adapter, error) {
	return nil, errWindowsOnly
}

func (Collector) ListPanels() []panels.Entry {
	return nil
}

func (Collector) ListServices() ([]services.Entry, error) {
	return nil, errWindowsOnly
}

func (Collector) ListConnectedDevices() ([]usb.Device, error) {
	return nil, errWindowsOnly
}

func (Collector) ListVolumes() ([]storage.Volume, error) {
	return nil, errWindowsOnly
}

func (Collector) ResourceSnapshot() (resources.Snapshot, error) {
	return resources.Snapshot{}, errWindowsOnly
}

func (Collector) RunDiskCheck(context.Context, string) (support.DiskCheckResult, error) {
	return support.DiskCheckResult{}, errWindowsOnly
}

func (Collector) OpenPanel(string) error {
	return errWindowsOnly
}
