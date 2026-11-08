package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/catalog"
	"github.com/MatheusCampagnolo/wintonicz/internal/config"
	"github.com/MatheusCampagnolo/wintonicz/internal/privilege"
	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
)

type flowStep int

const (
	stepParams flowStep = iota
	stepReview
	stepConfirm
	stepRun
	stepResult
)

// elevatedMsg reports the outcome of a restart-as-administrator request.
type elevatedMsg struct{ err error }

func elevateCmd() tea.Cmd {
	return func() tea.Msg { return elevatedMsg{err: privilege.StartElevated(os.Args[1:])} }
}

// openActionMsg asks the shell to start an action flow, optionally with some inputs filled in.
type openActionMsg struct {
	toolID string
	preset map[string]string
}

type flowEvent struct {
	ch    chan tea.Msg
	inner tea.Msg
}

type flowDone struct {
	result runner.Result
	err    error
}

// flow is the shared Inputs → Review → Confirm → Run → Result experience for action tools.
type flow struct {
	tool     catalog.Tool
	spec     runner.Spec
	params   map[string]string
	paramIdx int
	draft    string
	pos      int
	plan     runner.Plan
	planErr  error
	step     flowStep
	typed    string
	typedPos int
	lines    []string
	scroll   int
	result   *runner.Result
	runErr   error
	cancel   context.CancelFunc
	elev     bool
	readOnly bool
	noLog    bool
}

func newFlow(tool catalog.Tool, cfg config.Config, specs map[string]runner.Spec, preset map[string]string) *flow {
	f := &flow{tool: tool, elev: privilege.IsElevated(), readOnly: cfg.ReadOnly, noLog: !cfg.ActionLog, params: map[string]string{}}
	spec, ok := specs[tool.ID]
	if !ok {
		f.planErr = fmt.Errorf("no action is registered for %s", tool.ID)
		f.step = stepReview
		return f
	}
	f.spec = spec
	for k, v := range preset {
		f.params[k] = v
	}
	f.nextParam()
	return f
}

// nextParam advances to the next input that is not already filled, then builds the plan.
func (f *flow) nextParam() {
	for f.paramIdx < len(f.spec.Params) {
		p := f.spec.Params[f.paramIdx]
		if _, preset := f.params[p.Name]; !preset {
			f.step = stepParams
			f.draft = p.Default
			f.pos = len([]rune(f.draft))
			return
		}
		f.paramIdx++
	}
	f.plan, f.planErr = f.spec.Build(f.params)
	f.step = stepReview
}

func (f *flow) running() bool { return f.step == stepRun }

func (f *flow) blocked() string {
	switch {
	case f.planErr != nil:
		return "Cannot continue: " + f.planErr.Error()
	case f.readOnly && !f.plan.ReadOnlyOK:
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
		}
	case tea.KeyPressMsg:
		return f.key(msg)
	}
	return nil, false
}

func (f *flow) key(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	key := msg.String()
	switch f.step {
	case stepParams:
		switch key {
		case "esc":
			return nil, true
		case "enter":
			p := f.spec.Params[f.paramIdx]
			value := strings.TrimSpace(f.draft)
			if p.Required && value == "" {
				return nil, false
			}
			f.params[p.Name] = value
			f.paramIdx++
			f.nextParam()
		default:
			f.draft, f.pos = editTextDraft(f.draft, f.pos, msg, 300)
		}
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
				f.typed, f.typedPos = "", 0
			}
		}
	case stepConfirm:
		if word := f.plan.Preview.ConfirmWord; word != "" {
			switch key {
			case "esc":
				return nil, true
			case "enter":
				if f.typed == word {
					return f.start(), false
				}
			default:
				f.typed, f.typedPos = editTextDraft(f.typed, f.typedPos, msg, 40)
			}
			return nil, false
		}
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
	return nil, false
}

