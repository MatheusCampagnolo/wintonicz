package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/diskhealth"
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/panels"
	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/privilege"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/programs"
	"github.com/MatheusCampagnolo/wintonicz/internal/security"
	"github.com/MatheusCampagnolo/wintonicz/internal/services"
	"github.com/MatheusCampagnolo/wintonicz/internal/startup"
	"github.com/MatheusCampagnolo/wintonicz/internal/storage"
	"github.com/MatheusCampagnolo/wintonicz/internal/usb"
	"github.com/MatheusCampagnolo/wintonicz/internal/why"
)

func rowMatch[T any](row func(T) []string) func(T, string) bool {
	return func(item T, query string) bool {
		return strings.Contains(strings.ToLower(strings.Join(row(item), " ")), query)
	}
}

func newTableWithMatch[T any](cfg tableConfig[T]) *tableScreen[T] {
	if cfg.match == nil {
		cfg.match = rowMatch(cfg.row)
	}
	return newTable(cfg)
}

// buildScreen returns the screen for a catalog tool ID, or nil for tools that are
// actions (they use the action flow instead).
func buildScreen(id string, s app.Services, version string) Screen {
	switch id {
	case "home":
		return newInfo("Home", "windows overview", homeLines(s))
	case "settings.about":
		return newInfo("About and settings", "settings", func() ([]string, error) { return aboutLines(version), nil })
	case "inspect.system":
		return newInfo("System overview", "system overview", systemLines(s))
	case "inspect.resources":
		return newInfo("Resources", "resource snapshot", resourceLines(s))
	case "inspect.processes":
		return newTableWithMatch(tableConfig[processes.Entry]{
			title: "PROCESSES", noun: "processes",
			extra: map[string]extraKey[processes.Entry]{"e": {label: "end process", fn: func(e processes.Entry) tea.Cmd {
				return func() tea.Msg {
					return openActionMsg{toolID: "tools.process.end", preset: map[string]string{"pid": strconv.FormatUint(uint64(e.PID), 10)}}
				}
			}}},
			columns: []column{{"PID", 8}, {"NAME", 28}, {"PARENT", 8}, {"PATH", 0}},
			load:    func() ([]processes.Entry, error) { return s.Processes.ListProcesses() },
			row: func(e processes.Entry) []string {
				return []string{strconv.FormatUint(uint64(e.PID), 10), e.Name, strconv.FormatUint(uint64(e.ParentID), 10), e.Path}
			},
			detail: func(e processes.Entry) string {
				path := e.Path
				if path == "" {
					path = "unavailable for this process (Windows denied access)"
				}
				return fmt.Sprintf("%s\nPID %d · parent %d\n\nPath:\n%s", e.Name, e.PID, e.ParentID, path)
			},
		})
	case "inspect.ports":
		return newTableWithMatch(tableConfig[ports.Listener]{
			title: "TCP PORTS", noun: "TCP listeners",
			columns: []column{{"ADDRESS", 26}, {"PORT", 7}, {"PID", 8}, {"PROCESS", 0}},
			load:    func() ([]ports.Listener, error) { return s.Ports.ListListeners() },
			row: func(l ports.Listener) []string {
				name := l.ProcessName
				if name == "" {
					name = "<unknown>"
				}
				return []string{l.Address, strconv.Itoa(int(l.Port)), strconv.FormatUint(uint64(l.PID), 10), name}
			},
			detail: func(l ports.Listener) string {
				return fmt.Sprintf("Port %d on %s\nOwner: %s (PID %d)", l.Port, l.Address, orUnavailable(l.ProcessName), l.PID)
			},
		})
	case "inspect.services":
		return newTableWithMatch(tableConfig[services.Entry]{
			title: "SERVICES", noun: "services",
			columns: []column{{"SERVICE", 26}, {"STATE", 16}, {"PID", 7}, {"DISPLAY NAME", 0}},
			load:    func() ([]services.Entry, error) { return s.Services.ListServices() },
			row: func(e services.Entry) []string {
				return []string{e.Name, e.State, strconv.FormatUint(uint64(e.PID), 10), e.DisplayName}
			},
			detail: func(e services.Entry) string {
				return fmt.Sprintf("%s\n%s\nState: %s · PID %d\n\nRead-only. Windows may omit services this account cannot query.", e.DisplayName, e.Name, e.State, e.PID)
			},
		})
	case "inspect.usb":
		return newTableWithMatch(tableConfig[usb.Device]{
			title: "USB DEVICES", noun: "USB devices",
			columns: []column{{"DEVICE", 30}, {"MANUFACTURER", 18}, {"CLASS", 12}, {"LOCATION", 0}},
			load:    func() ([]usb.Device, error) { return s.USB.ListConnectedDevices() },
			row: func(d usb.Device) []string {
				return []string{d.Name, orUnavailable(d.Manufacturer), orUnavailable(d.Class), orUnavailable(d.Location)}
			},
			detail: func(d usb.Device) string {
				return fmt.Sprintf("%s\nManufacturer: %s\nClass: %s\nLocation: %s\n\nOnly devices present now are listed.", d.Name, orUnavailable(d.Manufacturer), orUnavailable(d.Class), orUnavailable(d.Location))
			},
		})
	case "inspect.storage":
		return newTableWithMatch(tableConfig[storage.Volume]{
			title: "STORAGE", noun: "logical drives",
			columns: []column{{"DRIVE", 7}, {"TYPE", 11}, {"FREE", 12}, {"TOTAL", 12}, {"STATUS", 0}},
			load:    func() ([]storage.Volume, error) { return s.Storage.ListVolumes() },
			row: func(v storage.Volume) []string {
				if !v.Available {
					return []string{v.Drive, v.Type, "—", "—", "unavailable"}
				}
				return []string{v.Drive, v.Type, formatBytes(v.Free), formatBytes(v.Total), "available"}
			},
			detail: func(v storage.Volume) string {
				if !v.Available {
					return fmt.Sprintf("%s (%s)\nSpace unavailable.\n%s", v.Drive, v.Type, v.Error)
				}
				used := 0.0
				if v.Total > 0 {
					used = float64(v.Total-v.Free) / float64(v.Total) * 100
				}
				return fmt.Sprintf("%s (%s)\n%s %.0f%% used\nFree to you: %s\nTotal: %s", v.Drive, v.Type, bar(used, 14), used, formatBytes(v.Free), formatBytes(v.Total))
			},
		})
	case "net.status":
		return newTableWithMatch(tableConfig[network.Adapter]{
			title: "NETWORK ADAPTERS", noun: "network adapters",
			columns: []column{{"IDX", 5}, {"ADAPTER", 26}, {"STATE", 9}, {"ADDRESSES", 0}},
			load:    func() ([]network.Adapter, error) { return s.Network.NetworkAdapters() },
			row: func(a network.Adapter) []string {
				return []string{strconv.Itoa(a.Index), a.Name, a.Status, strings.Join(a.Addresses, ", ")}
			},
			detail: func(a network.Adapter) string {
				mtu := "unavailable"
				if a.MTU > 0 {
					mtu = strconv.Itoa(a.MTU)
				}
				return fmt.Sprintf("%s\nMTU %s · MAC %s\n\nGateway:\n%s\n\nDNS servers:\n%s", a.Name, mtu, orUnavailable(a.MAC), orNone(a.Gateways), orNone(a.DNSServers))
			},
		})
	case "win.panels":
		return newTableWithMatch(tableConfig[panels.Entry]{
			title: "WINDOWS PANELS", noun: "Windows panels",
			columns: []column{{"KEY", 24}, {"PANEL", 0}},
			load:    func() ([]panels.Entry, error) { return s.Panels.ListPanels(), nil },
			row:     func(e panels.Entry) []string { return []string{e.Key, e.Name} },
			detail: func(e panels.Entry) string {
				return e.Name + "\n\n" + e.Description + "\n\nEnter opens it. Opening a panel changes nothing by itself."
			},
			enterKey: "open",
			enter: func(e panels.Entry) tea.Cmd {
				return func() tea.Msg {
					if err := s.Panels.OpenPanel(e.Key); err != nil {
						return noticeMsg{text: "Could not open " + e.Name + ": " + err.Error(), isErr: true}
					}
					return noticeMsg{text: "Opened " + e.Name + "."}
				}
			},
		})
	case "inspect.keyservices":
		return newTableWithMatch(tableConfig[services.Entry]{
			title: "KEY SERVICES", noun: "key services",
			columns: []column{{"SERVICE", 20}, {"STATE", 14}, {"PID", 7}, {"DISPLAY NAME", 0}},
			load: func() ([]services.Entry, error) {
				all, err := s.Services.ListServices()
				if err != nil {
					return nil, err
				}
				return services.Key(all), nil
			},
			row: func(e services.Entry) []string {
				return []string{e.Name, e.State, strconv.FormatUint(uint64(e.PID), 10), e.DisplayName}
			},
			detail: func(e services.Entry) string {
				return fmt.Sprintf("%s\n%s\nState: %s\n\nWhy it matters:\n%s\n\nRead-only. To change a service, open the Services panel from Windows panels.", e.DisplayName, e.Name, e.State, services.WhyKey(e.Name))
			},
		})
	case "inspect.diskhealth":
		return newTableWithMatch(tableConfig[diskhealth.Disk]{
			title: "DISK HEALTH", noun: "physical disks",
			columns: []column{{"DISK", 28}, {"TYPE", 12}, {"BUS", 7}, {"HEALTH", 12}, {"SIZE", 0}},
			load:    func() ([]diskhealth.Disk, error) { return s.DiskHealth.ListDiskHealth() },
			row: func(d diskhealth.Disk) []string {
				return []string{d.Name, d.MediaType, d.BusType, d.Verdict(), formatBytes(d.SizeBytes)}
			},
			detail: func(d diskhealth.Disk) string {
				return fmt.Sprintf("%s\n%s · %s · %s\nStatus: %s\n\nThis is the health the storage stack reports. It is not a surface scan and cannot predict every failure; keep backups.", d.Name, d.MediaType, d.BusType, formatBytes(d.SizeBytes), d.Operational)
			},
		})
	case "inspect.security":
		return newInfo("Security status", "security status", func() ([]string, error) {
			st, err := s.Security.SecurityStatus()
			if err != nil {
				return nil, err
			}
			lines := []string{strong("Antivirus")}
			if len(st.Antivirus) == 0 {
				lines = append(lines, "  "+warn("!")+" No product is registered with Windows Security Center.")
			}
			for _, p := range st.Antivirus {
				mark := okText("✓")
				if p.State != "on" || p.Signatures == "out of date" {
					mark = warn("!")
				}
				lines = append(lines, fmt.Sprintf("  %s %s — %s, signatures %s", mark, p.Name, p.State, p.Signatures))
			}
			lines = append(lines, "", strong("Firewall"))
			for _, f := range st.Firewall {
				mark, state := okText("✓"), "on"
				if !f.Enabled {
					mark, state = warn("!"), "off"
				}
				lines = append(lines, fmt.Sprintf("  %s %s profile — %s", mark, f.Profile, state))
			}
			return append(lines, "", muted("Data comes from Windows Security Center. Third-party products and managed PCs may report stale or no data.")), nil
		})
	case "inspect.startup":
		return newTableWithMatch(tableConfig[startup.Entry]{
			title: "STARTUP APPS", noun: "startup items",
			columns: []column{{"NAME", 30}, {"SCOPE", 8}, {"LOCATION", 24}, {"COMMAND", 0}},
			load:    func() ([]startup.Entry, error) { return s.Startup.ListStartup() },
			row:     func(e startup.Entry) []string { return []string{e.Name, e.Scope, e.Location, e.Command} },
			detail: func(e startup.Entry) string {
				return fmt.Sprintf("%s\nScope: %s\nWhere: %s\n\nCommand:\n%s\n\nRead-only. Disable items in Task Manager → Startup apps.", e.Name, e.Scope, e.Location, e.Command)
			},
		})
	case "inspect.programs":
		return newTableWithMatch(tableConfig[programs.Program]{
			title: "INSTALLED PROGRAMS", noun: "programs",
			columns: []column{{"NAME", 40}, {"VERSION", 16}, {"PUBLISHER", 24}, {"SCOPE", 0}},
			load:    func() ([]programs.Program, error) { return s.Programs.ListPrograms() },
			row:     func(p programs.Program) []string { return []string{p.Name, p.Version, p.Publisher, p.Scope} },
			detail: func(p programs.Program) string {
				size := "unknown"
				if p.SizeKB > 0 {
					size = formatBytes(uint64(p.SizeKB) * 1024)
				}
				return fmt.Sprintf("%s\nVersion: %s\nPublisher: %s\nInstalled: %s\nSize: %s\nFor: %s", p.Name, orUnavailable(p.Version), orUnavailable(p.Publisher), orUnavailable(p.InstallDate), size, p.Scope)
			},
		})
	case "net.quick":
		return newInfo("Quick internet check", "quick check", func() ([]string, error) {
			adapters, err := s.Network.NetworkAdapters()
			if err != nil {
				return nil, err
			}
			result := network.QuickCheck(context.Background(), adapters)
			lines := []string{strong("From your adapter outward")}
			for _, step := range result.Steps {
				mark := okText("✓")
				if !step.OK {
					mark = danger("✗")
				}
				lines = append(lines, fmt.Sprintf("  %s %-24s %s (%d ms)", mark, step.Name, step.Detail, step.DurationMS))
			}
			return append(lines, "", strong("Reading"), wrap("  "+result.Diagnosis, 70), "",
				muted("Contacted: "+network.QuickTargets.DNSName+" (DNS), "+network.QuickTargets.IPv4+" and "+network.QuickTargets.WebHost+" (TCP). Press r to run again.")), nil
		})
	case "inspect.why":
		return newWhyScreen(why.Deps{Processes: s.Processes, Ports: s.Ports, Services: s.Services, Startup: s.Startup, Chain: s.Chain})
	case "net.ping":
		return newPingScreen(s.Pinger)
	case "net.check":
		return &connectScreen{}
	}
	return nil
}

