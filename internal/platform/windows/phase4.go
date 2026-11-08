//go:build windows

package windows

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/diskhealth"
	"github.com/MatheusCampagnolo/wintonicz/internal/programs"
	"github.com/MatheusCampagnolo/wintonicz/internal/security"
	"github.com/MatheusCampagnolo/wintonicz/internal/startup"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// runPowerShell runs a fixed, reviewed script (never user input) and returns stdout.
// PowerShell is used only where a cmdlet or WMI class has no practical native API.
func runPowerShell(script string) ([]byte, error) {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, err
	}
	exe := filepath.Join(dir, "WindowsPowerShell", "v1.0", "powershell.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	prelude := "[Console]::OutputEncoding=[Text.Encoding]::UTF8; $ErrorActionPreference='Stop'; "
	cmd := exec.CommandContext(ctx, exe, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", prelude+script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && len(exit.Stderr) > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

// jsonList decodes PowerShell's ConvertTo-Json output, which is an object for one
// item, an array for several, and empty for none.
func jsonList[T any](data []byte) ([]T, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil, nil
	}
	if data[0] == '[' {
		var items []T
		return items, json.Unmarshal(data, &items)
	}
	var item T
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, err
	}
	return []T{item}, nil
}

// ---- disk health ----

func (Collector) ListDiskHealth() ([]diskhealth.Disk, error) {
	out, err := runPowerShell(`Get-PhysicalDisk | ForEach-Object { [pscustomobject]@{ name=$_.FriendlyName; mediaType=[string]$_.MediaType; busType=[string]$_.BusType; health=[string]$_.HealthStatus; operational=(($_.OperationalStatus | ForEach-Object { [string]$_ }) -join ', '); sizeBytes=[uint64]$_.Size } } | ConvertTo-Json -Compress`)
	if err != nil {
		return nil, fmt.Errorf("read physical disk health: %w", err)
	}
	return jsonList[diskhealth.Disk](out)
}

// ---- security status ----

func (Collector) SecurityStatus() (security.Status, error) {
	out, err := runPowerShell(`$av = @(Get-CimInstance -Namespace root/SecurityCenter2 -ClassName AntiVirusProduct -ErrorAction SilentlyContinue | ForEach-Object { [pscustomobject]@{ name=$_.displayName; state=[uint32]$_.productState } });
$fw = @(Get-NetFirewallProfile -ErrorAction SilentlyContinue | ForEach-Object { [pscustomobject]@{ profile=[string]$_.Name; enabled=([string]$_.Enabled -eq 'True') } });
[pscustomobject]@{ antivirus=$av; firewall=$fw } | ConvertTo-Json -Depth 4 -Compress`)
	if err != nil {
		return security.Status{}, fmt.Errorf("read security status: %w", err)
	}
	var raw struct {
		Antivirus json.RawMessage `json:"antivirus"`
		Firewall  json.RawMessage `json:"firewall"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return security.Status{}, err
	}
	type avRow struct {
		Name  string `json:"name"`
		State uint32 `json:"state"`
	}
	avs, err := jsonList[avRow](raw.Antivirus)
	if err != nil {
		return security.Status{}, err
	}
	fws, err := jsonList[security.Firewall](raw.Firewall)
	if err != nil {
		return security.Status{}, err
	}
	status := security.Status{Firewall: fws}
	for _, a := range avs {
		status.Antivirus = append(status.Antivirus, security.NewProduct(a.Name, a.State))
	}
	return status, nil
}

// ---- startup items ----

func (Collector) ListStartup() ([]startup.Entry, error) {
	var entries []startup.Entry
	runKeys := []struct {
		root  registry.Key
		path  string
		view  uint32
		scope string
		label string
	}{
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, 0, "user", `HKCU\...\Run`},
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.WOW64_64KEY, "machine", `HKLM\...\Run`},
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.WOW64_32KEY, "machine", `HKLM\...\Run (32-bit)`},
	}
	for _, k := range runKeys {
		key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE|k.view)
		if err != nil {
			continue
		}
		names, _ := key.ReadValueNames(0)
		for _, name := range names {
			value, _, err := key.GetStringValue(name)
			if err != nil {
				continue
			}
			entries = append(entries, startup.Entry{Name: name, Command: value, Location: k.label, Scope: k.scope})
		}
		key.Close()
	}
	folders := []struct{ env, scope, label string }{
		{"APPDATA", "user", "Startup folder (user)"},
		{"ProgramData", "machine", "Startup folder (all users)"},
	}
	for _, f := range folders {
		base := os.Getenv(f.env)
		if base == "" {
			continue
		}
		dir := filepath.Join(base, `Microsoft\Windows\Start Menu\Programs\Startup`)
		items, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, item := range items {
			if strings.EqualFold(item.Name(), "desktop.ini") {
				continue
			}
			entries = append(entries, startup.Entry{Name: item.Name(), Command: filepath.Join(dir, item.Name()), Location: f.label, Scope: f.scope})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name) })
	return entries, nil
}

// ---- installed programs ----

func (Collector) ListPrograms() ([]programs.Program, error) {
	const uninstall = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
	sources := []struct {
		root  registry.Key
		view  uint32
		scope string
	}{
		{registry.LOCAL_MACHINE, registry.WOW64_64KEY, "machine"},
		{registry.LOCAL_MACHINE, registry.WOW64_32KEY, "machine"},
		{registry.CURRENT_USER, 0, "user"},
	}
	seen := map[string]bool{}
	var result []programs.Program
	for _, s := range sources {
		root, err := registry.OpenKey(s.root, uninstall, registry.ENUMERATE_SUB_KEYS|s.view)
		if err != nil {
			continue
		}
		subkeys, _ := root.ReadSubKeyNames(0)
		root.Close()
		for _, sub := range subkeys {
			key, err := registry.OpenKey(s.root, uninstall+`\`+sub, registry.QUERY_VALUE|s.view)
			if err != nil {
				continue
			}
			name, _, _ := key.GetStringValue("DisplayName")
			system, _, sysErr := key.GetIntegerValue("SystemComponent")
			_, _, parentErr := key.GetStringValue("ParentKeyName")
			if name == "" || (sysErr == nil && system == 1) || parentErr == nil {
				key.Close()
				continue
			}
			p := programs.Program{Name: strings.TrimSpace(name), Scope: s.scope}
			p.Version, _, _ = key.GetStringValue("DisplayVersion")
			p.Publisher, _, _ = key.GetStringValue("Publisher")
			p.InstallDate, _, _ = key.GetStringValue("InstallDate")
			if size, _, err := key.GetIntegerValue("EstimatedSize"); err == nil {
				p.SizeKB = uint32(size)
			}
			key.Close()
			id := strings.ToLower(p.Name + "|" + p.Version)
			if seen[id] {
				continue
			}
			seen[id] = true
			result = append(result, p)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}
