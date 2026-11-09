package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/config"
	"github.com/MatheusCampagnolo/wintonicz/internal/privilege"
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
			restartElevatedIfWanted(cmd)
			if handedOff {
				return nil
			}
			return runTUI()
		},
	}

	buildVersion = version
	root.Flags().Bool("admin", false, "Open the interface as administrator (one UAC prompt)")
	root.Flags().Bool("no-admin", false, "Open the interface as the current user even if alwaysAdmin is on")
	root.Flags().Bool("elevated", false, "Internal: set on the elevated copy so it does not restart again")
	_ = root.Flags().MarkHidden("elevated")
	root.PersistentFlags().Bool("read-only", false, "Refuse every action that would change the system")
	root.SetVersionTemplate("Wintonicz {{.Version}}\n")
	root.AddCommand(
		newProcessCommand(services),
		newPortCommand(services),
		newSystemCommand(services),
		newServiceCommand(services),
		newUSBCommand(services),
		newStorageCommand(services),
		newResourceCommand(services),
		newNetworkCommand(services),
		newPanelCommand(services),
		newSecurityCommand(services),
		newStartupCommand(services),
		newProgramCommand(services),
		newWhyCommand(services),
		newToolsCommand(services),
		newMCPCommand(),
		newConfigCommand(),
		newPinCommand(),
		newUnpinCommand(),
	)
	registerActions(root, services)
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

// handedOff is set when the interface was restarted as administrator in a new window.
var handedOff bool

// restartElevatedIfWanted restarts the interface as administrator when --admin or the
// alwaysAdmin preference asks for it. The elevated copy is marked so it never loops.
func restartElevatedIfWanted(cmd *cobra.Command) {
	elevatedChild, _ := cmd.Flags().GetBool("elevated")
	wantAdmin, _ := cmd.Flags().GetBool("admin")
	if noAdmin, _ := cmd.Flags().GetBool("no-admin"); noAdmin {
		return
	}
	cfg, _ := config.Load()
	if elevatedChild || privilege.IsElevated() || !(wantAdmin || cfg.AlwaysAdmin) {
		return
	}
	err := privilege.StartElevated(append(os.Args[1:], "--elevated"))
	switch {
	case err == nil:
		handedOff = true
	case errors.Is(err, privilege.ErrCancelled):
		fmt.Fprintln(cmd.ErrOrStderr(), "Administrator approval was cancelled; continuing as a standard user.")
	default:
		fmt.Fprintln(cmd.ErrOrStderr(), "Could not restart as administrator:", err, "- continuing as a standard user.")
	}
}
