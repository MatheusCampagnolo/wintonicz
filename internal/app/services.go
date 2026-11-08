package app

import (
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/panels"
	"github.com/MatheusCampagnolo/wintonicz/internal/platform/windows"
	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/resources"
	"github.com/MatheusCampagnolo/wintonicz/internal/services"
	"github.com/MatheusCampagnolo/wintonicz/internal/storage"
	"github.com/MatheusCampagnolo/wintonicz/internal/system"
	"github.com/MatheusCampagnolo/wintonicz/internal/usb"
)

// Services is the shared read-only feature surface used by both the CLI and TUI.
type Services struct {
	Processes processes.Source
	Ports     ports.Source
	System    system.Source
	Network   network.Source
	Panels    panels.Source
	Services  services.Source
	USB       usb.Source
	Storage   storage.Source
	Resources resources.Source
}

// NewWindowsServices wires the initial Windows data collectors.
func NewWindowsServices() Services {
	collector := windows.Collector{}
	return Services{
		Processes: collector,
		Ports:     collector,
		System:    collector,
		Network:   collector,
		Panels:    collector,
		Services:  collector,
		USB:       collector,
		Storage:   collector,
		Resources: collector,
	}
}
