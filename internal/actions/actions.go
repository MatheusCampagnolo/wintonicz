// Package actions maps action tools (catalog entries with Kind "action") to the
// functions that build their runner plans. The CLI and TUI both read this table.
package actions

import (
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
)

// Plans is keyed by catalog tool ID.
var Plans = map[string]func() (runner.Plan, error){
	"net.flushdns": network.FlushDNSPlan,
}
