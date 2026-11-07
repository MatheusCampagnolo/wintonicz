package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestWelcomeViewExplainsCurrentState(t *testing.T) {
	content := newModel().View().Content
	for _, expected := range []string{"Wintonicz", "Diagnostic pages are not available yet", "quit"} {
		if !strings.Contains(content, expected) {
			t.Errorf("welcome view does not contain %q", expected)
		}
	}
}

func TestHelpKeyTogglesFullHelp(t *testing.T) {
	current := newModel()
	updated, _ := current.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	if !updated.(model).help.ShowAll {
		t.Fatal("help state = false after pressing ?")
	}
}

func TestQuitKeyReturnsQuitCommand(t *testing.T) {
	_, command := newModel().Update(tea.KeyPressMsg(tea.Key{Text: "q", Code: 'q'}))
	if command == nil {
		t.Fatal("quit command = nil after pressing q")
	}
}
