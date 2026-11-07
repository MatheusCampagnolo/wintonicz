package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/cli"
	"github.com/MatheusCampagnolo/wintonicz/internal/platform/windows"
	"github.com/MatheusCampagnolo/wintonicz/internal/tui"
)

var version = "dev"

func main() {
	if len(os.Args) == 1 && hasInteractiveTerminal() {
		relaunched, err := windows.EnsureElevated()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not restart Wintonicz with administrator privileges:", err)
			os.Exit(1)
		}
		if relaunched {
			return
		}
	}

	services := app.NewWindowsServices()
	command := cli.NewRootCommand(version, func() error { return tui.Run(services) }, services)
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		var exitError interface{ ExitCode() int }
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
		os.Exit(1)
	}
}

func hasInteractiveTerminal() bool {
	input, inputErr := os.Stdin.Stat()
	output, outputErr := os.Stdout.Stat()
	return inputErr == nil && outputErr == nil &&
		input.Mode()&os.ModeCharDevice != 0 && output.Mode()&os.ModeCharDevice != 0
}
