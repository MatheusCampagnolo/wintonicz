package main

import (
	"fmt"
	"os"

	"github.com/MatheusCampagnolo/wintonicz/internal/cli"
	"github.com/MatheusCampagnolo/wintonicz/internal/tui"
)

var version = "dev"

func main() {
	command := cli.NewRootCommand(version, tui.Run)
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
