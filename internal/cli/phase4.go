package cli

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/network"
	"github.com/spf13/cobra"
)

func table(w io.Writer) *tabwriter.Writer { return tabwriter.NewWriter(w, 0, 4, 2, ' ', 0) }

func newStorageHealthCommand(deps app.Services) *cobra.Command {
	var asJSON bool
	command := &cobra.Command{
		Use:   "health",
		Short: "Show the health Windows reports for each physical disk",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			disks, err := deps.DiskHealth.ListDiskHealth()
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "storage health", disks)
			}
			t := table(cmd.OutOrStdout())
			fmt.Fprintln(t, "DISK\tTYPE\tBUS\tHEALTH\tSTATUS\tSIZE")
			for _, d := range disks {
				fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\t%s\n", d.Name, d.MediaType, d.BusType, d.Verdict(), d.Operational, humanBytes(d.SizeBytes))
			}
			return t.Flush()
		},
	}
	command.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	return command
}

func newSecurityCommand(deps app.Services) *cobra.Command {
	command := &cobra.Command{Use: "security", Short: "Inspect antivirus and firewall status", Args: cobra.NoArgs}
	var asJSON bool
	status := &cobra.Command{
		Use:   "status",
		Short: "Show antivirus products and firewall profiles from Windows Security Center",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := deps.Security.SecurityStatus()
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "security status", st)
			}
			out := cmd.OutOrStdout()
			if len(st.Antivirus) == 0 {
				fmt.Fprintln(out, "No antivirus product is registered with Windows Security Center.")
			}
			t := table(out)
			for _, p := range st.Antivirus {
				fmt.Fprintf(t, "Antivirus\t%s\t%s\tsignatures %s\n", p.Name, p.State, p.Signatures)
			}
			for _, f := range st.Firewall {
				fmt.Fprintf(t, "Firewall\t%s profile\t%s\t\n", f.Profile, map[bool]string{true: "on", false: "off"}[f.Enabled])
			}
			return t.Flush()
		},
	}
	status.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	command.AddCommand(status)
	return command
}

func newStartupCommand(deps app.Services) *cobra.Command {
	command := &cobra.Command{Use: "startup", Short: "Inspect programs that start with Windows", Args: cobra.NoArgs}
	var asJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List startup items from Run keys and Startup folders",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := deps.Startup.ListStartup()
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "startup list", items)
			}
			t := table(cmd.OutOrStdout())
			fmt.Fprintln(t, "NAME\tSCOPE\tLOCATION\tCOMMAND")
			for _, e := range items {
				fmt.Fprintf(t, "%s\t%s\t%s\t%s\n", e.Name, e.Scope, e.Location, e.Command)
			}
			return t.Flush()
		},
	}
	list.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	command.AddCommand(list)
	return command
}

func newProgramCommand(deps app.Services) *cobra.Command {
	command := &cobra.Command{Use: "program", Short: "Inspect installed programs", Args: cobra.NoArgs}
	var asJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List installed desktop programs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := deps.Programs.ListPrograms()
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "program list", items)
			}
			t := table(cmd.OutOrStdout())
			fmt.Fprintln(t, "NAME\tVERSION\tPUBLISHER\tSCOPE")
			for _, p := range items {
				fmt.Fprintf(t, "%s\t%s\t%s\t%s\n", p.Name, p.Version, p.Publisher, p.Scope)
			}
			return t.Flush()
		},
	}
	list.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	command.AddCommand(list)
	return command
}

func newQuickCheckCommand(deps app.Services) *cobra.Command {
	var asJSON bool
	command := &cobra.Command{
		Use:   "quick",
		Short: "Check the path from your adapter to the internet and say where it breaks",
		Long: fmt.Sprintf("Contacts only %s (DNS), %s and %s (TCP). Sends one DNS query and two TCP connections.",
			network.QuickTargets.DNSName, network.QuickTargets.IPv4, network.QuickTargets.WebHost),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			adapters, err := deps.Network.NetworkAdapters()
			if err != nil {
				return err
			}
			result := network.QuickCheck(cmd.Context(), adapters)
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), "network quick", result)
			}
			out := cmd.OutOrStdout()
			for _, s := range result.Steps {
				mark := "✗"
				if s.OK {
					mark = "✓"
				}
				fmt.Fprintf(out, "%s %-26s %s (%d ms)\n", mark, s.Name, s.Detail, s.DurationMS)
			}
			fmt.Fprintln(out, "\n"+result.Diagnosis)
			return nil
		},
	}
	command.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	return command
}

func newPingCommand(deps app.Services) *cobra.Command {
	var asJSON bool
	var count int
	var timeout time.Duration
	command := &cobra.Command{
		Use:   "ping <host>",
		Short: "Send ICMP echo requests (IPv4) and show round-trip times",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if count < 1 || count > 100 {
				return ExitError{Code: exitUsage, Err: fmt.Errorf("--count must be from 1 to 100")}
			}
			if timeout <= 0 || timeout > 10*time.Second {
				return ExitError{Code: exitUsage, Err: fmt.Errorf("--timeout must be above 0 and at most 10s")}
			}
			ip, err := network.ResolveIPv4(args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			var results []network.PingResult
			for i := 1; i <= count; i++ {
				if cmd.Context().Err() != nil {
					break
				}
				r, err := deps.Pinger.Ping(ip, timeout)
				if err != nil {
					return err
				}
				r.Seq = i
				results = append(results, r)
				if !asJSON {
					if r.OK {
						fmt.Fprintf(out, "Reply from %s: time=%dms\n", r.Address, r.RTTMillis)
					} else {
						fmt.Fprintf(out, "%s: %s\n", r.Address, r.Status)
					}
				}
				if i < count {
					time.Sleep(time.Second)
				}
			}
			stats := network.Summarize(results)
			if asJSON {
				return writeJSON(out, "network ping", map[string]any{"target": args[0], "address": ip.String(), "results": results, "stats": stats, "lossPercent": stats.LossPercent()})
			}
			fmt.Fprintf(out, "\n%d sent, %d received, %.0f%% loss; min/avg/max = %d/%d/%d ms\n", stats.Sent, stats.Received, stats.LossPercent(), stats.MinMS, stats.AvgMS, stats.MaxMS)
			return nil
		},
	}
	command.Flags().IntVarP(&count, "count", "n", 4, "Number of requests (1-100)")
	command.Flags().DurationVar(&timeout, "timeout", 2*time.Second, "Wait per reply")
	command.Flags().BoolVar(&asJSON, "json", jsonDefault(), "Write structured JSON output")
	return command
}
