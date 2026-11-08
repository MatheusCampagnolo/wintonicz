// Package why answers "why is this running?" from facts Windows exposes: the
// process ancestry, services hosted by the process, its listening ports, and any
// startup entry that points at its executable. Anything not observed is reported
// as unknown rather than guessed, and every conclusion says whether it is
// confirmed or inferred.
package why

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/services"
	"github.com/MatheusCampagnolo/wintonicz/internal/startup"
)

// Process is one link in the ancestry chain.
type Process struct {
	PID         uint32    `json:"pid"`
	ParentPID   uint32    `json:"parentPid"`
	Name        string    `json:"name"`
	Path        string    `json:"path,omitempty"`
	CommandLine string    `json:"commandLine,omitempty"`
	Created     time.Time `json:"created,omitempty"`
}

// ChainSource returns the process and its ancestors (the process first). The note
// explains why the chain ended early, for example that the parent has exited.
type ChainSource interface {
	ProcessChain(pid uint32) (chain []Process, note string, err error)
}

// Reason is one statement about why the process is running.
type Reason struct {
	Text       string `json:"text"`
	Confidence string `json:"confidence"` // confirmed or inferred
}

// Report is the full investigation.
type Report struct {
	Target    string           `json:"target"`
	Chain     []Process        `json:"chain"`
	ChainNote string           `json:"chainNote,omitempty"`
	Services  []services.Entry `json:"services,omitempty"`
	Listeners []ports.Listener `json:"listeners,omitempty"`
	Startup   []startup.Entry  `json:"startup,omitempty"`
	Reasons   []Reason         `json:"reasons"`
	Limits    []string         `json:"limits,omitempty"`
}

// Deps are the data sources an investigation reads.
type Deps struct {
	Processes processes.Source
	Ports     ports.Source
	Services  services.Source
	Startup   startup.Source
	Chain     ChainSource
}

// ResolvePID turns "1234", "node.exe", or ":8080" into a PID. A name that matches
// several processes returns an error listing their PIDs.
func ResolvePID(deps Deps, target string) (uint32, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return 0, fmt.Errorf("enter a PID, a process name, or :port")
	}
	if strings.HasPrefix(target, ":") {
		port, err := strconv.Atoi(strings.TrimPrefix(target, ":"))
		if err != nil || port < 1 || port > 65535 {
			return 0, fmt.Errorf("port must be a number from 1 to 65535")
		}
		listeners, err := deps.Ports.ListListeners()
		if err != nil {
			return 0, fmt.Errorf("read listening ports: %w", err)
		}
		for _, l := range listeners {
			if int(l.Port) == port {
				return l.PID, nil
			}
		}
		return 0, fmt.Errorf("nothing is listening on TCP port %d", port)
	}
	if n, err := strconv.ParseUint(target, 10, 32); err == nil {
		return uint32(n), nil
	}
	all, err := deps.Processes.ListProcesses()
	if err != nil {
		return 0, err
	}
	var matches []processes.Entry
	for _, p := range all {
		if strings.EqualFold(p.Name, target) || strings.EqualFold(strings.TrimSuffix(p.Name, ".exe"), target) {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("no running process is named %q", target)
	case 1:
		return matches[0].PID, nil
	}
	pids := make([]string, 0, len(matches))
	for _, m := range matches {
		pids = append(pids, strconv.FormatUint(uint64(m.PID), 10))
	}
	return 0, fmt.Errorf("%d processes are named %q (PIDs %s); pick one PID", len(matches), target, strings.Join(pids, ", "))
}

// Investigate gathers the facts for one PID and states what they support.
func Investigate(deps Deps, target string, pid uint32) (Report, error) {
	report := Report{Target: target}
	chain, note, err := deps.Chain.ProcessChain(pid)
	if err != nil {
		return report, fmt.Errorf("read process details: %w", err)
	}
	if len(chain) == 0 {
		return report, fmt.Errorf("no running process has PID %d", pid)
	}
	report.Chain, report.ChainNote = chain, note
	self := chain[0]

	if all, err := deps.Services.ListServices(); err == nil {
		for _, s := range all {
			if s.PID == pid {
				report.Services = append(report.Services, s)
			}
		}
	} else {
		report.Limits = append(report.Limits, "Services could not be read: "+err.Error())
	}
	if listeners, err := deps.Ports.ListListeners(); err == nil {
		for _, l := range listeners {
			if l.PID == pid {
				report.Listeners = append(report.Listeners, l)
			}
		}
	} else {
		report.Limits = append(report.Limits, "Listening ports could not be read: "+err.Error())
	}
	if items, err := deps.Startup.ListStartup(); err == nil && self.Path != "" {
		for _, e := range items {
			if strings.Contains(strings.ToLower(e.Command), strings.ToLower(self.Path)) {
				report.Startup = append(report.Startup, e)
			}
		}
	}
	if self.Path == "" {
		report.Limits = append(report.Limits, "The executable path is hidden from this account (try administrator).")
	}
	if self.CommandLine == "" {
		report.Limits = append(report.Limits, "The command line is hidden from this account.")
	}
	report.Reasons = reasons(report)
	return report, nil
}

