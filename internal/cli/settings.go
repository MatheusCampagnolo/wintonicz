package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/MatheusCampagnolo/wintonicz/internal/config"
	"github.com/MatheusCampagnolo/wintonicz/internal/pin"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Read and change Wintonicz preferences", Args: cobra.NoArgs}
	var asJSON bool

	get := &cobra.Command{
		Use:   "get [key]",
		Short: "Show one setting, or all of them",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			values := map[string]string{}
			keys := config.Keys()
			if len(args) == 1 {
				keys = []string{args[0]}
			}
			for _, key := range keys {
				value, err := config.Get(cfg, key)
				if err != nil {
					return ExitError{Code: exitUsage, Err: err}
				}
				values[key] = value
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "config get", values)
			}
			for _, key := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", key, values[key])
			}
			return nil
		},
	}
	get.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")

	set := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Change a setting",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := config.Set(&cfg, args[0], args[1]); err != nil {
				return ExitError{Code: exitUsage, Err: err}
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s saved to %s\n", args[0], config.Path())
			return nil
		},
	}

	path := &cobra.Command{
		Use:   "path",
		Short: "Print the preference file location",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, _ []string) { fmt.Fprintln(cmd.OutOrStdout(), config.Path()) },
	}

	reset := &cobra.Command{
		Use:   "reset",
		Short: "Delete the preference file and return to defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if p := config.Path(); p != "" {
				if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Preferences reset to defaults.")
			return nil
		},
	}
	command.AddCommand(get, set, path, reset)
	return command
}

func newPinCommand() *cobra.Command {
	var yes, dryRun bool
	command := &cobra.Command{
		Use:   "pin",
		Short: "Copy Wintonicz to a per-user folder and add it to your user PATH (opt-in)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			self, _ := os.Executable()
			fmt.Fprintf(out, "Pin Wintonicz\n  Copies:  %s\n  To:      %s\n  Adds:    %s to your USER PATH (not the machine PATH)\n  Undo:    wintonicz unpin\n  Admin:   not needed\n", self, pin.Target(), pin.Dir())
			if dryRun {
				return nil
			}
			if !yes {
				fmt.Fprintln(out, "Nothing was changed. Add --yes to do it.")
				return ExitError{Code: exitNotRun, Err: errors.New("not run: add --yes to confirm")}
			}
			if cfg, _ := config.Load(); cfg.ReadOnly {
				return ExitError{Code: exitNotRun, Err: errors.New("read-only mode is on; turn it off with: wintonicz config set readOnly false")}
			}
			if err := pin.Pin(); err != nil {
				return err
			}
			fmt.Fprintln(out, "Pinned. Open a new terminal and run: wintonicz")
			return nil
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "Do it; without this only the preview is shown")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would change and stop")
	return command
}

func newUnpinCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "unpin",
		Short: "Remove the pinned copy and its user PATH entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Unpin Wintonicz\n  Removes: %s from your USER PATH\n  Deletes: %s\n", pin.Dir(), pin.Target())
			if !yes {
				fmt.Fprintln(out, "Nothing was changed. Add --yes to do it.")
				return ExitError{Code: exitNotRun, Err: errors.New("not run: add --yes to confirm")}
			}
			note, err := pin.Unpin()
			if err != nil {
				return err
			}
			fmt.Fprintln(out, "Unpinned.")
			if note != "" {
				fmt.Fprintln(out, note)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "Do it; without this only the preview is shown")
	return command
}
