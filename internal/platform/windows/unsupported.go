//go:build !windows

package windows

import (
	"errors"

	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/panels"
	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/system"
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

func (Collector) OpenPanel(string) error {
	return errWindowsOnly
}
