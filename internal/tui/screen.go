package tui

import tea "charm.land/bubbletea/v2"

// hint is one entry in the status bar.
type hint struct{ key, desc string }

// Screen is one tool's view. View is pure: state and size in, string out. Slow
// work happens in commands returned by Init and Update.
type Screen interface {
	Init() tea.Cmd
	Update(tea.Msg) (Screen, tea.Cmd)
	// Title is the content panel title, including any count.
	Title() string
	// View renders the main area inside w×h cells (no borders).
	View(w, h int) string
	// Details renders the side panel body for the current selection.
	Details(w int) string
	Hints() []hint
	// Editing reports that a text prompt owns the keyboard (even q and Esc).
	Editing() bool
	// Dirty reports that Esc should reach the screen first (an active filter).
	Dirty() bool
}

// routedMsg delivers an async result to the screen that requested it, even if the
// user has since moved to another tool.
type routedMsg struct {
	id  string
	msg tea.Msg
}

// noticeMsg shows a one-line message in the status bar.
type noticeMsg struct {
	text  string
	isErr bool
}

func route(id string, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			routed := make(tea.BatchMsg, len(batch))
			for i, inner := range batch {
				routed[i] = route(id, inner)
			}
			return routed
		}
		return routedMsg{id: id, msg: msg}
	}
}
