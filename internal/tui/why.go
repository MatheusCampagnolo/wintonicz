package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/why"
)

type whyMsg struct {
	report why.Report
	err    error
}

// whyScreen asks for a PID, name, or :port and shows the investigation.
type whyScreen struct {
	deps    why.Deps
	editing bool
	started bool
	draft   string
	pos     int
	running bool
	report  *why.Report
	err     error
}

func newWhyScreen(d why.Deps) *whyScreen { return &whyScreen{deps: d} }

func (w *whyScreen) Init() tea.Cmd {
	if !w.started {
		w.started, w.editing = true, true
	}
	return nil
}

func (w *whyScreen) Title() string { return "WHY IS THIS RUNNING?" }
func (w *whyScreen) Editing() bool { return w.editing }
func (w *whyScreen) Dirty() bool   { return false }
func (w *whyScreen) Details(int) string {
	return "Type a PID (1234), a process name (node.exe), or a port (:8080). [confirmed] facts were read from Windows; [inferred] ones are a reasoned guess. Unknowns are stated, not invented."
}

func (w *whyScreen) Hints() []hint {
	if w.editing {
		return []hint{{"enter", "investigate"}, {"esc", "cancel"}}
	}
	return []hint{{"c", "new target"}}
}

func (w *whyScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case whyMsg:
		w.running, w.err = false, msg.err
		if msg.err == nil {
			w.report = &msg.report
		}
	case tea.KeyPressMsg:
		if w.editing {
			switch msg.String() {
			case "enter":
				target := strings.TrimSpace(w.draft)
				if target == "" {
					return w, nil
				}
				w.editing, w.running, w.report, w.err = false, true, nil, nil
				deps := w.deps
				return w, func() tea.Msg {
					pid, err := why.ResolvePID(deps, target)
					if err != nil {
						return whyMsg{err: err}
					}
					r, err := why.Investigate(deps, target, pid)
					return whyMsg{report: r, err: err}
				}
			case "esc":
				w.editing = false
			default:
				w.draft, w.pos = editTextDraft(w.draft, w.pos, msg, 120)
			}
			return w, nil
		}
		if k := msg.String(); k == "c" || k == "enter" {
			w.editing, w.pos = true, len([]rune(w.draft))
		}
	}
	return w, nil
}

func (w *whyScreen) View(width, h int) string {
	lines := []string{"PID, process name, or :port"}
	if w.editing {
		lines = append(lines, "> "+renderDraft(w.draft, w.pos), "")
	} else {
		lines = append(lines, "> "+w.draft, "")
	}
	switch {
	case w.running:
		lines = append(lines, muted("Reading process details…"))
	case w.err != nil:
		lines = append(lines, danger("✗ ")+wrap(w.err.Error(), width-2))
	case w.report != nil:
		for _, l := range why.Lines(*w.report) {
			switch {
			case strings.HasPrefix(l, "  [confirmed]"):
				lines = append(lines, okText("  [confirmed]")+strings.TrimPrefix(l, "  [confirmed]"))
			case strings.HasPrefix(l, "  [inferred]"):
				lines = append(lines, warn("  [inferred]")+strings.TrimPrefix(l, "  [inferred]"))
			case l != "" && !strings.HasPrefix(l, " "):
				lines = append(lines, strong(l))
			default:
				lines = append(lines, l)
			}
		}
	default:
		lines = append(lines, muted("Type a target and press Enter."))
	}
	return strings.Join(lines, "\n")
}