var shells = map[string]bool{"cmd.exe": true, "powershell.exe": true, "pwsh.exe": true, "windowsterminal.exe": true, "bash.exe": true, "wt.exe": true}

func reasons(r Report) []Reason {
	var out []Reason
	self := r.Chain[0]
	for _, s := range r.Services {
		out = append(out, Reason{fmt.Sprintf("It is hosting the Windows service %q (%s), which Windows starts as a service.", s.DisplayName, s.Name), "confirmed"})
	}
	for _, e := range r.Startup {
		out = append(out, Reason{fmt.Sprintf("Windows is configured to start it automatically (%s: %s).", e.Location, e.Name), "confirmed"})
	}
	if strings.EqualFold(self.Name, "explorer.exe") && len(r.Chain) == 1 {
		out = append(out, Reason{"explorer.exe is your desktop shell: Windows starts it when you sign in.", "inferred"})
	}
	if len(r.Chain) > 1 {
		parent := r.Chain[1]
		switch pname := strings.ToLower(parent.Name); {
		case pname == "services.exe":
			if len(r.Services) == 0 {
				out = append(out, Reason{"Its parent is the Service Control Manager, so it is very likely a Windows service.", "inferred"})
			}
		case pname == "explorer.exe":
			out = append(out, Reason{"It was started from the desktop, Start menu, or a shortcut (parent: explorer.exe).", "inferred"})
		case shells[pname]:
			out = append(out, Reason{fmt.Sprintf("It was started from a command line in %s (PID %d).", parent.Name, parent.PID), "inferred"})
		default:
			out = append(out, Reason{fmt.Sprintf("It was started by %s (PID %d).", parent.Name, parent.PID), "confirmed"})
		}
	} else if self.ParentPID != 0 {
		out = append(out, Reason{fmt.Sprintf("Its parent (PID %d) is no longer running, so what launched it is unknown.", self.ParentPID), "confirmed"})
	}
	if len(r.Listeners) > 0 {
		out = append(out, Reason{fmt.Sprintf("It is listening on %d TCP port(s), so something may be connecting to it.", len(r.Listeners)), "confirmed"})
	}
	if len(out) == 0 {
		out = append(out, Reason{"Windows does not expose enough to say why it started.", "inferred"})
	}
	return out
}

// Lines renders the report as plain text lines (used by the CLI and the TUI).
func Lines(r Report) []string {
	var out []string
	self := r.Chain[0]
	out = append(out, fmt.Sprintf("%s (PID %d)", self.Name, self.PID))
	if self.Path != "" {
		out = append(out, "  Path     "+self.Path)
	}
	if self.CommandLine != "" {
		out = append(out, "  Command  "+self.CommandLine)
	}
	if !self.Created.IsZero() {
		out = append(out, "  Started  "+self.Created.Local().Format("2006-01-02 15:04:05"))
	}
	out = append(out, "", "Why it is running")
	for _, reason := range r.Reasons {
		out = append(out, fmt.Sprintf("  [%s] %s", reason.Confidence, reason.Text))
	}
	if len(r.Chain) > 1 || r.ChainNote != "" {
		out = append(out, "", "Started by (newest first)")
		for i, p := range r.Chain {
			out = append(out, fmt.Sprintf("  %s%s (PID %d)", strings.Repeat("  ", i), p.Name, p.PID))
		}
		if r.ChainNote != "" {
			out = append(out, "  "+r.ChainNote)
		}
	}
	if len(r.Listeners) > 0 {
		out = append(out, "", "Listening on")
		for _, l := range r.Listeners {
			out = append(out, fmt.Sprintf("  %s:%d", l.Address, l.Port))
		}
	}
	if len(r.Limits) > 0 {
		out = append(out, "", "Limits")
		for _, l := range r.Limits {
			out = append(out, "  - "+l)
		}
	}
	return out
}
