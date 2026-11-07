package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/panels"
	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/system"
	"github.com/spf13/cobra"
)

type jsonEnvelope struct {
	SchemaVersion int    `json:"schemaVersion"`
	Command       string `json:"command"`
	Data          any    `json:"data"`
}

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
	command.Flags().BoolVar(&asJSON, "json", false, "Write structured JSON output")
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
	command.Flags().BoolVar(&asJSON, "json", false, "Write structured JSON output")
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
	info.Flags().BoolVar(&asJSON, "json", false, "Write structured JSON output")
	command.AddCommand(info)
	return command
}

func newNetworkCommand(services app.Services) *cobra.Command {
	command := &cobra.Command{Use: "network", Short: "Inspect network adapters", Args: cobra.NoArgs}
	var asJSON bool
	status := &cobra.Command{
		Use:   "status",
		Short: "Show adapter state and assigned addresses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			adapters, err := services.Network.NetworkAdapters()
			if err != nil {
				return fmt.Errorf("read network adapters: %w", err)
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "network status", adapters)
			}
			return writeNetworkTable(cmd.OutOrStdout(), adapters)
		},
	}
	status.Flags().BoolVar(&asJSON, "json", false, "Write structured JSON output")
	command.AddCommand(status)
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
	list.Flags().BoolVar(&asJSON, "json", false, "Write structured JSON output")
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
	command.Flags().BoolVar(&asJSON, "json", false, "Write structured JSON output")
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
	command.Flags().BoolVar(&asJSON, "json", false, "Write structured JSON output")
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

func writeJSON(w io.Writer, command string, data any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonEnvelope{SchemaVersion: 1, Command: command, Data: data})
}

func writeProcessTable(w io.Writer, entries []processes.Entry) error {
	if len(entries) == 0 {
		_, err := fmt.Fprintln(w, "No processes found.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "PID\tNAME\tPARENT PID\tPATH")
	for _, entry := range entries {
		fmt.Fprintf(table, "%d\t%s\t%d\t%s\n", entry.PID, entry.Name, entry.ParentID, entry.Path)
	}
	return table.Flush()
}

func writePortTable(w io.Writer, listeners []ports.Listener) error {
	if len(listeners) == 0 {
		_, err := fmt.Fprintln(w, "No listening TCP sockets found.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "ADDRESS\tPORT\tPID\tPROCESS")
	for _, listener := range listeners {
		processName := listener.ProcessName
		if processName == "" {
			processName = "<unavailable>"
		}
		fmt.Fprintf(table, "%s\t%d\t%d\t%s\n", listener.Address, listener.Port, listener.PID, processName)
	}
	return table.Flush()
}

func writeSystemSummary(w io.Writer, snapshot system.Snapshot) error {
	_, err := fmt.Fprintf(w,
		"Host: %s\nWindows: %s\nDisplay version: %s\nBuild: %s\nLogical processors: %d\nMemory: %s total, %s available\nUptime: %s\n",
		snapshot.Hostname,
		snapshot.ProductName,
		valueOrUnavailable(snapshot.DisplayVersion),
		valueOrUnavailable(snapshot.Build),
		snapshot.LogicalProcessors,
		humanBytes(snapshot.TotalMemoryBytes),
		humanBytes(snapshot.AvailableMemoryBytes),
		snapshot.Uptime,
	)
	return err
}

func writeNetworkTable(w io.Writer, adapters []network.Adapter) error {
	if len(adapters) == 0 {
		_, err := fmt.Fprintln(w, "No network adapters found.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "INDEX\tNAME\tSTATUS\tADDRESSES")
	for _, adapter := range adapters {
		fmt.Fprintf(table, "%d\t%s\t%s\t%s\n", adapter.Index, adapter.Name, adapter.Status, strings.Join(adapter.Addresses, ", "))
	}
	return table.Flush()
}

func writePanelTable(w io.Writer, items []panels.Entry) error {
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "KEY\tNAME\tDESCRIPTION")
	for _, item := range items {
		fmt.Fprintf(table, "%s\t%s\t%s\n", item.Key, item.Name, item.Description)
	}
	return table.Flush()
}

func valueOrUnavailable(value string) string {
	if value == "" {
		return "unavailable"
	}
	return value
}

func humanBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(value)/float64(div), "KMGTPE"[exp])
}
