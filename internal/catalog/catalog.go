// Package catalog is the single table of tools Wintonicz offers. The CLI, the TUI,
// the command palette, and the AI manifest are all derived from it, so a tool's
// title, risk, and privilege needs are written down exactly once.
package catalog

import (
	"sort"
	"strings"
)

type Area string

const (
	AreaHome     Area = "home"
	AreaInspect  Area = "inspect"
	AreaNetwork  Area = "network"
	AreaRepair   Area = "repair"
	AreaClean    Area = "clean"
	AreaWindows  Area = "windows"
	AreaReports  Area = "reports"
	AreaSettings Area = "settings"
)

type Kind string

const (
	KindView   Kind = "view"
	KindLaunch Kind = "launch"
	KindAction Kind = "action"
)

type Risk string

const (
	RiskRead        Risk = "read"
	RiskLaunch      Risk = "launch"
	RiskChange      Risk = "change"
	RiskDestructive Risk = "destructive"
)

type Admin string

const (
	AdminNone     Admin = "none"
	AdminOptional Admin = "optional"
	AdminRequired Admin = "required"
)

type Duration string

const (
	DurationInstant Duration = "instant"
	DurationSeconds Duration = "seconds"
	DurationMinutes Duration = "minutes"
)

// Tool describes one capability. Metadata only: behavior lives in feature packages.
type Tool struct {
	ID         string   `json:"id"`
	Area       Area     `json:"area"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Keywords   []string `json:"keywords,omitempty"`
	Kind       Kind     `json:"kind"`
	Risk       Risk     `json:"risk"`
	Admin      Admin    `json:"admin"`
	Duration   Duration `json:"duration"`
	Restart    bool     `json:"restart"`
	Reversible bool     `json:"reversible"`
	Command    []string `json:"command,omitempty"`
}

func read(id string, area Area, title, summary string, admin Admin, command []string, keywords ...string) Tool {
	return Tool{ID: id, Area: area, Title: title, Summary: summary, Keywords: keywords, Kind: KindView,
		Risk: RiskRead, Admin: admin, Duration: DurationInstant, Reversible: true, Command: command}
}

var tools = []Tool{
	read("inspect.system", AreaInspect, "System overview", "Windows edition and build, uptime, CPU count, memory, and system drive space.", AdminNone, []string{"system", "info"}, "windows version", "uptime"),
	read("inspect.processes", AreaInspect, "Processes", "Running processes with parent PID and path, filterable by name or PID.", AdminOptional, []string{"process", "list"}, "slow", "what is running", "task manager"),
	read("inspect.ports", AreaInspect, "TCP ports", "Listening TCP ports and the process that owns each.", AdminOptional, []string{"port", "list"}, "port in use", "who is using 8080"),
	read("inspect.services", AreaInspect, "Services", "Windows services with state and process ID.", AdminOptional, []string{"service", "list"}),
	read("inspect.usb", AreaInspect, "USB devices", "USB devices connected right now.", AdminNone, []string{"usb", "list"}, "pendrive", "device not found"),
	read("inspect.storage", AreaInspect, "Storage", "Logical drives with type, free space, and total capacity.", AdminNone, []string{"storage", "list"}, "disk full", "free space"),
	read("inspect.resources", AreaInspect, "Resources", "CPU load over a short sample and physical memory use.", AdminNone, []string{"resource", "snapshot"}, "slow", "high cpu", "out of memory"),
	read("net.status", AreaNetwork, "Network status", "Adapters with addresses, gateways, DNS servers, MTU, and MAC.", AdminNone, []string{"network", "status"}, "no internet", "ip address", "ipconfig"),
	read("inspect.keyservices", AreaInspect, "Key services", "Twelve services that explain common problems (network, updates, printing, sound, security), with why each matters.", AdminNone, []string{"service", "list", "--key"}, "printer not working", "no sound", "windows update stuck"),
	read("inspect.diskhealth", AreaInspect, "Disk health", "Health status Windows reports for each physical disk (SSD, HDD, USB).", AdminNone, []string{"storage", "health"}, "disk failing", "slow disk", "smart"),
	read("inspect.security", AreaInspect, "Security status", "Antivirus products and firewall profiles as Windows Security Center reports them.", AdminNone, []string{"security", "status"}, "antivirus", "defender", "firewall"),
	read("inspect.startup", AreaInspect, "Startup apps", "Programs set to start with Windows or at sign-in (Run keys and Startup folders).", AdminNone, []string{"startup", "list"}, "slow boot", "slow startup"),
	read("inspect.programs", AreaInspect, "Installed programs", "Desktop programs listed in Windows' uninstall registry keys, with version and publisher.", AdminNone, []string{"program", "list"}, "what is installed", "uninstall"),
	read("inspect.why", AreaInspect, "Why is this running?", "Explains why a process exists: who started it, which service or startup entry owns it, and what it listens on. Says what is confirmed and what is inferred.", AdminOptional, []string{"why"}, "what is this process", "unknown process", "suspicious", "why is this running"),
	read("net.ping", AreaNetwork, "Ping", "Sends ICMP echo requests to a host and shows round-trip time, loss, and a live graph. IPv4 only.", AdminNone, []string{"network", "ping"}, "latency", "lag", "packet loss", "ping"),
	read("net.quick", AreaNetwork, "Quick internet check", "Walks from your adapter to the internet in four steps and says where it breaks. Contacts example.com and 1.1.1.1 only.", AdminNone, []string{"network", "quick"}, "no internet", "wifi connected no internet", "dns"),
	read("net.check", AreaNetwork, "Connectivity check", "DNS resolution and one TCP connection to a host and port you choose.", AdminNone, []string{"network", "check"}, "no internet", "cannot reach server"),
	{ID: "win.panels", Area: AreaWindows, Title: "Windows panels", Summary: "Open built-in Windows settings and management panels.", Kind: KindLaunch,
		Risk: RiskLaunch, Admin: AdminNone, Duration: DurationInstant, Reversible: true, Command: []string{"panel", "list"},
		Keywords: []string{"control panel", "device manager", "settings"}},
	{ID: "settings.about", Area: AreaSettings, Title: "About and settings", Summary: "Where Wintonicz keeps its files, which preferences are on, and how to change them.", Kind: KindView,
		Risk: RiskRead, Admin: AdminNone, Duration: DurationInstant, Reversible: true, Command: []string{"config", "get"},
		Keywords: []string{"preferences", "config", "logs", "version"}},
	{ID: "net.flushdns", Area: AreaNetwork, Title: "Flush DNS cache", Summary: "Clears the Windows DNS resolver cache. Names are looked up again on next use.", Kind: KindAction,
		Risk: RiskChange, Admin: AdminRequired, Duration: DurationInstant, Reversible: true, Command: []string{"network", "flushdns"},
		Keywords: []string{"no internet", "wrong website", "dns"}},
}

// All returns the tools sorted by ID. The result is a copy.
func All() []Tool {
	out := make([]Tool, len(tools))
	copy(out, tools)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Find returns the tool with the given ID.
func Find(id string) (Tool, bool) {
	for _, tool := range tools {
		if tool.ID == id {
			return tool, true
		}
	}
	return Tool{}, false
}

// Search ranks tools by a case-insensitive match on title, ID, summary, and keywords.
func Search(query string) []Tool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return All()
	}
	type scored struct {
		tool  Tool
		score int
	}
	var hits []scored
	for _, tool := range All() {
		score := 0
		switch {
		case strings.Contains(strings.ToLower(tool.Title), query):
			score = 3
		case strings.Contains(strings.ToLower(tool.ID), query):
			score = 2
		}
		for _, keyword := range tool.Keywords {
			if strings.Contains(strings.ToLower(keyword), query) && score < 3 {
				score = 3
			}
		}
		if score == 0 && strings.Contains(strings.ToLower(tool.Summary), query) {
			score = 1
		}
		if score > 0 {
			hits = append(hits, scored{tool, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]Tool, len(hits))
	for i, hit := range hits {
		out[i] = hit.tool
	}
	return out
}
