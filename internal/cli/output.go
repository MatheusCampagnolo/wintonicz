package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/MatheusCampagnolo/wintonicz/internal/panels"
	"github.com/MatheusCampagnolo/wintonicz/internal/ports"
	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/resources"
	"github.com/MatheusCampagnolo/wintonicz/internal/services"
	"github.com/MatheusCampagnolo/wintonicz/internal/storage"
	"github.com/MatheusCampagnolo/wintonicz/internal/system"
	"github.com/MatheusCampagnolo/wintonicz/internal/usb"
)

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
	driveSummary := "unavailable"
	if snapshot.WindowsDrive != "" {
		driveSummary = fmt.Sprintf("%s (%s free of %s)", snapshot.WindowsDrive, humanBytes(snapshot.DriveFreeBytes), humanBytes(snapshot.DriveTotalBytes))
	}
	_, err := fmt.Fprintf(w,
		"Host: %s\nWindows: %s\nDisplay version: %s\nBuild: %s\nLogical processors: %d\nMemory: %s total, %s available\nWindows drive: %s\nUptime: %s\n",
		snapshot.Hostname,
		snapshot.ProductName,
		valueOrUnavailable(snapshot.DisplayVersion),
		valueOrUnavailable(snapshot.Build),
		snapshot.LogicalProcessors,
		humanBytes(snapshot.TotalMemoryBytes),
		humanBytes(snapshot.AvailableMemoryBytes),
		driveSummary,
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
	fmt.Fprintln(table, "INDEX\tNAME\tSTATUS\tADDRESSES\tGATEWAYS\tDNS SERVERS")
	for _, adapter := range adapters {
		fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\t%s\n", adapter.Index, adapter.Name, adapter.Status, joinOrNone(adapter.Addresses), joinOrNone(adapter.Gateways), joinOrNone(adapter.DNSServers))
	}
	return table.Flush()
}

func writeConnectivityResult(w io.Writer, result network.ConnectivityResult) error {
	resolution := "failed"
	if result.TargetResolved {
		resolution = "resolved: " + strings.Join(result.ResolvedAddresses, ", ")
	}
	connection := "unreachable"
	if result.TCPConnected {
		connection = "reachable"
	}
	_, err := fmt.Fprintf(w, "Target: %s\nResolution: %s\nTCP: %s\nElapsed: %d ms\n", result.Target, resolution, connection, result.DurationMillis)
	if err != nil {
		return err
	}
	if result.FailureReason != "" {
		_, err = fmt.Fprintf(w, "Details: %s\n", result.FailureReason)
	}
	return err
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func writePanelTable(w io.Writer, items []panels.Entry) error {
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "KEY\tNAME\tDESCRIPTION")
	for _, item := range items {
		fmt.Fprintf(table, "%s\t%s\t%s\n", item.Key, item.Name, item.Description)
	}
	return table.Flush()
}

func writeServiceTable(w io.Writer, entries []services.Entry) error {
	if len(entries) == 0 {
		_, err := fmt.Fprintln(w, "No Windows services found.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tDISPLAY NAME\tSTATE\tPID")
	for _, entry := range entries {
		fmt.Fprintf(table, "%s\t%s\t%s\t%d\n", entry.Name, entry.DisplayName, entry.State, entry.PID)
	}
	return table.Flush()
}

func writeUSBTable(w io.Writer, devices []usb.Device) error {
	if len(devices) == 0 {
		_, err := fmt.Fprintln(w, "No connected USB devices found.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tMANUFACTURER\tCLASS\tLOCATION")
	for _, device := range devices {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", device.Name, valueOrUnavailable(device.Manufacturer), valueOrUnavailable(device.Class), valueOrUnavailable(device.Location))
	}
	return table.Flush()
}

func writeStorageTable(w io.Writer, volumes []storage.Volume) error {
	if len(volumes) == 0 {
		_, err := fmt.Fprintln(w, "No logical drives found.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "DRIVE\tTYPE\tFREE\tTOTAL\tSTATUS")
	for _, volume := range volumes {
		free, total, status := "unavailable", "unavailable", "unavailable"
		if volume.Available {
			free, total, status = humanBytes(volume.Free), humanBytes(volume.Total), "Available"
		} else if volume.Error != "" {
			status += ": " + volume.Error
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", volume.Drive, volume.Type, free, total, status)
	}
	return table.Flush()
}

func writeResourceSummary(w io.Writer, snapshot resources.Snapshot) error {
	_, err := fmt.Fprintf(w,
		"CPU usage: %.1f%% (sampled over %d ms)\nMemory: %s used of %s (%.1f%%)\nMemory available: %s\n",
		snapshot.CPUPercent,
		snapshot.SampleWindowMS,
		humanBytes(snapshot.MemoryUsedBytes),
		humanBytes(snapshot.MemoryTotalBytes),
		snapshot.MemoryPercent,
		humanBytes(snapshot.MemoryAvailableBytes),
	)
	return err
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
