package main

import (
	"fmt"
	"os"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/cli"
	"github.com/MatheusCampagnolo/wintonicz/internal/tui"
)

var version = "dev"

func main() {
	services := app.NewWindowsServices()
	command := cli.NewRootCommand(version, func() error { return tui.Run(services) }, services)
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
