// Package actions maps action tools (catalog entries with Kind "action") to their
// runner specs. The CLI and TUI both read this table.
package actions

import (
	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/maint"
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
)

// Specs is keyed by catalog tool ID. It needs the services only for lookups some
// actions must do before they can describe themselves (for example, which process
// a PID is).
func Specs(s app.Services) map[string]runner.Spec {
	specs := map[string]runner.Spec{
		"net.flushdns": {Build: func(map[string]string) (runner.Plan, error) { return network.FlushDNSPlan() }},
	}
	for id, spec := range maint.Specs(func() ([]processes.Entry, error) { return s.Processes.ListProcesses() }) {
		specs[id] = spec
	}
	return specs
}
