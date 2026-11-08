// Package runner executes allowlisted commands for action tools. It is the only
// place Wintonicz starts a child process: fixed executable under the Windows
// directory, an argument array (never a shell string), streamed output, a
// privilege/read-only gate, and a local log line.
package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/actionlog"
	"github.com/MatheusCampagnolo/wintonicz/internal/privilege"
)

var (
	ErrNeedsAdmin = errors.New("this action requires administrator rights")
	ErrReadOnly   = errors.New("read-only mode is on; actions are disabled")
)

// Preview is what the user sees before anything runs.
type Preview struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Command  string `json:"command"`
	Admin    bool   `json:"adminRequired"`
	Duration string `json:"duration"`
	Restart  bool   `json:"restart"`
	Undo     string `json:"undo"`
}

// Plan is one fully resolved command.
type Plan struct {
	Tool    string
	Exe     string
	Args    []string
	Preview Preview
}

// Line is one line of child output.
type Line struct {
	Stream string `json:"stream"`
	Text   string `json:"text"`
}

// Result describes a finished run.
type Result struct {
	Tool       string `json:"tool"`
	Command    string `json:"command"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
	Elevated   bool   `json:"elevated"`
	Cancelled  bool   `json:"cancelled"`
	LogPath    string `json:"logPath,omitempty"`
}

// Options control one run.
type Options struct {
	ReadOnly bool
	NoLog    bool
	OnLine   func(Line)
}

// CommandLine renders argv for display only; it is never executed as a string.
func CommandLine(exe string, args []string) string {
	parts := append([]string{filepath.Base(exe)}, args...)
	for i, part := range parts {
		if strings.ContainsAny(part, " \t") {
			parts[i] = `"` + part + `"`
		}
	}
	return strings.Join(parts, " ")
}

// Check applies the gates that must pass before a plan may start.
func Check(plan Plan, opts Options) error {
	if opts.ReadOnly {
		return ErrReadOnly
	}
	if plan.Preview.Admin && !privilege.IsElevated() {
		return ErrNeedsAdmin
	}
	return validateExe(plan.Exe)
}

// Run executes the plan and streams output. A non-zero child exit code is a
// Result, not an error; errors mean the command could not be started.
func Run(ctx context.Context, plan Plan, opts Options) (Result, error) {
	if err := Check(plan, opts); err != nil {
		return Result{}, err
	}
	result := Result{Tool: plan.Tool, Command: CommandLine(plan.Exe, plan.Args), Elevated: privilege.IsElevated()}
	started := time.Now()

	cmd := exec.CommandContext(ctx, plan.Exe, plan.Args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return result, err
	}
	if err := cmd.Start(); err != nil {
		return result, fmt.Errorf("start %s: %w", filepath.Base(plan.Exe), err)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	pump := func(name string, reader io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			if opts.OnLine == nil {
				continue
			}
			text := strings.TrimRight(decodeOEM(scanner.Bytes()), "\r")
			mu.Lock()
			opts.OnLine(Line{Stream: name, Text: text})
			mu.Unlock()
		}
	}
	wg.Add(2)
	go pump("stdout", stdout)
	go pump("stderr", stderr)
	wg.Wait()
	waitErr := cmd.Wait()

	result.DurationMS = time.Since(started).Milliseconds()
	var exitErr *exec.ExitError
	switch {
	case waitErr == nil:
	case errors.As(waitErr, &exitErr):
		result.ExitCode = exitErr.ExitCode()
	default:
		result.ExitCode = 1
	}
	if ctx.Err() != nil {
		result.Cancelled = true
	}

	outcome := "completed"
	if result.Cancelled {
		outcome = "cancelled"
	} else if result.ExitCode != 0 {
		outcome = "failed"
	}
	if opts.NoLog {
		return result, nil
	}
	result.LogPath = actionlog.Append(actionlog.Entry{
		Time: started, Tool: plan.Tool, Command: result.Command, Elevated: result.Elevated,
		ExitCode: result.ExitCode, DurationMS: result.DurationMS, Outcome: outcome,
	})
	return result, nil
}