func (f *flow) hints() []hint {
	switch f.step {
	case stepParams:
		return []hint{{"enter", "next"}, {"esc", "cancel"}}
	case stepReview:
		hints := []hint{{"enter", "continue"}, {"esc", "cancel"}}
		if f.plan.Preview.Admin && !f.elev {
			hints = append([]hint{{"a", "restart as admin"}}, hints...)
		}
		return hints
	case stepConfirm:
		if f.plan.Preview.ConfirmWord != "" {
			return []hint{{"enter", "run (word must match)"}, {"esc", "cancel"}}
		}
		return []hint{{"y", "run now"}, {"esc", "cancel"}}
	case stepRun:
		return []hint{{"ctrl+c", "stop"}}
	}
	return []hint{{"↑↓", "scroll"}, {"enter", "close"}}
}

func (f *flow) View(w, h int) string {
	p := f.plan.Preview
	inner := w - 4
	steps := []string{"1 Inputs", "2 Review", "3 Confirm", "4 Run", "5 Result"}
	for i := range steps {
		if i == int(f.step) {
			steps[i] = accent("[" + steps[i] + "]")
		} else {
			steps[i] = muted(steps[i])
		}
	}
	head := strings.Join(steps, " ")

	switch f.step {
	case stepParams:
		prm := f.spec.Params[f.paramIdx]
		lines := []string{head, "", strong(prm.Label)}
		if prm.Help != "" {
			lines = append(lines, wrap(muted(prm.Help), inner))
		}
		if len(prm.Choices) > 0 {
			lines = append(lines, muted("Options: "+strings.Join(prm.Choices, ", ")))
		}
		lines = append(lines, "", "> "+renderDraft(f.draft, f.pos), "",
			muted(fmt.Sprintf("Input %d of %d · Enter continues · Esc cancels", f.paramIdx+1, len(f.spec.Params))))
		return strings.Join(lines, "\n")
	case stepReview, stepConfirm:
		admin := okText("not needed")
		if p.Admin {
			admin = warn("required")
			if f.elev {
				admin = okText("required · you are administrator")
			}
		}
		lines := []string{head, "", strong(p.Title), wrap(p.Summary, inner), "",
			"Command   " + wrap(p.Command, inner-10),
			"Admin     " + admin,
			"Duration  " + p.Duration,
			"Risk      " + string(f.tool.Risk),
			"Undo      " + wrap(p.Undo, inner-10)}
		if len(p.Items) > 0 {
			lines = append(lines, "", strong("This will touch:"))
			for i, item := range p.Items {
				if i == 3 && len(p.Items) > 4 {
					lines = append(lines, fmt.Sprintf("  … and %d more", len(p.Items)-3))
					break
				}
				lines = append(lines, "  • "+item)
			}
		}
		var prompt []string
		switch {
		case f.blocked() != "":
			prompt = append(prompt, warn("! ")+wrap(f.blocked(), inner-2))
		case f.step == stepReview:
			prompt = append(prompt, muted("Enter continues to the confirmation step."))
		case p.ConfirmWord != "":
			mark := muted("type it exactly")
			if f.typed == p.ConfirmWord {
				mark = okText("✓ matches, press Enter")
			}
			prompt = append(prompt, strong("To confirm, type  "+p.ConfirmWord+"  then press Enter."), "> "+renderDraft(f.typed, f.typedPos)+"   "+mark)
		default:
			prompt = append(prompt, strong("Run this now? Press y to run, Esc to cancel."))
		}
		// The prompt must always stay visible; the description is what gets clipped.
		body := strings.Split(strings.Join(lines, "\n"), "\n")
		room := max(1, h-2-len(prompt)-1)
		if len(body) > room {
			body = body[:room]
		}
		return strings.Join(append(append(body, ""), prompt...), "\n")
	case stepRun:
		out := tail(f.lines, h-5)
		return strings.Join(append([]string{head, muted("Running… Ctrl+C stops it."), ""}, out...), "\n")
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
	space := max(1, h-len(lines)-3)
	f.scroll = max(0, min(f.scroll, max(0, len(f.lines)-space)))
	end := min(len(f.lines), f.scroll+space)
	lines = append(lines, "")
	lines = append(lines, f.lines[f.scroll:end]...)
	return strings.Join(lines, "\n")
}

func clip(lines []string, n int) string {
	if n < 1 {
		return ""
	}
	all := strings.Split(strings.Join(lines, "\n"), "\n")
	if len(all) > n {
		all = all[:n]
	}
	return strings.Join(all, "\n")
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
