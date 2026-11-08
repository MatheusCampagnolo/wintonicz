package tui

import (
	"fmt"
	"strings"

	"github.com/MatheusCampagnolo/wintonicz/internal/actionlog"
	"github.com/MatheusCampagnolo/wintonicz/internal/config"
	"github.com/MatheusCampagnolo/wintonicz/internal/pin"
)

// aboutLines describes where Wintonicz keeps its files and which preferences are on.
func aboutLines(version string) []string {
	pinned := "no"
	if ok, _ := pin.Pinned(); ok {
		pinned = "yes"
	}
	cfg, _ := config.Load()
	return []string{
		"Wintonicz " + version,
		"",
		"Preferences  " + orNA(config.Path()),
		"Action log   " + orNA(actionlog.Dir()),
		"Pinned copy  " + orNA(pin.Target()) + "  (on PATH: " + pinned + ")",
		"",
		fmt.Sprintf("readOnly=%t  actionLog=%t  jsonDefault=%t  defaultArea=%s", cfg.ReadOnly, cfg.ActionLog, cfg.JSONDefault, cfg.DefaultArea),
		"",
		"Change settings with: wintonicz config set <key> <value>",
		"Keys: " + strings.Join(config.Keys(), ", "),
		"Nothing here is sent anywhere. There is no telemetry and no self-update.",
	}
}

func orNA(s string) string {
	if s == "" {
		return "(unavailable)"
	}
	return s
}
