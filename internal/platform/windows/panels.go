//go:build windows

package windows

import (
	"fmt"
	"os/exec"

	"github.com/MatheusCampagnolo/wintonicz/internal/panels"
)

var panelCatalog = []panels.Entry{
	{Key: "computer-management", Name: "Computer Management", Description: "Devices, disks, services, and system tools."},
	{Key: "control-panel", Name: "Control Panel", Description: "Classic Windows Control Panel."},
	{Key: "mouse", Name: "Mouse Properties", Description: "Pointer, buttons, and wheel settings."},
	{Key: "network-connections", Name: "Network Connections", Description: "Network adapters and connection properties."},
	{Key: "power", Name: "Power Options", Description: "Power plans and sleep settings."},
	{Key: "printers", Name: "Printers", Description: "Installed printers and print settings."},
	{Key: "programs", Name: "Programs and Features", Description: "Installed desktop programs."},
	{Key: "region", Name: "Region", Description: "Regional format and location settings."},
	{Key: "security-maintenance", Name: "Security and Maintenance", Description: "Security and maintenance status."},
	{Key: "sound", Name: "Sound", Description: "Playback, recording, and sound devices."},
	{Key: "system", Name: "System Properties", Description: "Computer name and advanced system properties."},
	{Key: "date-time", Name: "Date and Time", Description: "Date, time, and time zone settings."},
	{Key: "firewall", Name: "Windows Defender Firewall", Description: "Firewall status and rules."},
	{Key: "system-restore", Name: "System Restore", Description: "Open the Windows restore point wizard."},
	{Key: "services", Name: "Services", Description: "Inspect and manage installed Windows services."},
	{Key: "event-viewer", Name: "Event Viewer", Description: "Review Windows application, system, and security logs."},
	{Key: "task-manager", Name: "Task Manager", Description: "Inspect running applications, processes, and resource usage."},
	{Key: "disk-management", Name: "Disk Management", Description: "View disks, partitions, and volumes."},
	{Key: "disk-cleanup", Name: "Disk Cleanup", Description: "Open the built-in utility for reviewing removable system files."},
}

type panelTarget struct {
	executable string
	arguments  []string
}

var panelTargets = map[string]panelTarget{
	"computer-management":  {"mmc.exe", []string{"compmgmt.msc"}},
	"control-panel":        {"control.exe", nil},
	"mouse":                {"control.exe", []string{"main.cpl"}},
	"network-connections":  {"control.exe", []string{"ncpa.cpl"}},
	"power":                {"control.exe", []string{"powercfg.cpl"}},
	"printers":             {"control.exe", []string{"printers"}},
	"programs":             {"control.exe", []string{"appwiz.cpl"}},
	"region":               {"control.exe", []string{"intl.cpl"}},
	"security-maintenance": {"control.exe", []string{"wscui.cpl"}},
	"sound":                {"control.exe", []string{"mmsys.cpl"}},
	"system":               {"control.exe", []string{"sysdm.cpl"}},
	"date-time":            {"control.exe", []string{"timedate.cpl"}},
	"firewall":             {"control.exe", []string{"firewall.cpl"}},
	"system-restore":       {"rstrui.exe", nil},
	"services":             {"mmc.exe", []string{"services.msc"}},
	"event-viewer":         {"mmc.exe", []string{"eventvwr.msc"}},
	"task-manager":         {"taskmgr.exe", nil},
	"disk-management":      {"mmc.exe", []string{"diskmgmt.msc"}},
	"disk-cleanup":         {"cleanmgr.exe", nil},
}

func (Collector) ListPanels() []panels.Entry {
	return append([]panels.Entry(nil), panelCatalog...)
}

func (Collector) OpenPanel(key string) error {
	target, ok := panelTargets[key]
	if !ok {
		return fmt.Errorf("unknown Windows panel %q", key)
	}
	command := exec.Command(target.executable, target.arguments...)
	if err := command.Start(); err != nil {
		return fmt.Errorf("open %s: %w", key, err)
	}
	return nil
}