// ---- info screens (dashboard-style text) ----

type infoMsg struct {
	gen   int
	lines []string
	err   error
}

type infoScreen struct {
	title, noun string
	load        func() ([]string, error)
	gen         int
	loading     bool
	err         error
	lines       []string
}

func newInfo(title, noun string, load func() ([]string, error)) *infoScreen {
	return &infoScreen{title: title, noun: noun, load: load}
}

func (i *infoScreen) Init() tea.Cmd {
	i.gen++
	i.loading, i.err = true, nil
	gen, load := i.gen, i.load
	return func() tea.Msg {
		lines, err := load()
		return infoMsg{gen: gen, lines: lines, err: err}
	}
}

func (i *infoScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case infoMsg:
		if msg.gen == i.gen {
			i.loading, i.err = false, msg.err
			if msg.err == nil {
				i.lines = msg.lines
			}
		}
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			return i, i.Init()
		}
	}
	return i, nil
}

func (i *infoScreen) Title() string {
	if i.loading {
		return strings.ToUpper(i.title) + " · loading…"
	}
	return strings.ToUpper(i.title)
}
func (i *infoScreen) Editing() bool      { return false }
func (i *infoScreen) Dirty() bool        { return false }
func (i *infoScreen) Hints() []hint      { return []hint{{"r", "refresh"}} }
func (i *infoScreen) Details(int) string { return "" }
func (i *infoScreen) View(w, h int) string {
	switch {
	case i.err != nil:
		return danger("✗ Could not read "+i.noun+": ") + wrap(i.err.Error(), w) + "\n\n" + muted("Press r to try again.")
	case i.loading && len(i.lines) == 0:
		return muted("Loading…")
	}
	return strings.Join(i.lines, "\n")
}

