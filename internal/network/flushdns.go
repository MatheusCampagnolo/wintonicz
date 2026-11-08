package network

import "github.com/MatheusCampagnolo/wintonicz/internal/runner"

// FlushDNSPlan builds the fixed command for tool net.flushdns.
func FlushDNSPlan() (runner.Plan, error) {
	exe, err := runner.SystemExe("ipconfig.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	args := []string{"/flushdns"}
	return runner.Plan{
		Tool: "net.flushdns",
		Exe:  exe,
		Args: args,
		Preview: runner.Preview{
			Title:    "Flush DNS cache",
			Summary:  "Clears the Windows DNS resolver cache. Names are looked up again the next time they are used.",
			Command:  runner.CommandLine(exe, args),
			Admin:    true,
			Duration: "instant",
			Undo:     "Nothing to undo; the cache refills on its own.",
		},
	}, nil
}
