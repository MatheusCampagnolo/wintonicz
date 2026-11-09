package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MatheusCampagnolo/wintonicz/internal/catalog"
)

func TestOnlyReadToolsAreExposed(t *testing.T) {
	names := map[string]bool{}
	for _, tool := range Tools(catalog.All()) {
		names[tool.Name] = true
	}
	for _, want := range []string{"inspect_system", "inspect_why", "power_list", "undo_list"} {
		if !names[want] {
			t.Errorf("%s should be exposed", want)
		}
	}
	for _, banned := range []string{"net_flushdns", "repair_sfc", "clean_temp", "undo", "power_set", "tools_process_end", "win_panels"} {
		if names[banned] {
			t.Errorf("%s must not be exposed", banned)
		}
	}
}

func TestArgvRefusesFlagsAndForcesReadOnly(t *testing.T) {
	tool := Tool{Name: "inspect_why", Command: []string{"why"}}
	if _, err := Argv(tool, []string{"--elevated"}); err == nil {
		t.Error("a flag-shaped argument must be refused")
	}
	if _, err := Argv(tool, []string{"a\nb"}); err == nil {
		t.Error("control characters must be refused")
	}
	argv, err := Argv(tool, []string{"explorer.exe"})
	if err != nil || strings.Join(argv, " ") != "why explorer.exe --json --read-only" {
		t.Errorf("argv = %v, %v", argv, err)
	}
	act := Tool{Command: []string{"power", "list"}, Action: true}
	if argv, _ := Argv(act, nil); argv[len(argv)-1] != "--yes" {
		t.Errorf("read-only actions need --yes: %v", argv)
	}
}

func TestServeHandshakeListAndCall(t *testing.T) {
	tools := []Tool{{Name: "inspect_system", Description: "d", Command: []string{"system", "info"}}}
	var got []string
	run := func(_ context.Context, argv []string) (string, int, error) {
		got = argv
		return `{"ok":true}`, 0, nil
	}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"inspect_system","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"repair_sfc"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"nope"}`,
		`not json`,
	}, "\n")
	var out bytes.Buffer
	if err := Serve(context.Background(), strings.NewReader(in), &out, tools, run, "test"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 { // the notification gets no reply
		t.Fatalf("got %d replies:\n%s", len(lines), out.String())
	}
	var call struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &call); err != nil || call.Result.IsError {
		t.Errorf("call reply = %s", lines[2])
	}
	if strings.Join(got, " ") != "system info --json --read-only" {
		t.Errorf("argv = %v", got)
	}
	for i, wantCode := range map[int]string{3: "-32602", 4: "-32601", 5: "-32700"} {
		if !strings.Contains(lines[i], wantCode) {
			t.Errorf("reply %d should carry %s: %s", i, wantCode, lines[i])
		}
	}
}
