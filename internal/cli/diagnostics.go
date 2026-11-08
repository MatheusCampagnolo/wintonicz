package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/services"
	"github.com/spf13/cobra"
)

func newProcessCommand(services app.Services) *cobra.Command {
	command := &cobra.Command{Use: "process", Short: "Inspect running processes", Args: cobra.NoArgs}
	command.AddCommand(newProcessListCommand(services), newProcessPortsCommand(services))
	return command
}

func newProcessListCommand(services app.Services) *cobra.Command {
	var nameFilter string
	var asJSON bool
	command := &cobra.Command{
		Use:   "list",
		Short: "List running processes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := services.Processes.ListProcesses()
			if err != nil {
				return fmt.Errorf("list processes: %w", err)
			}
			entries = filterProcesses(entries, nameFilter)
			sort.Slice(entries, func(i, j int) bool {
				if strings.EqualFold(entries[i].Name, entries[j].Name) {
					return entries[i].PID < entries[j].PID
				}
				return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
			})
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "process list", entries)
			}
			return writeProcessTable(cmd.OutOrStdout(), entries)
		},
	}
	command.Flags().StringVarP(&nameFilter, "name", "n", "", "Filter by process name (case-insensitive substring)")
	command.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	return command
}

func newProcessPortsCommand(services app.Services) *cobra.Command {
	var asJSON bool
	command := &cobra.Command{
		Use:   "ports <name-or-pid>",
		Short: "Find listening TCP ports owned by a process",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := services.Processes.ListProcesses()
			if err != nil {
				return fmt.Errorf("list processes: %w", err)
			}
			listeners, err := services.Ports.ListListeners()
			if err != nil {
				return fmt.Errorf("list TCP listeners: %w", err)
			}
			matches := matchProcessListeners(args[0], entries, listeners)
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "process ports", matches)
			}
			return writePortTable(cmd.OutOrStdout(), matches)
		},
	}
	command.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	return command
}

func newPortCommand(services app.Services) *cobra.Command {
	command := &cobra.Command{Use: "port", Short: "Inspect listening TCP ports", Args: cobra.NoArgs}
	command.AddCommand(newPortListCommand(services), newPortFindCommand(services))
	return command
}

func newSystemCommand(services app.Services) *cobra.Command {
	command := &cobra.Command{Use: "system", Short: "Inspect Windows system information", Args: cobra.NoArgs}
	var asJSON bool
	info := &cobra.Command{
		Use:   "info",
		Short: "Show a basic system summary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			snapshot, err := services.System.SystemSnapshot()
			if err != nil {
				return fmt.Errorf("read system information: %w", err)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "system info", snapshot)
			}
			return writeSystemSummary(cmd.OutOrStdout(), snapshot)
		},
	}
	info.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	command.AddCommand(info)
	return command
}

func newServiceCommand(deps app.Services) *cobra.Command {
	command := &cobra.Command{Use: "service", Short: "Inspect Windows services", Args: cobra.NoArgs}
	var asJSON, keyOnly bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List Windows services and their current state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := deps.Services.ListServices()
			if err != nil {
				return fmt.Errorf("list Windows services: %w", err)
			}
			if keyOnly {
				entries = services.Key(entries)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "service list", entries)
			}
			return writeServiceTable(cmd.OutOrStdout(), entries)
		},
	}
	list.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	list.Flags().BoolVar(&keyOnly, "key", false, "Only the curated key services that explain common problems")
	command.AddCommand(list)
	return command
}

func newUSBCommand(deps app.Services) *cobra.Command {
	command := &cobra.Command{Use: "usb", Short: "Inspect connected USB devices", Args: cobra.NoArgs}
	var asJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List currently connected USB devices",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			devices, err := deps.USB.ListConnectedDevices()
			if err != nil {
				return fmt.Errorf("list connected USB devices: %w", err)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "usb list", devices)
			}
			return writeUSBTable(cmd.OutOrStdout(), devices)
		},
	}
	list.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	command.AddCommand(list)
	return command
}

func newStorageCommand(deps app.Services) *cobra.Command {
	command := &cobra.Command{Use: "storage", Short: "Inspect logical drive capacity", Args: cobra.NoArgs}
	var asJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List logical drives and available space",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			volumes, err := deps.Storage.ListVolumes()
			if err != nil {
				return fmt.Errorf("list storage volumes: %w", err)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "storage list", volumes)
			}
			return writeStorageTable(cmd.OutOrStdout(), volumes)
		},
	}
	list.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	command.AddCommand(list, newStorageHealthCommand(deps))
	return command
}

func newResourceCommand(deps app.Services) *cobra.Command {
	command := &cobra.Command{Use: "resource", Short: "Inspect CPU and memory usage", Args: cobra.NoArgs}
	var asJSON bool
	snapshot := &cobra.Command{
		Use:   "snapshot",
		Short: "Show a point-in-time CPU and memory snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := deps.Resources.ResourceSnapshot()
			if err != nil {
				return fmt.Errorf("read resource snapshot: %w", err)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "resource snapshot", result)
			}
			return writeResourceSummary(cmd.OutOrStdout(), result)
		},
	}
	snapshot.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	command.AddCommand(snapshot)
	return command
}

