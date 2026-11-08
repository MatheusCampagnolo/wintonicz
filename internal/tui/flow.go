package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/actions"
	"github.com/MatheusCampagnolo/wintonicz/internal/catalog"
	"github.com/MatheusCampagnolo/wintonicz/internal/config"
	"github.com/MatheusCampagnolo/wintonicz/internal/privilege"
	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
)

type flowStep int

const (
	stepReview flowStep = iota
	stepConfirm
	stepRun
	stepResult
)

// elevatedMsg reports the outcome of a restart-as-administrator request.
type elevatedMsg struct{ err error }

func elevateCmd() tea.Cmd {
	return func() tea.Msg { return elevatedMsg{err: privilege.StartElevated(os.Args[1:])} }
}

type flowEvent struct {
	ch    chan tea.Msg
	inner tea.Msg
}

type flowDone struct {
	result runner.Result
	err    error
}

// flow is the shared Review → Confirm → Run → Result experience for action tools.
type flow struct {
	tool     catalog.Tool
	plan     runner.Plan
	planErr  error
	step     flowStep
	lines    []string
	scroll   int
	result   *runner.Result
	runErr   error
	cancel   context.CancelFunc
	elev     bool
	readOnly bool
	noLog    bool
}

func newFlow(tool catalog.Tool, cfg config.Config) *flow {
	f := &flow{tool: tool, elev: privilege.IsElevated(), readOnly: cfg.ReadOnly, noLog: !cfg.ActionLog}
	build, ok := actions.Plans[tool.ID]
	if !ok {
		f.planErr = fmt.Errorf("no action is registered for %s", tool.ID)
		return f
	}
	f.plan, f.planErr = build()
	return f
}

func (f *flow) running() bool { return f.step == stepRun }

func (f *flow) blocked() string {
	switch {
	case f.planErr != nil:
		return "This action cannot run here: " + f.planErr.Error()
	case f.readOnly:
		return "Read-only mode is on, so actions are disabled. Turn it off with: wintonicz config set readOnly false"
	case f.plan.Preview.Admin && !f.elev:
		return "Administrator rights are required. Press a to restart Wintonicz as administrator."
	}
	return ""
}

func waitForFlow(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return flowEvent{ch: ch, inner: msg}
	}
}

func (f *flow) start() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	f.step = stepRun
	ch := make(chan tea.Msg, 128)
	plan := f.plan
	go func() {
		defer close(ch)
		result, err := runner.Run(ctx, plan, runner.Options{ReadOnly: f.readOnly, NoLog: f.noLog, OnLine: func(l runner.Line) { ch <- l }})
		ch <- flowDone{result: result, err: err}
	}()
	return waitForFlow(ch)
}

// Update returns the command to run and whether the flow should close.
func (f *flow) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case flowEvent:
		switch inner := msg.inner.(type) {
		case runner.Line:
			f.lines = append(f.lines, inner.Text)
			if len(f.lines) > 5000 {
				f.lines = f.lines[len(f.lines)-5000:]
			}
			f.scroll = 1 << 30 // follow the end; clamped in View
			return waitForFlow(msg.ch), false
		case flowDone:
			f.step, f.result, f.runErr = stepResult, &inner.result, inner.err
			return nil, false
		}
	case tea.KeyPressMsg:
		key := msg.String()
		switch f.step {
		case stepReview:
			switch key {
			case "esc", "q", "n", "backspace":
				return nil, true
			case "a":
				if f.plan.Preview.Admin && !f.elev {
					return elevateCmd(), false
				}
			case "enter":
				if f.blocked() == "" {
					f.step = stepConfirm
				}
			}
		case stepConfirm:
			switch key {
			case "y":
				return f.start(), false
			case "esc", "n", "backspace", "q":
				return nil, true
			}
		case stepRun:
			if key == "ctrl+c" && f.cancel != nil {
				f.cancel()
			}
		case stepResult:
			switch key {
			case "esc", "enter", "q", "backspace":
				return nil, true
			case "up", "k":
				f.scroll = max(0, f.scroll-1)
			case "down", "j":
				f.scroll++
			}
		}
	}
	return nil, false
}

func (f *flow) hints() []hint {
	switch f.step {
	case stepReview:
		hints := []hint{{"enter", "continue"}, {"esc", "cancel"}}
		if f.plan.Preview.Admin && !f.elev {
			hints = append([]hint{{"a", "restart as admin"}}, hints...)
		}
		return hints
	case stepConfirm:
		return []hint{{"y", "run now"}, {"esc", "cancel"}}
	case stepRun:
		return []hint{{"ctrl+c", "stop"}}
	}
	return []hint{{"↑↓", "scroll"}, {"enter", "close"}}
}

func (f *flow) View(w, h int) string {
	p := f.plan.Preview
	inner := w - 4
	steps := []string{"1 Review", "2 Confirm", "3 Run", "4 Result"}
	for i := range steps {
		if flowStep(i) == f.step {
			steps[i] = accent("[" + steps[i] + "]")
		} else {
			steps[i] = muted(steps[i])
		}
	}
	head := strings.Join(steps, "  ")
	switch f.step {
	case stepReview, stepConfirm:
		admin := okText("not needed")
		if p.Admin {
			admin = warn("required")
			if f.elev {
				admin = okText("required · you are administrator")
			}
		}
		lines := []string{head, "", wrap(p.Summary, inner), "",
			"Command   " + strong(p.Command),
			"Admin     " + admin,
			"Duration  " + p.Duration,
			fmt.Sprintf("Restart   %s", map[bool]string{true: "may be needed", false: "not expected"}[p.Restart]),
			"Risk      " + string(f.tool.Risk),
			"Undo      " + wrap(p.Undo, inner-10), ""}
		if reason := f.blocked(); reason != "" {
			lines = append(lines, warn("! ")+wrap(reason, inner-2))
		} else if f.step == stepReview {
			lines = append(lines, muted("Enter continues to the confirmation step."))
		} else {
			lines = append(lines, strong("Run this now? Press y to run, Esc to cancel."))
		}
		return strings.Join(lines, "\n")
	case stepRun:
		out := tail(f.lines, h-4)
		return strings.Join(append([]string{head, muted("Running " + p.Command + " … Ctrl+C stops it."), ""}, out...), "\n")
	}
	status := okText("✓ Completed")
	switch {
	case f.runErr != nil:
		status = danger("✗ Could not run: " + f.runErr.Error())
	case f.result != nil && f.result.Cancelled:
		status = warn("− Cancelled")
	case f.result != nil && f.result.ExitCode != 0:
		status = danger(fmt.Sprintf("✗ Failed (exit %d)", f.result.ExitCode))
	}
	lines := []string{head, "", status}
	if f.result != nil && f.runErr == nil {
		lines = append(lines, muted(fmt.Sprintf("%d ms · logged to %s", f.result.DurationMS, orUnavailable(f.result.LogPath))))
	}
	space := max(1, h-len(lines)-2)
	f.scroll = max(0, min(f.scroll, max(0, len(f.lines)-space)))
	end := min(len(f.lines), f.scroll+space)
	lines = append(lines, "")
	lines = append(lines, f.lines[f.scroll:end]...)
	return strings.Join(lines, "\n")
}

func tail(lines []string, n int) []string {
	if n < 1 {
		return nil
	}
	if len(lines) > n {
		return lines[len(lines)-n:]
	}
	return lines
}
