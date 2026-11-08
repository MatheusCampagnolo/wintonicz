package cli

import (
	"fmt"
	"strings"

	"github.com/MatheusCampagnolo/wintonicz/internal/actions"
	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/catalog"
	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
	"github.com/spf13/cobra"
)

// registerActions adds one command per action tool, at the path its catalog entry gives.
// Every action command has --dry-run, --yes, and --json and goes through runAction.
func registerActions(root *cobra.Command, services app.Services) {
	specs := actions.Specs(services)
	for _, tool := range catalog.All() {
		spec, ok := specs[tool.ID]
		if tool.Kind != catalog.KindAction || !ok || len(tool.Command) == 0 {
			continue
		}
		parent := root
		for _, name := range tool.Command[:len(tool.Command)-1] {
			parent = childNamed(parent, name)
		}
		parent.AddCommand(actionCommand(tool, spec))
	}
}

func childNamed(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	c := &cobra.Command{Use: name, Short: strings.ToUpper(name[:1]) + name[1:] + " tools", Args: cobra.NoArgs}
	parent.AddCommand(c)
	return c
}

func actionCommand(tool catalog.Tool, spec runner.Spec) *cobra.Command {
	var asJSON, dryRun, yes bool
	values := map[string]*string{}
	var positional string
	required := 0
	for _, p := range spec.Params {
		if p.Required {
			required++
			positional = p.Name
		}
	}
	use := tool.Command[len(tool.Command)-1]
	args := cobra.NoArgs
	if required == 1 {
		use += " [" + positional + "]"
		args = cobra.MaximumNArgs(1)
	}
	short := tool.Summary
	if tool.Admin == catalog.AdminRequired {
		short += " (administrator)"
	}
	command := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  args,
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			params := map[string]string{}
			for name, v := range values {
				params[name] = *v
			}
			if len(posArgs) == 1 {
				params[positional] = posArgs[0]
			}
			for _, p := range spec.Params {
				if p.Required && strings.TrimSpace(params[p.Name]) == "" {
					return ExitError{Code: exitUsage, Err: fmt.Errorf("%s is required (use --%s)", p.Label, p.Name)}
				}
				if len(p.Choices) > 0 && params[p.Name] != "" && !contains(p.Choices, params[p.Name]) {
					return ExitError{Code: exitUsage, Err: fmt.Errorf("--%s must be one of: %s", p.Name, strings.Join(p.Choices, ", "))}
				}
			}
			name := strings.Join(tool.Command, " ")
			return runAction(cmd, name, func() (runner.Plan, error) { return spec.Build(params) }, asJSON, dryRun, yes)
		},
	}
	for _, p := range spec.Params {
		v := new(string)
		values[p.Name] = v
		help := p.Help
		if len(p.Choices) > 0 {
			help += " (" + strings.Join(p.Choices, "|") + ")"
		}
		command.Flags().StringVar(v, p.Name, p.Default, help)
	}
	command.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would run and stop")
	command.Flags().BoolVar(&yes, "yes", false, "Run without asking; without it only the preview is shown")
	return command
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
