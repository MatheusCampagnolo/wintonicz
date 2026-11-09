package change

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
)

// Specs returns the reversible-change actions keyed by catalog tool ID.
func Specs() map[string]runner.Spec {
	return map[string]runner.Spec{
		"power.list": {Build: func(map[string]string) (runner.Plan, error) { return powerListPlan() }},
		"power.set": {Params: []runner.ParamSpec{{Name: "plan", Label: "Power plan", Required: true,
			Help: "balanced, high (High performance), or saver (Power saver). High performance is missing on some PCs.", Choices: []string{"balanced", "high", "saver"}}},
			Build: powerSetPlan},
		"win.explorer.pref": {Params: []runner.ParamSpec{
			{Name: "setting", Label: "Setting", Required: true, Help: "file-extensions or hidden-files", Choices: []string{"file-extensions", "hidden-files"}},
			{Name: "value", Label: "Value", Required: true, Help: "show or hide", Choices: []string{"show", "hide"}}},
			Build: explorerPrefPlan},
		"repair.restorepoint": {Params: []runner.ParamSpec{{Name: "description", Label: "Description", Default: "Wintonicz",
			Help: "Letters, digits, spaces, and . _ - only (up to 60 characters)."}}, Build: restorePointPlan},
		"repair.wureset": {Build: func(map[string]string) (runner.Plan, error) { return wuResetPlan() }},
		"net.reset":      {Build: func(map[string]string) (runner.Plan, error) { return netResetPlan() }},
		"undo.list":      {Build: func(map[string]string) (runner.Plan, error) { return undoListPlan() }},
		"undo": {Params: []runner.ParamSpec{{Name: "id", Label: "Entry ID", Help: "From `undo list`. Default: the newest change that can still be undone."}},
			Build: undoPlan},
	}
}

// query runs one fixed System32 program and returns its output. It is read-only and
// used to record prior state; tests replace it.
var query = func(name string, args ...string) (string, error) {
	exe, err := runner.SystemExe(name)
	if err != nil {
		return "", err
	}
	out, err := exec.Command(exe, args...).Output()
	return string(out), err
}

func exeStep(tool, exe string, args ...string) runner.Plan {
	return runner.Plan{Tool: tool, Exe: exe, Args: args, Preview: runner.Preview{Command: runner.CommandLine(exe, args)}}
}

func scriptStep(tool, exe, label, script string) runner.Plan {
	return runner.Plan{Tool: tool, Exe: exe, Args: []string{"-NoProfile", "-NonInteractive", "-Command", script},
		Preview: runner.Preview{Command: "powershell.exe — " + label}}
}

func recordStep(tool, label string, build func() (Entry, error)) runner.Plan {
	return runner.Plan{Tool: tool, Preview: runner.Preview{Command: "record previous state in the undo journal"},
		Do: func(_ context.Context, emit func(string)) error {
			entry, err := build()
			if err != nil {
				return fmt.Errorf("cannot record the previous state, so nothing was changed: %w", err)
			}
			saved, err := Save(entry)
			if err != nil {
				return fmt.Errorf("cannot write the undo journal, so nothing was changed: %w", err)
			}
			emit(label + " (undo entry " + saved.ID + ")")
			return nil
		}}
}

func powershell() (string, error) {
	return runner.SystemExe(filepath.Join("WindowsPowerShell", "v1.0", "powershell.exe"))
}

// ---- power plans ----

type powerPlan struct{ guid, label string }

var powerPlans = map[string]powerPlan{
	"balanced": {"381b4222-f694-41f0-9685-ff5bb260df2e", "Balanced"},
	"high":     {"8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c", "High performance"},
	"saver":    {"a1841308-3541-4fab-bc81-f71556f20b4a", "Power saver"},
}

var (
	guidRE   = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)
	schemeRE = regexp.MustCompile(`(?i)([0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12})\s*(?:\(([^)]*)\))?`)
)

// activeScheme parses `powercfg /getactivescheme`.
func activeScheme() (guid, name string, err error) {
	out, err := query("powercfg.exe", "/getactivescheme")
	if err != nil {
		return "", "", fmt.Errorf("read the active power plan: %w", err)
	}
	m := schemeRE.FindStringSubmatch(out)
	if m == nil {
		return "", "", errors.New("could not read the active power plan")
	}
	return strings.ToLower(m[1]), m[2], nil
}

