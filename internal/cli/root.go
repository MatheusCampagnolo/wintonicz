package cli

import (
	"os"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/spf13/cobra"
)

// NewRootCommand creates the command-line entry point and starts the TUI only
// when both standard input and output are connected to an interactive terminal.
func NewRootCommand(version string, runTUI func() error, services app.Services) *cobra.Command {
	root := &cobra.Command{
		Use:           "wintonicz",
		Short:         "Windows diagnostics and support toolbox",
		Long:          "Wintonicz is a Windows support toolbox. Run without arguments in an interactive terminal to open its terminal interface.",
		Args:          cobra.NoArgs,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !hasInteractiveTerminal(cmd) {
				return cmd.Help()
			}
			return runTUI()
		},
	}

	root.SetVersionTemplate("Wintonicz {{.Version}}\n")
	root.AddCommand(
		newProcessCommand(services),
		newPortCommand(services),
		newSystemCommand(services),
		newServiceCommand(services),
		newUSBCommand(services),
		newNetworkCommand(services),
		newPanelCommand(services),
	)
	return root
}

func hasInteractiveTerminal(cmd *cobra.Command) bool {
	input, inputIsFile := cmd.InOrStdin().(*os.File)
	output, outputIsFile := cmd.OutOrStdout().(*os.File)
	if !inputIsFile || !outputIsFile {
		return false
	}

	inputInfo, inputErr := input.Stat()
	outputInfo, outputErr := output.Stat()
	if inputErr != nil || outputErr != nil {
		return false
	}

	return inputInfo.Mode()&os.ModeCharDevice != 0 && outputInfo.Mode()&os.ModeCharDevice != 0
}
