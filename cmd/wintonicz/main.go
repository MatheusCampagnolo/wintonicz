package main

import (
	"os"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/cli"
	"github.com/MatheusCampagnolo/wintonicz/internal/tui"
)

var version = "dev"

func main() {
	services := app.NewWindowsServices()
	command := cli.NewRootCommand(version, func() error { return tui.Run(services, version) }, services)
	os.Exit(cli.Execute(command, os.Args[1:], os.Stdout, os.Stderr))
}
