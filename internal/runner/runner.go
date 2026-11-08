// Package runner executes the actions of action tools. It is the only place
// Wintonicz starts a child process or performs a state-changing operation:
// fixed executables from known Windows locations, argument arrays (never a shell
// string), a privilege/read-only gate, streamed output, cancellation, and a local
// log line. In-process operations (Plan.Do) get the same gates and logging.
package runner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

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
	// Items lists what will be touched (files, a process, a folder), when that is known up front.
	Items []string `json:"items,omitempty"`
	// ConfirmWord, when set, must be typed to confirm (destructive actions).
	ConfirmWord string `json:"confirmWord,omitempty"`
}

// Plan is one fully resolved operation: a command, an ordered list of steps, or an
// in-process function.
type Plan struct {
	Tool    string
	Exe     string
	Args    []string
	Steps   []Plan
	Do      func(ctx context.Context, emit func(string)) error
	Preview Preview
	// ReadOnlyOK marks plans that never change anything, so read-only mode lets them run.
	ReadOnlyOK bool
}

// Line is one line of output.
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
	if opts.ReadOnly && !plan.ReadOnlyOK {
		return ErrReadOnly
	}
	if plan.Preview.Admin && !privilege.IsElevated() {
		return ErrNeedsAdmin
	}
	return checkExes(plan)
}

func checkExes(plan Plan) error {
	if plan.Exe != "" {
		if err := validateExe(plan.Exe); err != nil {
			return err
		}
	}
	for _, step := range plan.Steps {
		if err := checkExes(step); err != nil {
			return err
		}
	}
	return nil
}

// Run executes the plan and streams output. A non-zero exit code is a Result, not
// an error; errors mean it could not start.
func Run(ctx context.Context, plan Plan, opts Options) (Result, error) {
	if err := Check(plan, opts); err != nil {
		return Result{}, err
	}
	command := plan.Preview.Command
	if command == "" && plan.Exe != "" {
		command = CommandLine(plan.Exe, plan.Args)
	}
	result := Result{Tool: plan.Tool, Command: command, Elevated: privilege.IsElevated()}
	started := time.Now()

	var mu sync.Mutex
	emit := func(stream, text string) {
		if opts.OnLine == nil {
			return
		}
		mu.Lock()
		opts.OnLine(Line{Stream: stream, Text: text})
		mu.Unlock()
	}

	code, err := execute(ctx, plan, emit)
	if err != nil && !errors.Is(err, context.Canceled) {
		if code == 0 {
			code = 1
		}
		emit("stderr", err.Error())
	}
	result.ExitCode = code
	result.DurationMS = time.Since(started).Milliseconds()
	result.Cancelled = ctx.Err() != nil

	if opts.NoLog {
		return result, nil
	}
	outcome := "completed"
	if result.Cancelled {
		outcome = "cancelled"
	} else if result.ExitCode != 0 {
		outcome = "failed"
	}
	result.LogPath = actionlog.Append(actionlog.Entry{
		Time: started, Tool: plan.Tool, Command: result.Command, Elevated: result.Elevated,
		ExitCode: result.ExitCode, DurationMS: result.DurationMS, Outcome: outcome,
	})
	return result, nil
}

func execute(ctx context.Context, plan Plan, emit func(stream, text string)) (int, error) {
	switch {
	case plan.Do != nil:
		if err := plan.Do(ctx, func(s string) { emit("stdout", s) }); err != nil {
			return 1, err
		}
		return 0, nil
	case len(plan.Steps) > 0:
		for i, step := range plan.Steps {
			if ctx.Err() != nil {
				return 1, ctx.Err()
			}
			emit("stdout", fmt.Sprintf("── step %d of %d: %s", i+1, len(plan.Steps), step.Preview.Command))
			code, err := execute(ctx, step, emit)
			if err != nil || code != 0 {
				return code, err
			}
		}
		return 0, nil
	}
	return runProcess(ctx, plan, emit)
}

func runProcess(ctx context.Context, plan Plan, emit func(stream, text string)) (int, error) {
	cmd := exec.CommandContext(ctx, plan.Exe, plan.Args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, err
	}
	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("start %s: %w", filepath.Base(plan.Exe), err)
	}
	var wg sync.WaitGroup
	pump := func(name string, r io.Reader) {
		defer wg.Done()
		reader, wide := decodingReader(r)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		scanner.Split(splitLines)
		for scanner.Scan() {
			text := scanner.Bytes()
			if wide {
				emit(name, string(text))
			} else {
				emit(name, decodeOEM(text))
			}
		}
	}
	wg.Add(2)
	go pump("stdout", stdout)
	go pump("stderr", stderr)
	wg.Wait()
	waitErr := cmd.Wait()
	var exitErr *exec.ExitError
	switch {
	case waitErr == nil:
		return 0, nil
	case errors.As(waitErr, &exitErr):
		return exitErr.ExitCode(), nil
	}
	return 1, waitErr
}

// splitLines splits on \n, \r\n, or a lone \r (progress lines such as sfc's), and drops empty lines.
func splitLines(data []byte, atEOF bool) (int, []byte, error) {
	start := 0
	for start < len(data) && (data[start] == '\r' || data[start] == '\n') {
		start++
	}
	if start == len(data) {
		return start, nil, nil
	}
	i := bytes.IndexAny(data[start:], "\r\n")
	if i < 0 {
		if atEOF {
			return len(data), data[start:], nil
		}
		return start, nil, nil
	}
	return start + i + 1, data[start : start+i], nil
}

// decodingReader detects UTF-16LE output (sfc writes it when piped) and converts it to UTF-8.
func decodingReader(r io.Reader) (io.Reader, bool) {
	br := bufio.NewReader(r)
	head, _ := br.Peek(2)
	if len(head) == 2 && head[1] == 0 && head[0] != 0 || len(head) == 2 && head[0] == 0xFF && head[1] == 0xFE {
		return &utf16Reader{src: br}, true
	}
	return br, false
}

type utf16Reader struct {
	src     io.Reader
	pending []byte
	out     []byte
}

func (u *utf16Reader) Read(p []byte) (int, error) {
	for len(u.out) == 0 {
		buf := make([]byte, 4096)
		n, err := u.src.Read(buf)
		data := append(u.pending, buf[:n]...)
		u.pending = nil
		if len(data)%2 == 1 {
			u.pending = []byte{data[len(data)-1]}
			data = data[:len(data)-1]
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			units[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
		}
		if len(units) > 0 && units[0] == 0xFEFF {
			units = units[1:]
		}
		u.out = []byte(string(utf16.Decode(units)))
		if err != nil && len(u.out) == 0 {
			return 0, err
		}
	}
	n := copy(p, u.out)
	u.out = u.out[n:]
	return n, nil
}

// ParamSpec describes one input of an action. Inputs are always text; the builder
// validates them before any plan exists.
type ParamSpec struct {
	Name     string
	Label    string
	Help     string
	Default  string
	Required bool
	Choices  []string
}

// Spec is how an action tool turns validated parameters into a Plan.
type Spec struct {
	Params []ParamSpec
	Build  func(params map[string]string) (Plan, error)
}