func systemLines(s app.Services) func() ([]string, error) {
	return func() ([]string, error) {
		snap, err := s.System.SystemSnapshot()
		if err != nil {
			return nil, err
		}
		drive := "unavailable"
		if snap.WindowsDrive != "" {
			drive = fmt.Sprintf("%s  %s free of %s", snap.WindowsDrive, formatBytes(snap.DriveFreeBytes), formatBytes(snap.DriveTotalBytes))
		}
		return []string{
			strong("Windows"),
			"  Product   " + orUnavailable(snap.ProductName),
			"  Version   " + orUnavailable(snap.DisplayVersion),
			"  Build     " + orUnavailable(snap.Build),
			"  Host      " + snap.Hostname,
			"  Uptime    " + snap.Uptime,
			"",
			strong("Hardware"),
			fmt.Sprintf("  Logical processors  %d", snap.LogicalProcessors),
			fmt.Sprintf("  Memory              %s total, %s available", formatBytes(snap.TotalMemoryBytes), formatBytes(snap.AvailableMemoryBytes)),
			"  System drive        " + drive,
		}, nil
	}
}

func resourceLines(s app.Services) func() ([]string, error) {
	return func() ([]string, error) {
		r, err := s.Resources.ResourceSnapshot()
		if err != nil {
			return nil, err
		}
		return []string{
			strong("CPU"),
			fmt.Sprintf("  %s  %.1f%%", bar(r.CPUPercent, 30), r.CPUPercent),
			muted(fmt.Sprintf("  System average over a %d ms sample", r.SampleWindowMS)),
			"",
			strong("Physical memory"),
			fmt.Sprintf("  %s  %.1f%%", bar(r.MemoryPercent, 30), r.MemoryPercent),
			fmt.Sprintf("  Used %s of %s · available %s", formatBytes(r.MemoryUsedBytes), formatBytes(r.MemoryTotalBytes), formatBytes(r.MemoryAvailableBytes)),
			"",
			muted("A snapshot, not a live monitor. Press r to sample again."),
		}, nil
	}
}