func powerListPlan() (runner.Plan, error) {
	exe, err := runner.SystemExe("powercfg.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	args := []string{"/list"}
	return runner.Plan{Tool: "power.list", Exe: exe, Args: args, ReadOnlyOK: true, Preview: runner.Preview{
		Title: "List power plans", Summary: "Shows the power plans on this PC; the active one is marked with *.",
		Command: runner.CommandLine(exe, args), Duration: "instant", Undo: "Nothing changes."}}, nil
}

func powerSetPlan(p map[string]string) (runner.Plan, error) {
	target, ok := powerPlans[p["plan"]]
	if !ok {
		return runner.Plan{}, errors.New("plan must be balanced, high, or saver")
	}
	exe, err := runner.SystemExe("powercfg.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	current, currentName, err := activeScheme()
	if err != nil {
		return runner.Plan{}, err
	}
	if current == target.guid {
		return runner.Plan{}, fmt.Errorf("%s is already the active power plan", target.label)
	}
	record := recordStep("power.set", "Recorded the active plan "+currentName, func() (Entry, error) {
		guid, name, err := activeScheme()
		if err != nil {
			return Entry{}, err
		}
		return Entry{Tool: "power.set", Kind: KindPower, Data: map[string]string{"previous": guid, "previousName": name},
			Summary: "Power plan " + name + " → " + target.label}, nil
	})
	set := exeStep("power.set", exe, "/setactive", target.guid)
	return runner.Plan{Tool: "power.set", Steps: []runner.Plan{record, set}, Preview: runner.Preview{
		Title:    "Switch power plan to " + target.label,
		Summary:  fmt.Sprintf("Changes the active Windows power plan from %q to %q. Affects sleep, CPU behavior, and battery life.", currentName, target.label),
		Command:  runner.CommandLine(exe, set.Args),
		Duration: "instant",
		Undo:     "Run `wintonicz undo` to switch back to " + currentName + ".",
	}}, nil
}

func undoPower(e Entry) ([]runner.Plan, bool, error) {
	prev := e.Data["previous"]
	if !guidRE.MatchString(prev) {
		return nil, false, errors.New("the journal entry has no valid power plan to return to")
	}
	exe, err := runner.SystemExe("powercfg.exe")
	if err != nil {
		return nil, false, err
	}
	return []runner.Plan{exeStep("undo", exe, "/setactive", prev)}, false, nil
}

// ---- Explorer preferences ----

const explorerKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced`

type explorerSetting struct {
	value string
	data  map[string]string // "show"/"hide" → DWORD
	label string
}

var explorerSettings = map[string]explorerSetting{
	"file-extensions": {"HideFileExt", map[string]string{"show": "0", "hide": "1"}, "file name extensions"},
	"hidden-files":    {"Hidden", map[string]string{"show": "1", "hide": "2"}, "hidden files and folders"},
}

var valueRE = regexp.MustCompile(`(?i)REG_DWORD\s+0x([0-9a-f]+)`)

// readExplorerValue returns the current DWORD as a decimal string, or "" when unset.
func readExplorerValue(name string) (string, error) {
	out, err := query("reg.exe", "query", explorerKey, "/v", name)
	if err != nil {
		// reg exits 1 when the value does not exist; that is a valid prior state.
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", nil
		}
		return "", err
	}
	m := valueRE.FindStringSubmatch(out)
	if m == nil {
		return "", nil
	}
	var n int
	if _, err := fmt.Sscanf(m[1], "%x", &n); err != nil {
		return "", err
	}
	return fmt.Sprint(n), nil
}

func explorerPrefPlan(p map[string]string) (runner.Plan, error) {
	setting, ok := explorerSettings[p["setting"]]
	if !ok {
		return runner.Plan{}, errors.New("setting must be file-extensions or hidden-files")
	}
	data, ok := setting.data[p["value"]]
	if !ok {
		return runner.Plan{}, errors.New("value must be show or hide")
	}
	exe, err := runner.SystemExe("reg.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	current, err := readExplorerValue(setting.value)
	if err != nil {
		return runner.Plan{}, fmt.Errorf("read the current setting: %w", err)
	}
	if current == data {
		return runner.Plan{}, fmt.Errorf("%s are already set to %s", setting.label, p["value"])
	}
	record := recordStep("win.explorer.pref", "Recorded the current setting", func() (Entry, error) {
		prev, err := readExplorerValue(setting.value)
		if err != nil {
			return Entry{}, err
		}
		return Entry{Tool: "win.explorer.pref", Kind: KindExplorer, Data: map[string]string{"setting": p["setting"], "previous": prev},
			Summary: fmt.Sprintf("Explorer: %s → %s", setting.label, p["value"])}, nil
	})
	set := exeStep("win.explorer.pref", exe, "add", explorerKey, "/v", setting.value, "/t", "REG_DWORD", "/d", data, "/f")
	return runner.Plan{Tool: "win.explorer.pref", Steps: []runner.Plan{record, set}, Preview: runner.Preview{
		Title:    fmt.Sprintf("%s %s in Explorer", strings.ToUpper(p["value"][:1])+p["value"][1:], setting.label),
		Summary:  "Changes one File Explorer preference for the signed-in account. Open Explorer windows pick it up after F5 or after restarting Explorer.",
		Command:  runner.CommandLine(exe, set.Args),
		Duration: "instant",
		Undo:     "Run `wintonicz undo` to restore the previous value.",
	}}, nil
}

func undoExplorer(e Entry) ([]runner.Plan, bool, error) {
	setting, ok := explorerSettings[e.Data["setting"]]
	if !ok {
		return nil, false, errors.New("the journal entry names an unknown Explorer setting")
	}
	prev := e.Data["previous"]
	exe, err := runner.SystemExe("reg.exe")
	if err != nil {
		return nil, false, err
	}
	if prev == "" {
		return []runner.Plan{exeStep("undo", exe, "delete", explorerKey, "/v", setting.value, "/f")}, false, nil
	}
	if !regexp.MustCompile(`^[0-9]{1,2}$`).MatchString(prev) {
		return nil, false, errors.New("the journal entry has an invalid previous value")
	}
	return []runner.Plan{exeStep("undo", exe, "add", explorerKey, "/v", setting.value, "/t", "REG_DWORD", "/d", prev, "/f")}, false, nil
}

// ---- restore point ----

var descRE = regexp.MustCompile(`^[A-Za-z0-9 ._\-]{1,60}$`)

func restorePointPlan(p map[string]string) (runner.Plan, error) {
	desc := strings.TrimSpace(p["description"])
	if desc == "" {
		desc = "Wintonicz"
	}
	if !descRE.MatchString(desc) {
		return runner.Plan{}, errors.New("description may use letters, digits, spaces, and . _ - only (up to 60 characters)")
	}
	ps, err := powershell()
	if err != nil {
		return runner.Plan{}, err
	}
	script := "$ErrorActionPreference='Stop'; Checkpoint-Computer -Description '" + desc + "' -RestorePointType MODIFY_SETTINGS; 'Restore point created: " + desc + "'"
	create := scriptStep("repair.restorepoint", ps, "Checkpoint-Computer", script)
	note := runner.Plan{Tool: "repair.restorepoint", Preview: runner.Preview{Command: "record the restore point in the undo journal"},
		Do: func(context.Context, func(string)) error {
			_, err := Save(Entry{Tool: "repair.restorepoint", Kind: KindRestorePnt, Data: map[string]string{"description": desc},
				Summary: "Restore point “" + desc + "”"})
			return err
		}}
	return runner.Plan{Tool: "repair.restorepoint", Steps: []runner.Plan{create, note}, Preview: runner.Preview{
		Title:    "Create a restore point",
		Summary:  "Asks System Restore for a snapshot of system files and settings. System Protection must be on for the system drive, and Windows allows one restore point per 24 hours by default (a second request is skipped).",
		Command:  "powershell.exe — Checkpoint-Computer -Description '" + desc + "'",
		Admin:    true,
		Duration: "under a minute",
		Undo:     "Use System Restore (rstrui.exe) to roll back to it. Wintonicz does not roll restore points back itself.",
		Items:    []string{desc},
	}}, nil
}

// ---- Windows Update reset ----

var suffixRE = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}$`)

const wuServices = "'wuauserv','bits','cryptsvc'"

func wuResetPlan() (runner.Plan, error) {
	ps, err := powershell()
	if err != nil {
		return runner.Plan{}, err
	}
	suffix := time.Now().Format("20060102-150405")
	script := "$ErrorActionPreference='Stop'; $sfx='" + suffix + "'; $svc=" + wuServices + "; " +
		"foreach($s in $svc){ Stop-Service -Name $s -Force -ErrorAction SilentlyContinue }; " +
		"try { foreach($t in @(\"$env:SystemRoot\\SoftwareDistribution\",\"$env:SystemRoot\\System32\\catroot2\")) { " +
		"if(Test-Path -LiteralPath $t){ Rename-Item -LiteralPath $t -NewName ((Split-Path $t -Leaf)+'.wtz-'+$sfx); 'Renamed '+$t } else { 'Not found: '+$t } } } " +
		"finally { foreach($s in $svc){ Start-Service -Name $s -ErrorAction SilentlyContinue } }"
	record := recordStep("repair.wureset", "Recorded the reset", func() (Entry, error) {
		return Entry{Tool: "repair.wureset", Kind: KindWUReset, Data: map[string]string{"suffix": suffix},
			Summary: "Windows Update caches renamed (*.wtz-" + suffix + ")"}, nil
	})
	return runner.Plan{Tool: "repair.wureset", Steps: []runner.Plan{record, scriptStep("repair.wureset", ps, "rename update caches", script)}, Preview: runner.Preview{
		Title:    "Reset Windows Update components",
		Summary:  "Stops the update services, renames SoftwareDistribution and catroot2 so Windows rebuilds them, then starts the services again. Nothing is deleted. Update history shown in Settings may restart from empty and the next scan takes longer.",
		Command:  "powershell.exe — rename SoftwareDistribution and catroot2 to *.wtz-" + suffix,
		Admin:    true,
		Duration: "under a minute",
		Undo:     "Run `wintonicz undo` to put the original folders back. The folders Windows rebuilt are renamed aside, not deleted.",
		Items:    []string{`C:\Windows\SoftwareDistribution`, `C:\Windows\System32\catroot2`},
	}}, nil
}

func undoWUReset(e Entry) ([]runner.Plan, bool, error) {
	suffix := e.Data["suffix"]
	if !suffixRE.MatchString(suffix) {
		return nil, false, errors.New("the journal entry has an invalid timestamp")
	}
	ps, err := powershell()
	if err != nil {
		return nil, false, err
	}
	script := "$ErrorActionPreference='Stop'; $sfx='" + suffix + "'; $svc=" + wuServices + "; " +
		"foreach($s in $svc){ Stop-Service -Name $s -Force -ErrorAction SilentlyContinue }; " +
		"try { foreach($t in @(\"$env:SystemRoot\\SoftwareDistribution\",\"$env:SystemRoot\\System32\\catroot2\")) { " +
		"$old=$t+'.wtz-'+$sfx; if(Test-Path -LiteralPath $old){ " +
		"if(Test-Path -LiteralPath $t){ Rename-Item -LiteralPath $t -NewName ((Split-Path $t -Leaf)+'.wtz-replaced-'+$sfx) }; " +
		"Rename-Item -LiteralPath $old -NewName (Split-Path $t -Leaf); 'Restored '+$t } else { 'Nothing to restore for '+$t } } } " +
		"finally { foreach($s in $svc){ Start-Service -Name $s -ErrorAction SilentlyContinue } }"
	return []runner.Plan{scriptStep("undo", ps, "restore update caches", script)}, true, nil
}

// ---- network reset ----

var snapshotRE = regexp.MustCompile(`^netreset-[0-9]{8}-[0-9]{6}\.txt$`)

func netResetPlan() (runner.Plan, error) {
	dir := Dir()
	if dir == "" {
		return runner.Plan{}, errNoDir
	}
	ps, err := powershell()
	if err != nil {
		return runner.Plan{}, err
	}
	netsh, err := runner.SystemExe("netsh.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	stamp := time.Now().Format("20060102-150405")
	snapshot := filepath.Join(dir, "netreset-"+stamp+".txt")
	quoted := strings.ReplaceAll(snapshot, "'", "''")
	prepare := runner.Plan{Tool: "net.reset", Preview: runner.Preview{Command: "create the undo folder"},
		Do: func(context.Context, func(string)) error { return os.MkdirAll(dir, 0o700) }}
	dump := scriptStep("net.reset", ps, "save the current network configuration",
		"$ErrorActionPreference='Stop'; netsh dump | Out-File -LiteralPath '"+quoted+"' -Encoding ASCII; if(-not (Get-Item -LiteralPath '"+quoted+"').Length){ throw 'The configuration dump is empty.' }; 'Saved '+'"+quoted+"'")
	record := recordStep("net.reset", "Saved the network configuration", func() (Entry, error) {
		return Entry{Tool: "net.reset", Kind: KindNetReset, Data: map[string]string{"snapshot": snapshot},
			Summary: "Network reset (configuration saved to " + filepath.Base(snapshot) + ")"}, nil
	})
	winsock := exeStep("net.reset", netsh, "winsock", "reset")
	ip := exeStep("net.reset", netsh, "int", "ip", "reset")
	return runner.Plan{Tool: "net.reset", Steps: []runner.Plan{prepare, dump, record, winsock, ip}, Preview: runner.Preview{
		Title:    "Reset the network stack",
		Summary:  "Saves your current network configuration, then resets Winsock and the TCP/IP stack to defaults. Fixes broken sockets and some \"connected but no internet\" cases. Static IP addresses, custom DNS servers, and VPN adapters may need to be set again. A restart is required to finish.",
		Command:  "netsh winsock reset  →  netsh int ip reset",
		Admin:    true,
		Restart:  true,
		Duration: "under a minute, then restart",
		Undo:     "Run `wintonicz undo` to re-apply the saved IP configuration. The Winsock catalog is reset to Windows defaults and is not restored.",
		Items:    []string{snapshot},
	}}, nil
}

func undoNetReset(e Entry) ([]runner.Plan, bool, error) {
	snapshot := filepath.Clean(e.Data["snapshot"])
	dir := Dir()
	if dir == "" || !strings.EqualFold(filepath.Dir(snapshot), filepath.Clean(dir)) || !snapshotRE.MatchString(filepath.Base(snapshot)) {
		return nil, false, errors.New("the journal entry points outside the undo folder")
	}
	if _, err := os.Stat(snapshot); err != nil {
		return nil, false, errors.New("the saved network configuration is missing")
	}
	netsh, err := runner.SystemExe("netsh.exe")
	if err != nil {
		return nil, false, err
	}
	return []runner.Plan{exeStep("undo", netsh, "-f", snapshot)}, true, nil
}

// ---- undo ----

func undoListPlan() (runner.Plan, error) {
	return runner.Plan{Tool: "undo.list", ReadOnlyOK: true, Preview: runner.Preview{
		Title: "List undoable changes", Summary: "Shows the changes Wintonicz recorded, newest first.",
		Command: "read the undo journal", Duration: "instant", Undo: "Nothing changes."},
		Do: func(_ context.Context, emit func(string)) error {
			entries, err := List()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				emit("No changes recorded yet.")
				return nil
			}
			for _, e := range entries {
				state := "can undo"
				switch {
				case e.Undone:
					state = "undone"
				case e.Kind == KindRestorePnt:
					state = "use System Restore"
				}
				emit(fmt.Sprintf("%s  %-18s  %s  [%s]", e.ID, e.Tool, e.Summary, state))
			}
			return nil
		}}, nil
}

func undoPlan(p map[string]string) (runner.Plan, error) {
	entry, err := Find(strings.TrimSpace(p["id"]))
	if err != nil {
		return runner.Plan{}, err
	}
	if entry.Undone {
		return runner.Plan{}, fmt.Errorf("entry %s was already undone", entry.ID)
	}
	var steps []runner.Plan
	var admin bool
	switch entry.Kind {
	case KindPower:
		steps, admin, err = undoPower(entry)
	case KindExplorer:
		steps, admin, err = undoExplorer(entry)
	case KindWUReset:
		steps, admin, err = undoWUReset(entry)
	case KindNetReset:
		steps, admin, err = undoNetReset(entry)
	case KindRestorePnt:
		err = errors.New("a restore point is rolled back with System Restore (run rstrui.exe), not with undo")
	default:
		err = fmt.Errorf("unknown journal entry kind %q", entry.Kind)
	}
	if err != nil {
		return runner.Plan{}, err
	}
	steps = append(steps, runner.Plan{Tool: "undo", Preview: runner.Preview{Command: "mark the journal entry as undone"},
		Do: func(context.Context, func(string)) error { return MarkUndone(entry.ID) }})
	first := steps[0].Preview.Command
	return runner.Plan{Tool: "undo", Steps: steps, Preview: runner.Preview{
		Title:    "Undo: " + entry.Summary,
		Summary:  fmt.Sprintf("Reverses the change recorded as %s on %s.", entry.ID, entry.Time.Local().Format("2006-01-02 15:04")),
		Command:  first,
		Admin:    admin,
		Restart:  entry.Kind == KindNetReset,
		Duration: "under a minute",
		Undo:     "Make the change again to redo it.",
		Items:    []string{entry.ID},
	}}, nil
}
