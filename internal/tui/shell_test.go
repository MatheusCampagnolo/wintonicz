package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
)

type fakeProcesses struct{}

func (fakeProcesses) ListProcesses() ([]processes.Entry, error) {
	return []processes.Entry{{PID: 7, ParentID: 1, Name: "alpha.exe"}, {PID: 8, ParentID: 1, Name: "beta.exe"}}, nil
}

type failingProcesses struct{}

func (failingProcesses) ListProcesses() ([]processes.Entry, error) {
	return nil, errors.New("Access is denied")
}

// drive runs a command and feeds every resulting message back into the model.
func drive(m model, cmd tea.Cmd) model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = drive(m, c)
		}
		return m
	}
	if msg == nil {
		return m
	}
	next, following := m.Update(msg)
	return drive(next.(model), following)
}

func press(m model, key tea.Key) model {
	next, cmd := m.Update(tea.KeyPressMsg(key))
	return drive(next.(model), cmd)
}

func typed(m model, text string) model {
	for _, r := range text {
		m = press(m, tea.Key{Text: string(r), Code: r})
	}
	return m
}

func sized(services app.Services) model {
	m := newModel(services, "test")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	return next.(model)
}

func TestSidebarListsAreasAndRiskBadges(t *testing.T) {
	m := sized(app.Services{})
	m.elevated = false // CI runners are administrators; the test must not depend on who runs it
	view := m.render()
	for _, want := range []string{"INSPECT", "NETWORK", "WINDOWS", "SETTINGS", "standard user"} {
		if !strings.Contains(view, want) {
			t.Errorf("collapsed sidebar is missing %q", want)
		}
	}
	if strings.Contains(view, "Processes") {
		t.Error("tools of a collapsed area should be hidden")
	}
	for i, e := range m.entries {
		switch e.id {
		case "inspect.processes":
			m.cursor = i
		}
	}
	if view := m.render(); !strings.Contains(view, "Processes") || !strings.Contains(view, "[R ]") {
		t.Error("the selected tool's area should be expanded with its badge")
	}
	for i, e := range m.entries {
		if e.id == "net.flushdns" {
			m.cursor = i
		}
	}
	if view := m.render(); !strings.Contains(view, "Flush DNS") || !strings.Contains(view, "[CA]") {
		t.Error("network area should expand and show the admin-required badge")
	}
}

func TestTooSmallTerminalShowsGuardMessage(t *testing.T) {
	m := newModel(app.Services{}, "test")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	if !strings.Contains(next.(model).render(), "Terminal too small") {
		t.Fatal("expected the small-terminal message")
	}
}

func TestOpeningToolLoadsTableAndFilterNarrowsRows(t *testing.T) {
	m := sized(app.Services{Processes: fakeProcesses{}})
	m.cursor = 2 // Processes
	m = press(m, tea.Key{Code: tea.KeyEnter})
	if view := m.render(); !strings.Contains(view, "alpha.exe") || !strings.Contains(view, "beta.exe") {
		t.Fatalf("process rows missing:\n%s", view)
	}
	m = press(m, tea.Key{Text: "/", Code: '/'})
	m = typed(m, "beta")
	m = press(m, tea.Key{Code: tea.KeyEnter})
	view := m.render()
	if strings.Contains(view, "alpha.exe") || !strings.Contains(view, "1 of 2") {
		t.Fatalf("filter did not narrow rows:\n%s", view)
	}
}

func TestCollectionErrorIsNotShownAsEmpty(t *testing.T) {
	m := sized(app.Services{Processes: failingProcesses{}})
	m.cursor = 2
	m = press(m, tea.Key{Code: tea.KeyEnter})
	view := m.render()
	if !strings.Contains(view, "Access is denied") || strings.Contains(view, "were found") {
		t.Fatalf("error state not shown:\n%s", view)
	}
}

func TestPaletteFindsToolBySymptomAndOpensActionFlow(t *testing.T) {
	m := sized(app.Services{})
	m = press(m, tea.Key{Code: 'k', Mod: tea.ModCtrl})
	m = typed(m, "wrong website")
	if !strings.Contains(m.render(), "Flush DNS cache") {
		t.Fatal("palette did not match the symptom keyword")
	}
	m = press(m, tea.Key{Code: tea.KeyEnter})
	if m.flow == nil {
		t.Fatal("selecting an action tool did not start the action flow")
	}
}

func TestActionFlowRefusesToContinueWithoutAdministrator(t *testing.T) {
	m := sized(app.Services{})
	m.elevated = false
	for _, e := range m.entries {
		if e.id == "net.flushdns" {
			m.flow = newFlow(*e.tool, m.cfg)
		}
	}
	m.flow.elev = false
	m = press(m, tea.Key{Code: tea.KeyEnter})
	if m.flow == nil || m.flow.step != stepReview {
		t.Fatal("flow advanced past Review without administrator rights")
	}
	if !strings.Contains(m.render(), "Administrator rights are required") {
		t.Fatal("review step does not explain the administrator requirement")
	}
}
