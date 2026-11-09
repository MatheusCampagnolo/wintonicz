// Package mcp serves Wintonicz's read-only tools over the Model Context Protocol
// (newline-delimited JSON-RPC on stdin/stdout). It exposes only catalog tools whose
// risk is "read" and runs each one by starting this same program with --read-only,
// so the MCP surface can never do more than the CLI's read-only mode.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/MatheusCampagnolo/wintonicz/internal/catalog"
)

const (
	protocolVersion = "2025-06-18"
	maxArgs         = 4
	maxArgLength    = 256
	maxOutput       = 256 * 1024
)

// Tool is one exposed capability.
type Tool struct {
	Name        string
	Description string
	Command     []string
	// Action is true for catalog actions whose risk is read (they need --yes to run).
	Action bool
}

// Runner starts the program with argv and returns its standard output and exit code.
type Runner func(ctx context.Context, argv []string) (output string, exitCode int, err error)

// Tools selects the read-only catalog tools that have a command.
func Tools(all []catalog.Tool) []Tool {
	var out []Tool
	for _, t := range all {
		if t.Risk != catalog.RiskRead || len(t.Command) == 0 || t.Kind == catalog.KindLaunch {
			continue
		}
		desc := t.Summary
		if t.Admin == catalog.AdminOptional {
			desc += " Shows more when Wintonicz runs as administrator."
		}
		out = append(out, Tool{Name: strings.ReplaceAll(t.ID, ".", "_"), Description: desc,
			Command: t.Command, Action: t.Kind == catalog.KindAction})
	}
	return out
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve reads requests until the input closes or ctx ends.
func Serve(ctx context.Context, in io.Reader, out io.Writer, tools []Tool, run Runner, version string) error {
	byName := map[string]Tool{}
	for _, t := range tools {
		byName[t.Name] = t
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	encoder := json.NewEncoder(out)
	reply := func(r response) error {
		r.JSONRPC = "2.0"
		return encoder.Encode(r)
	}
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := reply(response{ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}}); err != nil {
				return err
			}
			continue
		}
		if len(req.ID) == 0 {
			continue // a notification, such as notifications/initialized: no reply
		}
		result, rerr := handle(ctx, req, tools, byName, run, version)
		if err := reply(response{ID: req.ID, Result: result, Error: rerr}); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func handle(ctx context.Context, req request, tools []Tool, byName map[string]Tool, run Runner, version string) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "wintonicz", "version": version},
			"instructions":    "Read-only Windows diagnostics. Nothing here changes the system. Each tool returns Wintonicz's JSON envelope; check ok and error.",
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		list := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			list = append(list, map[string]any{
				"name": t.Name, "description": t.Description,
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{"args": map[string]any{
						"type": "array", "items": map[string]any{"type": "string"}, "maxItems": maxArgs,
						"description": "Positional arguments such as a process name, PID, host, or port. Flags are not accepted.",
					}},
					"additionalProperties": false,
				},
				"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "openWorldHint": false},
			})
		}
		return map[string]any{"tools": list}, nil
	case "tools/call":
		var p struct {
			Name      string `json:"name"`
			Arguments struct {
				Args []string `json:"args"`
			} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		tool, ok := byName[p.Name]
		if !ok {
			return nil, &rpcError{-32602, fmt.Sprintf("unknown tool %q", p.Name)}
		}
		argv, err := Argv(tool, p.Arguments.Args)
		if err != nil {
			return toolResult(err.Error(), true), nil
		}
		callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		output, code, err := run(callCtx, argv)
		if err != nil {
			return toolResult("could not run the tool: "+err.Error(), true), nil
		}
		if len(output) > maxOutput {
			output = output[:maxOutput] + "\n[output truncated]"
		}
		return toolResult(output, code != 0), nil
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{"content": []map[string]string{{"type": "text", "text": text}}, "isError": isError}
}

// Argv builds the child command line. Arguments are positional values only: a value
// that starts with "-" could switch on a flag, so it is refused.
func Argv(t Tool, args []string) ([]string, error) {
	if len(args) > maxArgs {
		return nil, fmt.Errorf("at most %d arguments are accepted", maxArgs)
	}
	argv := append([]string{}, t.Command...)
	for _, a := range args {
		switch {
		case a == "" || len(a) > maxArgLength:
			return nil, errors.New("arguments must be 1 to 256 characters")
		case strings.HasPrefix(a, "-"):
			return nil, fmt.Errorf("argument %q looks like a flag; only plain values are accepted", a)
		case strings.IndexFunc(a, unicode.IsControl) >= 0:
			return nil, errors.New("arguments may not contain control characters")
		}
		argv = append(argv, a)
	}
	argv = append(argv, "--json", "--read-only")
	if t.Action {
		argv = append(argv, "--yes")
	}
	return argv, nil
}
