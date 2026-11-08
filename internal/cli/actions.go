package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/MatheusCampagnolo/wintonicz/internal/actions"
	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/catalog"
	"github.com/MatheusCampagnolo/wintonicz/internal/config"
	"github.com/MatheusCampagnolo/wintonicz/internal/privilege"
	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
	"github.com/spf13/cobra"
)

// Exit codes (see the CLI contract in the project plan).
const (
	exitError      = 1
	exitUsage      = 2
	exitNeedsAdmin = 3
	exitNotRun     = 4
	exitChildFail  = 10
)

// buildVersion is shown in JSON metadata; set by NewRootCommand.
var buildVersion = "dev"

// ExitError carries a specific process exit code through cobra to main.
type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string { return e.Err.Error() }
func (e ExitError) Unwrap() error { return e.Err }
func (e ExitError) ExitCode() int { return e.Code }

// silent is an error whose message is empty; Execute prints nothing for it.
type silent struct{}

func (silent) Error() string { return "" }

// JSON v2 envelope. Success and failure share one shape so a script reads one channel.
type envelopeV2 struct {
	SchemaVersion int        `json:"schemaVersion"`
	OK            bool       `json:"ok"`
	Tool          string     `json:"tool,omitempty"`
	Command       string     `json:"command"`
	Data          any        `json:"data,omitempty"`
	Warnings      []warning  `json:"warnings,omitempty"`
	Error         *errorBody `json:"error,omitempty"`
	Meta          metaBody   `json:"meta"`
}

type warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

type metaBody struct {
	Elevated bool   `json:"elevated"`
	Version  string `json:"version"`
}

func writeEnvelope(w io.Writer, env envelopeV2) error {
	env.SchemaVersion = 2
	env.Meta = metaBody{Elevated: privilege.IsElevated(), Version: buildVersion}
	if env.Tool == "" {
		env.Tool = toolForCommand(env.Command)
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(env)
}

// writeJSON writes a successful result for a read command.
func writeJSON(w io.Writer, command string, data any) error {
	return writeEnvelope(w, envelopeV2{OK: true, Command: command, Data: data})
}

func toolForCommand(command string) string {
	for _, tool := range catalog.All() {
		if strings.Join(tool.Command, " ") == command {
			return tool.ID
		}
	}
	return ""
}

func jsonDefault() bool {
	cfg, _ := config.Load()
	return cfg.JSONDefault
}

// Execute runs the root command and returns the process exit code. With --json (or
// jsonDefault) failures are written to stdout as an envelope; otherwise to stderr.
func Execute(root *cobra.Command, args []string, stdout, stderr io.Writer) int {
	err := root.Execute()
	if err == nil {
		return 0
	}
	var exit interface{ ExitCode() int }
	code := exitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if isUsageError(err) {
		code = exitUsage
	}
	if err.Error() == "" {
		return code // already reported
	}
	if wantsJSON(args) {
		_ = writeEnvelope(stdout, envelopeV2{OK: false, Command: strings.Join(nonFlags(args), " "),
			Error: &errorBody{Code: errorCode(code), Message: err.Error()}})
		return code
	}
	fmt.Fprintln(stderr, "Error:", err)
	return code
}

func isUsageError(err error) bool {
	msg := err.Error()
	for _, prefix := range []string{"unknown command", "unknown flag", "unknown shorthand", "invalid argument", "accepts ", "requires at least", "flag needs", "required flag"} {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	return false
}

func errorCode(code int) string {
	switch code {
	case exitUsage:
		return "usage"
	case exitNeedsAdmin:
		return "needs_admin"
	case exitNotRun:
		return "not_run"
	case exitChildFail:
		return "command_failed"
	}
	return "error"
}

func wantsJSON(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "--json=true" {
			return true
		}
	}
	return jsonDefault()
}

func nonFlags(args []string) []string {
	var out []string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			out = append(out, arg)
		}
	}
	return out
}