func homeLines(s app.Services) func() ([]string, error) {
	return func() ([]string, error) {
		var lines, attention []string
		if snap, err := s.System.SystemSnapshot(); err == nil {
			lines = append(lines, strong("Windows"),
				"  "+orUnavailable(snap.ProductName)+" "+snap.DisplayVersion+" (build "+orUnavailable(snap.Build)+")",
				"  "+snap.Hostname+" · up "+snap.Uptime, "")
		} else {
			lines = append(lines, strong("Windows"), "  "+danger("unavailable: "+err.Error()), "")
		}
		lines = append(lines, strong("Resources"))
		if r, err := s.Resources.ResourceSnapshot(); err == nil {
			lines = append(lines,
				fmt.Sprintf("  CPU     %s %5.1f%%", bar(r.CPUPercent, 24), r.CPUPercent),
				fmt.Sprintf("  Memory  %s %5.1f%%", bar(r.MemoryPercent, 24), r.MemoryPercent))
			if r.MemoryPercent >= 90 {
				attention = append(attention, "Memory use is above 90%.")
			}
		} else {
			lines = append(lines, "  "+danger("unavailable: "+err.Error()))
		}
		lines = append(lines, "", strong("Storage"))
		if vols, err := s.Storage.ListVolumes(); err == nil {
			for _, v := range vols {
				if !v.Available || v.Total == 0 {
					continue
				}
				used := float64(v.Total-v.Free) / float64(v.Total) * 100
				lines = append(lines, fmt.Sprintf("  %-4s %s %5.1f%%  %s free", v.Drive, bar(used, 24), used, formatBytes(v.Free)))
				if v.Type == "Fixed" && used >= 90 {
					attention = append(attention, fmt.Sprintf("Drive %s is %.0f%% full.", v.Drive, used))
				}
			}
		} else {
			lines = append(lines, "  "+danger("unavailable: "+err.Error()))
		}
		// Slower checks run in parallel; each failure is simply skipped (the dashboard is a hint, not an audit).
		var disks []diskhealth.Disk
		var sec security.Status
		var secErr error
		var svc []services.Entry
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); disks, _ = s.DiskHealth.ListDiskHealth() }()
		go func() { defer wg.Done(); sec, secErr = s.Security.SecurityStatus() }()
		go func() {
			defer wg.Done()
			if all, err := s.Services.ListServices(); err == nil {
				svc = services.Key(all)
			}
		}()
		wg.Wait()
		for _, d := range disks {
			if d.Health != "" && d.Health != "Healthy" {
				attention = append(attention, fmt.Sprintf("Disk %s reports %s. Back up your files.", d.Name, strings.ToLower(d.Health)))
			}
		}
		if secErr == nil {
			if len(sec.Antivirus) == 0 {
				attention = append(attention, "No antivirus is registered with Windows Security Center.")
			}
			for _, p := range sec.Antivirus {
				if p.State == "off" {
					attention = append(attention, p.Name+" is turned off.")
				} else if p.Signatures == "out of date" {
					attention = append(attention, p.Name+" signatures are out of date.")
				}
			}
			for _, f := range sec.Firewall {
				if !f.Enabled {
					attention = append(attention, "Firewall is off for the "+f.Profile+" profile.")
				}
			}
		}
		critical := map[string]bool{"Dhcp": true, "Dnscache": true, "EventLog": true, "RpcSs": true, "mpssvc": true}
		for _, e := range svc {
			if critical[e.Name] && e.State != "Running" && e.State != "not installed" {
				attention = append(attention, fmt.Sprintf("Service %s is %s.", e.DisplayName, strings.ToLower(e.State)))
			}
		}
		if !privilege.IsElevated() {
			attention = append(attention, "Running as a standard user: some tools show limited data.")
		}
		lines = append(lines, "", strong("Needs attention"))
		if len(attention) == 0 {
			lines = append(lines, "  "+okText("✓")+" Nothing found by the quick checks.")
		}
		for _, a := range attention {
			lines = append(lines, "  "+warn("!")+" "+a)
		}
		lines = append(lines, "", muted("Press r to refresh · Ctrl+K searches every tool."))
		return lines, nil
	}
}

