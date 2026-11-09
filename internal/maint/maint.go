// Package maint holds the maintenance actions: repairs, cleanup,
// log backup, download, and ending a process. Each action is a runner.Spec that
// validates its inputs and returns a Plan with an honest Preview.
package maint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/privilege"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
)

var olderThanParam = runner.ParamSpec{Name: "older-than-hours", Label: "Only files older than (hours)", Help: "0 removes everything that is not in use.", Default: "24"}

// Specs returns the maintenance actions keyed by catalog tool ID.
func Specs(listProcesses func() ([]processes.Entry, error)) map[string]runner.Spec {
	return map[string]runner.Spec{
		"repair.sfc": {Build: func(map[string]string) (runner.Plan, error) { return sfcPlan() }},
		"repair.dism": {Params: []runner.ParamSpec{{Name: "stage", Label: "Stage", Help: "check (fast), scan (thorough), restore (repair), or all three in order",
			Default: "scan", Choices: []string{"check", "scan", "restore", "all"}}}, Build: dismPlan},
		"repair.gpupdate":    {Build: func(map[string]string) (runner.Plan, error) { return gpupdatePlan() }},
		"repair.winget.list": {Build: func(map[string]string) (runner.Plan, error) { return wingetListPlan() }},
		"repair.winget": {Params: []runner.ParamSpec{{Name: "id", Label: "Package ID", Required: true,
			Help: "Exact winget package ID, for example Git.Git. Run the update list first."}}, Build: wingetUpgradePlan},
		"clean.temp":       {Params: []runner.ParamSpec{olderThanParam}, Build: func(p map[string]string) (runner.Plan, error) { return tempPlan(p, false) }},
		"clean.systemtemp": {Params: []runner.ParamSpec{olderThanParam}, Build: func(p map[string]string) (runner.Plan, error) { return tempPlan(p, true) }},
		"clean.recyclebin": {Build: func(map[string]string) (runner.Plan, error) { return recycleBinPlan() }},
		"clean.explorer":   {Build: func(map[string]string) (runner.Plan, error) { return explorerPlan() }},
		"tools.process.end": {Params: []runner.ParamSpec{{Name: "pid", Label: "Process ID", Required: true, Help: "PID from the Processes list. System processes are refused."}},
			Build: func(p map[string]string) (runner.Plan, error) { return endProcessPlan(p, listProcesses) }},
		"report.eventlogs": {Params: []runner.ParamSpec{{Name: "folder", Label: "Folder", Help: "Where to write the .evtx files. Default: a new timestamped folder in Documents."}}, Build: eventLogPlan},
		"report.download": {Params: []runner.ParamSpec{
			{Name: "url", Label: "HTTPS address", Required: true, Help: "Only https:// addresses. The file goes to your Downloads folder and is never run."},
			{Name: "sha256", Label: "Expected SHA-256 (optional)", Help: "If given, the file is deleted when its hash does not match."}}, Build: downloadPlan},
	}
}