func newNetworkCommand(services app.Services) *cobra.Command {
	command := &cobra.Command{Use: "network", Short: "Inspect network adapters", Args: cobra.NoArgs}
	var statusJSON bool
	status := &cobra.Command{
		Use:   "status",
		Short: "Show adapter addresses, gateways, and DNS servers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			adapters, err := services.Network.NetworkAdapters()
			if err != nil {
				return fmt.Errorf("read network adapters: %w", err)
			}
			if statusJSON {
				return writeJSON(cmd.OutOrStdout(), "network status", adapters)
			}
			return writeNetworkTable(cmd.OutOrStdout(), adapters)
		},
	}
	status.Flags().BoolVar(&statusJSON, "json", jsonDefault(), "Write structured JSON output")
	var timeout time.Duration
	var checkJSON bool
	check := &cobra.Command{
		Use:   "check <host:port>",
		Short: "Check DNS resolution and TCP reachability for a target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := network.CheckConnectivity(cmd.Context(), args[0], timeout)
			if err != nil {
				return err
			}
			if checkJSON {
				return writeJSON(cmd.OutOrStdout(), "network check", result)
			}
			return writeConnectivityResult(cmd.OutOrStdout(), result)
		},
	}
	check.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "Maximum duration for DNS and TCP checks (max 30s)")
	check.Flags().BoolVar(&checkJSON, "json", jsonDefault(), "Write structured JSON output")
	command.AddCommand(status, check, newQuickCheckCommand(services), newPingCommand(services))
	return command
}

func newPanelCommand(services app.Services) *cobra.Command {
	command := &cobra.Command{Use: "panel", Short: "Open allowlisted Windows settings panels", Args: cobra.NoArgs}
	var asJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List supported Windows panels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items := services.Panels.ListPanels()
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "panel list", items)
			}
			return writePanelTable(cmd.OutOrStdout(), items)
		},
	}
	list.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	open := &cobra.Command{
		Use:   "open <panel-key>",
		Short: "Open a panel by its listed key",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := services.Panels.OpenPanel(args[0]); err != nil {
				return fmt.Errorf("open Windows panel: %w", err)
			}
			return nil
		},
	}
	command.AddCommand(list, open)
	return command
}

func newPortListCommand(services app.Services) *cobra.Command {
	var asJSON bool
	command := &cobra.Command{
		Use:   "list",
		Short: "List listening TCP ports",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			listeners, err := services.Ports.ListListeners()
			if err != nil {
				return fmt.Errorf("list TCP listeners: %w", err)
			}
			sortListeners(listeners)
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "port list", listeners)
			}
			return writePortTable(cmd.OutOrStdout(), listeners)
		},
	}
	command.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	return command
}

func newPortFindCommand(services app.Services) *cobra.Command {
	var asJSON bool
	command := &cobra.Command{
		Use:   "find <port>",
		Short: "Find the process listening on a TCP port",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			portNumber, err := strconv.ParseUint(args[0], 10, 16)
			if err != nil || portNumber == 0 {
				return fmt.Errorf("port must be a number from 1 to 65535")
			}
			listeners, err := services.Ports.ListListeners()
			if err != nil {
				return fmt.Errorf("list TCP listeners: %w", err)
			}
			matches := make([]ports.Listener, 0)
			for _, listener := range listeners {
				if listener.Port == uint16(portNumber) {
					matches = append(matches, listener)
				}
			}
			sortListeners(matches)
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "port find", matches)
			}
			if len(matches) == 0 {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "No listening TCP socket found on port %d.\n", portNumber)
				return err
			}
			return writePortTable(cmd.OutOrStdout(), matches)
		},
	}
	command.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	return command
}

func filterProcesses(entries []processes.Entry, nameFilter string) []processes.Entry {
	if nameFilter == "" {
		return entries
	}
	filter := strings.ToLower(nameFilter)
	filtered := make([]processes.Entry, 0)
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name), filter) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func matchProcessListeners(query string, entries []processes.Entry, listeners []ports.Listener) []ports.Listener {
	query = strings.TrimSpace(query)
	pid, err := strconv.ParseUint(query, 10, 32)
	wantedPIDs := make(map[uint32]struct{})
	if err == nil {
		wantedPIDs[uint32(pid)] = struct{}{}
	} else {
		name := strings.TrimSuffix(strings.ToLower(query), ".exe")
		for _, entry := range entries {
			if strings.TrimSuffix(strings.ToLower(entry.Name), ".exe") == name {
				wantedPIDs[entry.PID] = struct{}{}
			}
		}
	}

	matches := make([]ports.Listener, 0)
	for _, listener := range listeners {
		if _, ok := wantedPIDs[listener.PID]; ok {
			matches = append(matches, listener)
		}
	}
	sortListeners(matches)
	return matches
}

func sortListeners(listeners []ports.Listener) {
	sort.Slice(listeners, func(i, j int) bool {
		if listeners[i].Port == listeners[j].Port {
			if listeners[i].Address == listeners[j].Address {
				return listeners[i].PID < listeners[j].PID
			}
			return listeners[i].Address < listeners[j].Address
		}
		return listeners[i].Port < listeners[j].Port
	})
}