func newToolsCommand(services app.Services) *cobra.Command {
	command := &cobra.Command{Use: "tools", Short: "Describe the tools this build offers (for scripts and AI agents)", Args: cobra.NoArgs}
	var asJSON bool
	var area string
	list := &cobra.Command{
		Use:   "list",
		Short: "List every tool with its risk, administrator need, and command",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var tools []catalog.Tool
			for _, tool := range catalog.All() {
				if area == "" || string(tool.Area) == area {
					tools = append(tools, tool)
				}
			}
			if asJSON {
				return writeEnvelope(cmd.OutOrStdout(), envelopeV2{OK: true, Tool: "tools.list", Command: "tools list", Data: tools})
			}
			for _, tool := range tools {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-8s risk=%-11s admin=%-8s %s\n", tool.ID, tool.Area, tool.Risk, tool.Admin, tool.Title)
			}
			return nil
		},
	}
	list.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	list.Flags().StringVar(&area, "area", "", "Only tools in this area, such as network")

	describe := &cobra.Command{
		Use:   "describe <tool-id>",
		Short: "Describe one tool, including the exact command an action would run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, ok := catalog.Find(args[0])
			if !ok {
				return ExitError{Code: exitUsage, Err: fmt.Errorf("no tool with ID %q; run `tools list`", args[0])}
			}
			data := map[string]any{"tool": tool}
			if tool.Kind == catalog.KindAction {
				if spec, ok := actions.Specs(services, buildVersion)[tool.ID]; ok && len(spec.Params) == 0 {
					if plan, err := spec.Build(map[string]string{}); err == nil {
						data["preview"] = plan.Preview
					}
				}
			}
			if asJSON {
				return writeEnvelope(cmd.OutOrStdout(), envelopeV2{OK: true, Tool: "tools.describe", Command: "tools describe", Data: data})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n%s\nRisk: %s · Admin: %s · Duration: %s\n", tool.ID, tool.Title, tool.Summary, tool.Risk, tool.Admin, tool.Duration)
			if p, ok := data["preview"].(runner.Preview); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Command: %s\nUndo: %s\n", p.Command, p.Undo)
			}
			return nil
		},
	}
	describe.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	command.AddCommand(list, describe)
	return command
}

// runAction is the shared CLI path for every action tool: preview, gate, run, report.
func runAction(cmd *cobra.Command, commandName string, build func() (runner.Plan, error), asJSON, dryRun, yes bool) error {
	out := cmd.OutOrStdout()
	cfg, _ := config.Load()
	readOnly := cfg.ReadOnly
	if flag, err := cmd.Flags().GetBool("read-only"); err == nil && flag {
		readOnly = true
	}
	plan, err := build()
	if err != nil {
		return actionFailure(cmd, "", commandName, "error", err, "", exitError, asJSON)
	}
	if dryRun || !yes {
		if asJSON {
			if err := writeEnvelope(out, envelopeV2{OK: true, Tool: plan.Tool, Command: commandName, Data: map[string]any{"preview": plan.Preview, "ran": false}}); err != nil {
				return err
			}
		} else {
			p := plan.Preview
			fmt.Fprintf(out, "%s\n%s\nCommand: %s\nAdministrator: %s\nDuration: %s\nUndo: %s\n", p.Title, p.Summary, p.Command, yesNo(p.Admin, "required", "not needed"), p.Duration, p.Undo)
			if !dryRun {
				fmt.Fprintln(out, "Nothing was run. Add --yes to run it.")
			}
		}
		if dryRun {
			return nil
		}
		return ExitError{Code: exitNotRun, Err: silentIf(asJSON, errors.New("not run: add --yes to confirm"))}
	}

	opts := runner.Options{ReadOnly: readOnly, NoLog: !cfg.ActionLog}
	if !asJSON {
		opts.OnLine = func(line runner.Line) { fmt.Fprintln(out, line.Text) }
	}
	result, err := runner.Run(cmd.Context(), plan, opts)
	switch {
	case errors.Is(err, runner.ErrNeedsAdmin):
		return actionFailure(cmd, plan.Tool, commandName, "needs_admin", err, "Run from an administrator terminal, or use the launcher with -Admin.", exitNeedsAdmin, asJSON)
	case errors.Is(err, runner.ErrReadOnly):
		return actionFailure(cmd, plan.Tool, commandName, "read_only", err, "Turn it off with: wintonicz config set readOnly false", exitNotRun, asJSON)
	case err != nil:
		return actionFailure(cmd, plan.Tool, commandName, "start_failed", err, "", exitError, asJSON)
	}
	if asJSON {
		if err := writeEnvelope(out, envelopeV2{OK: result.ExitCode == 0, Tool: plan.Tool, Command: commandName, Data: result}); err != nil {
			return err
		}
		if result.ExitCode != 0 {
			return ExitError{Code: exitChildFail, Err: silent{}}
		}
		return nil
	}
	if result.ExitCode != 0 {
		return ExitError{Code: exitChildFail, Err: fmt.Errorf("%s exited with status %d", plan.Tool, result.ExitCode)}
	}
	return nil
}

func silentIf(quiet bool, err error) error {
	if quiet {
		return silent{}
	}
	return err
}

// actionFailure reports a failure on the channel the caller asked for. In JSON mode
// the envelope goes to stdout and the returned error is silent on stderr.
func actionFailure(cmd *cobra.Command, tool, commandName, code string, err error, hint string, exit int, asJSON bool) error {
	if asJSON {
		if werr := writeEnvelope(cmd.OutOrStdout(), envelopeV2{OK: false, Tool: tool, Command: commandName, Error: &errorBody{Code: code, Message: err.Error(), Hint: hint}}); werr != nil {
			return werr
		}
		return ExitError{Code: exit, Err: silent{}}
	}
	if hint != "" {
		err = fmt.Errorf("%w. %s", err, hint)
	}
	return ExitError{Code: exit, Err: err}
}

func yesNo(value bool, yes, no string) string {
	if value {
		return yes
	}
	return no
}
