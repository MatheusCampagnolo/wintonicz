package catalog

func action(id string, area Area, title, summary string, risk Risk, admin Admin, dur Duration, reversible, restart bool, command []string, keywords ...string) Tool {
	return Tool{ID: id, Area: area, Title: title, Summary: summary, Keywords: keywords, Kind: KindAction, Risk: risk, Admin: admin,
		Duration: dur, Restart: restart, Reversible: reversible, Command: command, Phase: "5"}
}

// Support Pack II: guided repairs, cleanup, and reports. Every one goes through the runner.
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

// Phase 6 reports.
func init() {
	support := action("report.support", AreaReports, "Support report", "Writes one file with system, disk, network, security, startup, programs, and ports. Redacts user, host, IP, and MAC by default. Stays on this PC.",
		RiskChange, AdminOptional, DurationSeconds, true, false, []string{"report", "support"}, "send to support", "diagnostics report", "export")
	gp := action("report.gpresult", AreaReports, "Group Policy report", "Saves the Group Policy result as an HTML file, redacted. Computer policy needs administrator.",
		RiskChange, AdminOptional, DurationSeconds, true, false, []string{"report", "gpresult"}, "gpo", "group policy", "gpresult")
	support.Phase, gp.Phase = "6", "6"
	tools = append(tools, support, gp)
}
