package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/panels"
	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/system"
)

type fixtureServices struct {
	processItems []processes.Entry
	listeners    []ports.Listener
}

func (f fixtureServices) ListProcesses() ([]processes.Entry, error) {
	return f.processItems, nil
}

func (f fixtureServices) ListListeners() ([]ports.Listener, error) {
	return f.listeners, nil
}

func (fixtureServices) SystemSnapshot() (system.Snapshot, error)    { return system.Snapshot{}, nil }
func (fixtureServices) NetworkAdapters() ([]network.Adapter, error) { return nil, nil }
func (fixtureServices) ListPanels() []panels.Entry                  { return nil }
func (fixtureServices) OpenPanel(string) error                      { return nil }

func TestPortFindWritesVersionedJSON(t *testing.T) {
	services := fixtureServices{listeners: []ports.Listener{
		{Address: "127.0.0.1", Port: 8080, PID: 42, ProcessName: "server.exe"},
		{Address: "0.0.0.0", Port: 9090, PID: 55, ProcessName: "other.exe"},
	}}
	var stdout, stderr bytes.Buffer
	command := NewRootCommand("test", func() error { return nil }, app.Services{
		Processes: services,
		Ports:     services,
		System:    services,
		Network:   services,
		Panels:    services,
	})
	command.SetArgs([]string{"port", "find", "8080", "--json"})
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var output struct {
		SchemaVersion int              `json:"schemaVersion"`
		OK            bool             `json:"ok"`
		Command       string           `json:"command"`
		Data          []ports.Listener `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if output.SchemaVersion != 2 || !output.OK || output.Command != "port find" {
		t.Fatalf("unexpected JSON envelope: %+v", output)
	}
	if len(output.Data) != 1 || output.Data[0].PID != 42 {
		t.Fatalf("unexpected port matches: %+v", output.Data)
	}
}

func TestProcessPortsMatchesExecutableName(t *testing.T) {
	services := fixtureServices{
		processItems: []processes.Entry{{PID: 42, Name: "server.exe"}, {PID: 55, Name: "other.exe"}},
		listeners:    []ports.Listener{{Address: "127.0.0.1", Port: 8080, PID: 42, ProcessName: "server.exe"}},
	}
	var stdout, stderr bytes.Buffer
	command := NewRootCommand("test", func() error { return nil }, app.Services{
		Processes: services,
		Ports:     services,
		System:    services,
		Network:   services,
		Panels:    services,
	})
	command.SetArgs([]string{"process", "ports", "SERVER"})
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "8080") || !strings.Contains(stdout.String(), "server.exe") {
		t.Fatalf("unexpected process port output: %q", stdout.String())
	}
}