// ---- connectivity check (prompt + result) ----

type connectMsg struct {
	result network.ConnectivityResult
	err    error
}

type connectScreen struct {
	editing bool
	draft   string
	pos     int
	running bool
	result  *network.ConnectivityResult
	err     error
	started bool
}

func (c *connectScreen) Init() tea.Cmd {
	if !c.started {
		c.started, c.editing = true, true
	}
	return nil
}

func (c *connectScreen) Title() string { return "CONNECTIVITY CHECK" }
func (c *connectScreen) Editing() bool { return c.editing }
func (c *connectScreen) Dirty() bool   { return false }
func (c *connectScreen) Details(int) string {
	return "Resolves the name (if needed) and opens one TCP connection to the host and port you choose, then closes it. Nothing else is sent. Not a speed test."
}

func (c *connectScreen) Hints() []hint {
	if c.editing {
		return []hint{{"enter", "run check"}, {"esc", "cancel"}, {"ctrl+u", "clear"}}
	}
	return []hint{{"c", "new check"}}
}

func (c *connectScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case connectMsg:
		c.running, c.err = false, msg.err
		if msg.err == nil {
			c.result = &msg.result
		}
	case tea.KeyPressMsg:
		if c.editing {
			switch msg.String() {
			case "enter":
				target := strings.TrimSpace(c.draft)
				c.editing, c.running, c.result, c.err = false, true, nil, nil
				return c, func() tea.Msg {
					r, err := network.CheckConnectivity(context.Background(), target, 3*time.Second)
					return connectMsg{result: r, err: err}
				}
			case "esc":
				c.editing = false
			default:
				c.draft, c.pos = editTextDraft(c.draft, c.pos, msg, 255)
			}
			return c, nil
		}
		if msg.String() == "c" || msg.String() == "enter" {
			c.editing, c.pos = true, len([]rune(c.draft))
		}
	}
	return c, nil
}

func (c *connectScreen) View(w, h int) string {
	lines := []string{"Target (host:port, for example example.com:443)"}
	if c.editing {
		lines = append(lines, "> "+renderDraft(c.draft, c.pos), "")
	} else {
		lines = append(lines, "> "+c.draft, "")
	}
	switch {
	case c.running:
		lines = append(lines, muted("Checking DNS and TCP…"))
	case c.err != nil:
		lines = append(lines, danger("✗ Could not run: ")+c.err.Error())
	case c.result != nil:
		r := c.result
		resolved := danger("✗ failed")
		if r.TargetResolved {
			resolved = okText("✓ ") + strings.Join(r.ResolvedAddresses, ", ")
		}
		tcp := danger("✗ unreachable")
		if r.TCPConnected {
			tcp = okText("✓ reachable")
		}
		lines = append(lines, "Resolution  "+resolved, fmt.Sprintf("TCP         %s (%d ms)", tcp, r.DurationMillis))
		if r.FailureReason != "" {
			lines = append(lines, "", wrap("Details: "+r.FailureReason, w))
		}
	default:
		lines = append(lines, muted("Type a target and press Enter."))
	}
	return strings.Join(lines, "\n")
}
