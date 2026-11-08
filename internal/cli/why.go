package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/why"
	"github.com/spf13/cobra"
)

func whyDeps(s app.Services) why.Deps {
	return why.Deps{Processes: s.Processes, Ports: s.Ports, Services: s.Services, Startup: s.Startup, Chain: s.Chain}
}

func newWhyCommand(services app.Services) *cobra.Command {
	var asJSON bool
	var port int
	command := &cobra.Command{
		Use:   "why <pid|name|:port>",
		Short: "Explain why a process is running (ancestry, service, startup entry, ports)",
		Long: "Gathers what Windows exposes about a process and says what it supports. Each conclusion is marked " +
			"confirmed or inferred; what cannot be observed is reported as unknown.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			switch {
			case port != 0:
				target = ":" + strconv.Itoa(port)
			case len(args) == 1:
				target = args[0]
			default:
				return ExitError{Code: exitUsage, Err: fmt.Errorf("give a PID, a process name, :port, or --port")}
			}
			deps := whyDeps(services)
			pid, err := why.ResolvePID(deps, target)
			if err != nil {
				return err
			}
			report, err := why.Investigate(deps, target, pid)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "why", report)
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(why.Lines(report), "\n"))
			return nil
		},
	}
	command.Flags().IntVar(&port, "port", 0, "Investigate the process listening on this TCP port")
	command.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	return command
}
