package catalog

func action(id string, area Area, title, summary string, risk Risk, admin Admin, dur Duration, reversible, restart bool, command []string, keywords ...string) Tool {
	return Tool{ID: id, Area: area, Title: title, Summary: summary, Keywords: keywords, Kind: KindAction, Risk: risk, Admin: admin,
		Duration: dur, Restart: restart, Reversible: reversible, Command: command}
}

// Guided repairs, cleanup, and reports. Every one goes through the runner.
func init() {
	tools = append(tools,
		action("repair.sfc", AreaRepair, "System File Checker", "Scans protected Windows files and repairs corrupted ones. Takes 10–30 minutes.",
			RiskChange, AdminRequired, DurationMinutes, false, false, []string{"repair", "sfc"}, "windows corrupted", "system files", "sfc"),
		action("repair.dism", AreaRepair, "DISM image health", "Checks (and can repair) the Windows component store. Choose check, scan, restore, or all.",
			RiskChange, AdminRequired, DurationMinutes, false, false, []string{"repair", "dism"}, "windows update broken", "corrupted", "dism"),
		action("repair.gpupdate", AreaRepair, "Group Policy refresh", "Re-applies local and domain Group Policy.",
			RiskChange, AdminOptional, DurationSeconds, true, false, []string{"repair", "gpupdate"}, "policy", "gpo"),
		action("repair.winget.list", AreaRepair, "App updates (list)", "Lists installed apps that have a newer version, using winget. Installs nothing.",
			RiskRead, AdminNone, DurationSeconds, true, false, []string{"repair", "winget", "list"}, "outdated apps", "updates"),
		action("repair.winget", AreaRepair, "Update one app", "Updates a single app by its exact winget ID. No bulk updates.",
			RiskChange, AdminOptional, DurationMinutes, false, false, []string{"repair", "winget", "upgrade"}, "update app", "winget"),
		action("clean.temp", AreaClean, "Clean Temp folder", "Deletes old files from your Temp folder after showing exactly how many and how big. Files in use are skipped.",
			RiskDestructive, AdminNone, DurationSeconds, false, false, []string{"clean", "temp"}, "disk full", "free space", "temporary files"),
		action("clean.systemtemp", AreaClean, "Clean Windows Temp", "Same as Clean Temp folder, for C:\\Windows\\Temp.",
			RiskDestructive, AdminRequired, DurationSeconds, false, false, []string{"clean", "systemtemp"}, "disk full", "free space"),
		action("clean.recyclebin", AreaClean, "Empty Recycle Bin", "Permanently deletes everything in the Recycle Bin, after showing item count and size.",
			RiskDestructive, AdminNone, DurationInstant, false, false, []string{"clean", "recyclebin"}, "disk full", "recycle bin"),
		action("clean.explorer", AreaClean, "Restart Explorer", "Restarts the taskbar and desktop shell when they freeze. Windows brings them back by itself.",
			RiskChange, AdminNone, DurationInstant, true, false, []string{"clean", "explorer"}, "taskbar frozen", "desktop frozen", "explorer"),
		action("tools.process.end", AreaClean, "End a process", "Stops one process by PID after showing its name and path. System processes are refused.",
			RiskDestructive, AdminOptional, DurationInstant, false, false, []string{"process", "end"}, "app frozen", "not responding", "kill"),
		action("report.eventlogs", AreaReports, "Back up event logs", "Exports Application, System, and Setup logs (and Security as administrator) to .evtx files.",
			RiskChange, AdminOptional, DurationSeconds, true, false, []string{"report", "eventlogs"}, "send logs", "event viewer"),
		action("report.download", AreaReports, "Download and verify", "Downloads one HTTPS file to Downloads and shows its SHA-256. Never runs it.",
			RiskChange, AdminNone, DurationSeconds, true, false, []string{"report", "download"}, "download", "checksum", "hash"),
	)
}

// Reports.
func init() {
	support := action("report.support", AreaReports, "Support report", "Writes one file with system, disk, network, security, startup, programs, and ports. Redacts user, host, IP, and MAC by default. Stays on this PC.",
		RiskChange, AdminOptional, DurationSeconds, true, false, []string{"report", "support"}, "send to support", "diagnostics report", "export")
	gp := action("report.gpresult", AreaReports, "Group Policy report", "Saves the Group Policy result as an HTML file, redacted. Computer policy needs administrator.",
		RiskChange, AdminOptional, DurationSeconds, true, false, []string{"report", "gpresult"}, "gpo", "group policy", "gpresult")
	tools = append(tools, support, gp)
}

// Reversible changes. Each writes a prior-state record first and can be
// reversed with `undo` (a restore point is rolled back with System Restore).
func init() {
	list := action("power.list", AreaWindows, "Power plans", "Lists the power plans on this PC and marks the active one.",
		RiskRead, AdminNone, DurationInstant, true, false, []string{"power", "list"}, "battery", "sleep", "performance")
	set := action("power.set", AreaWindows, "Switch power plan", "Switches between Balanced, High performance, and Power saver. Records the previous plan so it can be undone.",
		RiskChange, AdminOptional, DurationInstant, true, false, []string{"power", "set"}, "slow laptop", "battery drains", "high performance")
	pref := action("win.explorer.pref", AreaWindows, "Explorer preferences", "Shows or hides file name extensions and hidden files for your account. Records the previous value.",
		RiskChange, AdminNone, DurationInstant, true, false, []string{"explorer", "pref"}, "file extensions", "hidden files", "show extensions")
	point := action("repair.restorepoint", AreaRepair, "Create restore point", "Asks System Restore for a snapshot before you change something risky. Windows allows one per 24 hours.",
		RiskChange, AdminRequired, DurationSeconds, false, false, []string{"repair", "restorepoint"}, "system restore", "backup", "before changes")
	wu := action("repair.wureset", AreaRepair, "Reset Windows Update", "Renames SoftwareDistribution and catroot2 so Windows rebuilds them. Nothing is deleted; undo puts them back.",
		RiskChange, AdminRequired, DurationSeconds, true, false, []string{"repair", "wureset"}, "windows update stuck", "update error", "update failed")
	reset := action("net.reset", AreaNetwork, "Reset network stack", "Saves your network configuration, then resets Winsock and TCP/IP. Needs a restart.",
		RiskChange, AdminRequired, DurationSeconds, true, true, []string{"network", "reset"}, "no internet", "winsock", "network broken")
	undoList := action("undo.list", AreaRepair, "Change history", "Lists the changes Wintonicz recorded and whether each can still be undone.",
		RiskRead, AdminNone, DurationInstant, true, false, []string{"undo", "list"}, "history", "what changed")
	undo := action("undo", AreaRepair, "Undo a change", "Reverses a recorded change (power plan, Explorer preference, update reset, network reset). Default: the newest.",
		RiskChange, AdminOptional, DurationSeconds, true, false, []string{"undo"}, "revert", "rollback", "undo")
	tools = append(tools, list, set, pref, point, wu, reset, undoList, undo)
}
