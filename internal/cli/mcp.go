package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/MatheusCampagnolo/wintonicz/internal/catalog"
	"github.com/MatheusCampagnolo/wintonicz/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "mcp",
		Short: "Serve the read-only tools to an AI client over MCP (stdio)",
		Long: "Starts a Model Context Protocol server on standard input and output. It exposes only tools whose risk is " +
			"\"read\"; every call runs this program with --read-only. Nothing is written to standard output except protocol messages.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if readOnly, _ := cmd.Flags().GetBool("read-only"); !readOnly {
				return ExitError{Code: exitUsage, Err: errors.New("only --read-only is supported; pass --read-only")}
			}
			self, err := os.Executable()
			if err != nil {
				return err
			}
			run := func(ctx context.Context, argv []string) (string, int, error) {
				child := exec.CommandContext(ctx, self, argv...)
				child.Env = append(os.Environ(), "WTZ_ADMIN=0")
				var stdout bytes.Buffer
				child.Stdout = &stdout
				err := child.Run()
				var exit *exec.ExitError
				if errors.As(err, &exit) {
					return stdout.String(), exit.ExitCode(), nil
				}
				return stdout.String(), 0, err
			}
			return mcp.Serve(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), mcp.Tools(catalog.All()), run, buildVersion)
		},
	}
	return command
}
