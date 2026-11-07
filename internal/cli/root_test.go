package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlagPrintsBuildVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	command := NewRootCommand("test-build", func() error {
		t.Fatal("version flag must not start the TUI")
		return nil
	})
	command.SetArgs([]string{"--version"})
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "Wintonicz test-build" {
		t.Fatalf("version output = %q", got)
	}
}

func TestNoArgumentsWithRedirectedStreamsPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	startedTUI := false
	command := NewRootCommand("dev", func() error {
		startedTUI = true
		return nil
	})
	command.SetArgs(nil)
	command.SetIn(strings.NewReader(""))
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if startedTUI {
		t.Fatal("redirected streams must not start the TUI")
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("help output missing usage: %q", stdout.String())
	}
}

func TestUnknownCommandReturnsError(t *testing.T) {
	command := NewRootCommand("dev", func() error { return nil })
	command.SetArgs([]string{"unknown"})

	if err := command.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want unknown-command error")
	}
}
