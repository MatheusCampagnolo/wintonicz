package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
)

func TestHomeViewShowsGroupedNavigationAndHelp(t *testing.T) {
	current := newModel(app.Services{})
	current.width, current.height = 100, 24
	content := current.View().Content
	for _, expected := range []string{"Wintonicz", "Overview", "Diagnostics", "Windows tools", "navigate", "quit"} {
		if !strings.Contains(content, expected) {
			t.Errorf("home view does not contain %q", expected)
		}
	}
}

func TestHelpKeyTogglesFullHelp(t *testing.T) {
	current := newModel(app.Services{})
	updated, _ := current.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	if !updated.(model).help.ShowAll {
		t.Fatal("help state = false after pressing ?")
	}
}

func TestQuitKeyReturnsQuitCommand(t *testing.T) {
	_, command := newModel(app.Services{}).Update(tea.KeyPressMsg(tea.Key{Text: "q", Code: 'q'}))
	if command == nil {
		t.Fatal("quit command = nil after pressing q")
	}
}

func TestHomeTabShowsToolsAndEnterOpensSelectedTool(t *testing.T) {
	current := newModel(app.Services{})
	updated, _ := current.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	current = updated.(model)
	if current.homeSectionPage() != diagnosticsMenuPage {
		t.Fatal("Tab did not switch the active Home area to Diagnostics")
	}
	if !strings.Contains(current.menuView(), "Processes") {
		t.Fatal("Diagnostics tools were not shown on Home")
	}
	updated, _ = current.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	updated, command := updated.(model).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if updated.(model).page != portsPage {
		t.Fatal("Enter did not open the selected TCP ports tool")
	}
	if command == nil {
		t.Fatal("opening a diagnostic tool did not request its data")
	}
}

func TestChangingTableColumnLayoutsDoesNotPanic(t *testing.T) {
	current := newModel(app.Services{})
	current.width, current.height = 100, 24
	current.page = processesPage
	current.processes = []processes.Entry{{PID: 12, ParentID: 1, Name: "sample.exe"}}
	current.setTableRows()

	current.page = networkPage
	current.adapters = []network.Adapter{{Index: 1, Name: "Ethernet", Status: "up", Addresses: []string{"192.0.2.2/24"}}}
	current.setTableRows()

	if !strings.Contains(current.table.View(), "Ethernet") {
		t.Fatalf("network table missing row after changing columns: %q", current.table.View())
	}
}

func TestCollectionErrorWrapsToFitTerminalWidth(t *testing.T) {
	current := newModel(app.Services{})
	current.width, current.height = 72, 24
	current.loading = false
	current.err = errors.New("read TCP listeners for address family 2: Access is denied")

	if view := current.pageView(); !strings.Contains(view, "\n") {
		t.Fatalf("collection error should wrap within the terminal width, got %q", view)
	}
}