func sfcPlan() (runner.Plan, error) {
	exe, err := runner.SystemExe("sfc.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	args := []string{"/scannow"}
	return runner.Plan{Tool: "repair.sfc", Exe: exe, Args: args, Preview: runner.Preview{
		Title:    "System File Checker",
		Summary:  "Checks protected Windows files and replaces corrupted ones from the local cache.",
		Command:  runner.CommandLine(exe, args),
		Admin:    true,
		Duration: "10–30 minutes",
		Undo:     `Not reversible, but it only restores Microsoft files. Details are in C:\Windows\Logs\CBS\CBS.log.`,
	}}, nil
}

func dismPlan(p map[string]string) (runner.Plan, error) {
	exe, err := runner.SystemExe("dism.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	flags := map[string]string{"check": "/CheckHealth", "scan": "/ScanHealth", "restore": "/RestoreHealth"}
	stage := p["stage"]
	if stage == "" {
		stage = "scan"
	}
	var order []string
	switch stage {
	case "check", "scan", "restore":
		order = []string{stage}
	case "all":
		order = []string{"check", "scan", "restore"}
	default:
		return runner.Plan{}, errors.New("stage must be check, scan, restore, or all")
	}
	var steps []runner.Plan
	var cmds []string
	for _, s := range order {
		args := []string{"/Online", "/Cleanup-Image", flags[s]}
		command := runner.CommandLine(exe, args)
		steps = append(steps, runner.Plan{Tool: "repair.dism", Exe: exe, Args: args, Preview: runner.Preview{Command: command}})
		cmds = append(cmds, command)
	}
	plan := runner.Plan{Tool: "repair.dism", Steps: steps, Preview: runner.Preview{
		Title:    "DISM image health (" + stage + ")",
		Summary:  "Checks the Windows component store for corruption; the restore stage repairs it using Windows Update as the source.",
		Command:  strings.Join(cmds, "  →  "),
		Admin:    true,
		Duration: map[string]string{"check": "under a minute", "scan": "5–15 minutes", "restore": "10–30 minutes", "all": "15–45 minutes"}[stage],
		Undo:     `Not reversible. Details are in C:\Windows\Logs\DISM\dism.log.`,
	}}
	if len(steps) == 1 {
		plan.Exe, plan.Args, plan.Steps = steps[0].Exe, steps[0].Args, nil
	}
	if stage == "restore" || stage == "all" {
		plan.Preview.Summary += " Needs an internet connection."
	}
	return plan, nil
}

func gpupdatePlan() (runner.Plan, error) {
	exe, err := runner.SystemExe("gpupdate.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	args := []string{"/force"}
	return runner.Plan{Tool: "repair.gpupdate", Exe: exe, Args: args, Preview: runner.Preview{
		Title:    "Group Policy refresh",
		Summary:  "Re-applies local and domain Group Policy. On a managed PC this can change settings the organization controls. User policy works without administrator rights; computer policy needs them.",
		Command:  runner.CommandLine(exe, args),
		Duration: "up to 2 minutes",
		Undo:     "Policies are re-applied from their source; there is nothing to undo locally.",
	}}, nil
}

func wingetListPlan() (runner.Plan, error) {
	exe, err := runner.WingetExe()
	if err != nil {
		return runner.Plan{}, err
	}
	args := []string{"upgrade", "--accept-source-agreements"}
	return runner.Plan{Tool: "repair.winget.list", Exe: exe, Args: args, ReadOnlyOK: true, Preview: runner.Preview{
		Title:    "List app updates",
		Summary:  "Shows installed apps that have a newer version available. Contacts the winget package sources. Installs nothing.",
		Command:  runner.CommandLine(exe, args),
		Duration: "10–60 seconds",
		Undo:     "Nothing changes.",
	}}, nil
}

var wingetID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+\-]{0,127}$`)

func wingetUpgradePlan(p map[string]string) (runner.Plan, error) {
	id := strings.TrimSpace(p["id"])
	if !wingetID.MatchString(id) {
		return runner.Plan{}, errors.New("package ID must use letters, digits, and . _ + - only (for example Git.Git)")
	}
	exe, err := runner.WingetExe()
	if err != nil {
		return runner.Plan{}, err
	}
	args := []string{"upgrade", "--id", id, "--exact", "--silent", "--accept-source-agreements", "--accept-package-agreements"}
	return runner.Plan{Tool: "repair.winget", Exe: exe, Args: args, Preview: runner.Preview{
		Title:    "Update " + id,
		Summary:  "Updates this one app to its newest version through winget. The app may close or restart.",
		Command:  runner.CommandLine(exe, args),
		Duration: "1–10 minutes",
		Undo:     "Not reversible through Wintonicz; reinstall the older version from the publisher if needed.",
		Items:    []string{id},
	}}, nil
}

func explorerPlan() (runner.Plan, error) {
	exe, err := runner.SystemExe("taskkill.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	args := []string{"/F", "/IM", "explorer.exe"}
	return runner.Plan{Tool: "clean.explorer", Exe: exe, Args: args, Preview: runner.Preview{
		Title:    "Restart Windows Explorer",
		Summary:  "Closes the taskbar, desktop, and File Explorer windows. Windows restarts them by itself within a few seconds. Other apps and their unsaved work are not affected.",
		Command:  runner.CommandLine(exe, args),
		Duration: "a few seconds",
		Undo:     "Windows restarts Explorer automatically. Reopen your File Explorer windows by hand.",
	}}, nil
}

func eventLogPlan(p map[string]string) (runner.Plan, error) {
	exe, err := runner.SystemExe("wevtutil.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	folder := strings.TrimSpace(p["folder"])
	if folder == "" {
		home, _ := os.UserHomeDir()
		if home == "" {
			return runner.Plan{}, errors.New("cannot find your user folder; pass a folder")
		}
		folder = filepath.Join(home, "Documents", "Wintonicz", "event-logs", time.Now().Format("2006-01-02_150405"))
	}
	folder, err = CleanLocalDir(folder)
	if err != nil {
		return runner.Plan{}, err
	}
	logs := []string{"Application", "System", "Setup"}
	note := "The Security log is skipped: it needs administrator rights."
	if privilege.IsElevated() {
		logs = append(logs, "Security")
		note = "The Security log is included because you are administrator."
	}
	prepare := runner.Plan{Tool: "report.eventlogs", Preview: runner.Preview{Command: "create folder " + folder},
		Do: func(_ context.Context, _ func(string)) error { return os.MkdirAll(folder, 0o700) }}
	steps := []runner.Plan{prepare}
	var items []string
	for _, name := range logs {
		dest := filepath.Join(folder, name+".evtx")
		args := []string{"epl", name, dest, "/ow:false"}
		steps = append(steps, runner.Plan{Tool: "report.eventlogs", Exe: exe, Args: args, Preview: runner.Preview{Command: runner.CommandLine(exe, args)}})
		items = append(items, dest)
	}
	return runner.Plan{Tool: "report.eventlogs", Steps: steps, Preview: runner.Preview{
		Title:    "Back up event logs",
		Summary:  "Exports Windows event logs to .evtx files you can open in Event Viewer or send to support. Logs can contain user names and machine details, so treat the files as private. " + note,
		Command:  "wevtutil epl <log> <file> for " + strings.Join(logs, ", "),
		Duration: "under a minute",
		Undo:     "Delete the folder to remove the backup. The live logs are not touched.",
		Items:    items,
	}}, nil
}

// CleanLocalDir validates a user-supplied folder: absolute, local drive, not a drive root, no UNC.
func CleanLocalDir(path string) (string, error) {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		return "", errors.New("network (UNC) paths are not allowed")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New(`folder must be an absolute path such as C:\Users\you\Documents\logs`)
	}
	clean := filepath.Clean(path)
	if len(clean) <= 3 {
		return "", errors.New("refusing to use a drive root")
	}
	return clean, nil
}

func parseHours(p map[string]string) (time.Duration, error) {
	value := strings.TrimSpace(p["older-than-hours"])
	if value == "" {
		value = "24"
	}
	hours, err := strconv.Atoi(value)
	if err != nil || hours < 0 || hours > 24*365 {
		return 0, errors.New("older-than-hours must be a whole number from 0 to 8760")
	}
	return time.Duration(hours) * time.Hour, nil
}

var _ = fmt.Sprintf
