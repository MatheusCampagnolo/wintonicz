package main

import (
	"fmt"
	"io"
	"os"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stdout, "Wintonicz is in early development.")
		fmt.Fprintln(stdout, "The interactive interface and diagnostic commands are planned but not available yet.")
		fmt.Fprintln(stdout, "Run 'wintonicz --help' for current commands.")
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage(stdout)
		return 0
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "Wintonicz %s\n", version)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Wintonicz — Windows support toolbox")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  wintonicz              Start the interactive interface (planned)")
	fmt.Fprintln(w, "  wintonicz help         Show this help")
	fmt.Fprintln(w, "  wintonicz version      Show the application version")
}
