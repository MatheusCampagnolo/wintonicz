// Package report builds the local support report and the redacted Group Policy
// report. Reports are only ever written to a file the user chose; nothing is sent.
package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/app"
	"github.com/MatheusCampagnolo/wintonicz/internal/maint"
	"github.com/MatheusCampagnolo/wintonicz/internal/redact"
	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
	"github.com/MatheusCampagnolo/wintonicz/internal/services"
)

// Section is one part of the report. A failed collector is recorded, not hidden.
type Section struct {
	Name  string `json:"name"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// Collect reads the diagnostics in parallel. Every section is read-only.
func Collect(s app.Services) []Section {
	type job struct {
		name string
		run  func() (any, error)
	}
	jobs := []job{
		{"System", func() (any, error) { return s.System.SystemSnapshot() }},
		{"Resources", func() (any, error) { return s.Resources.ResourceSnapshot() }},
		{"Storage", func() (any, error) { return s.Storage.ListVolumes() }},
		{"Disk health", func() (any, error) { return s.DiskHealth.ListDiskHealth() }},
		{"Network adapters", func() (any, error) { return s.Network.NetworkAdapters() }},
		{"Security", func() (any, error) { return s.Security.SecurityStatus() }},
		{"Key services", func() (any, error) {
			all, err := s.Services.ListServices()
			if err != nil {
				return nil, err
			}
			return services.Key(all), nil
		}},
		{"Startup items", func() (any, error) { return s.Startup.ListStartup() }},
		{"Installed programs", func() (any, error) { return s.Programs.ListPrograms() }},
		{"Listening TCP ports", func() (any, error) { return s.Ports.ListListeners() }},
	}
	sections := make([]Section, len(jobs))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := j.run()
			sections[i] = Section{Name: j.name, Data: data}
			if err != nil {
				sections[i] = Section{Name: j.name, Error: err.Error()}
			}
		}()
	}
	wg.Wait()
	return sections
}

// Render turns sections into text. format is txt, md, or json.
func Render(format string, sections []Section, generated time.Time, version string, masked []string) (string, error) {
	switch format {
	case "json":
		doc := map[string]any{
			"schemaVersion": 1, "tool": "wintonicz", "version": version,
			"generatedAt": generated.Format(time.RFC3339), "redaction": masked, "sections": sections,
		}
		data, err := json.MarshalIndent(doc, "", "  ")
		return string(data) + "\n", err
	case "txt", "md":
	default:
		return "", errors.New("format must be txt, md, or json")
	}
	var b strings.Builder
	title := "Wintonicz support report"
	if format == "md" {
		fmt.Fprintf(&b, "# %s\n\n", title)
	} else {
		fmt.Fprintf(&b, "%s\n%s\n\n", title, strings.Repeat("=", len(title)))
	}
	fmt.Fprintf(&b, "Generated: %s · Wintonicz %s\n", generated.Format("2006-01-02 15:04:05 -07:00"), version)
	if len(masked) == 0 {
		b.WriteString("Redaction: none. This file contains personal and network identifiers.\n")
	} else {
		fmt.Fprintf(&b, "Redaction: %s\n", strings.Join(masked, ", "))
	}
	for _, s := range sections {
		if format == "md" {
			fmt.Fprintf(&b, "\n## %s\n\n", s.Name)
		} else {
			fmt.Fprintf(&b, "\n%s\n%s\n", strings.ToUpper(s.Name), strings.Repeat("-", len(s.Name)))
		}
		if s.Error != "" {
			fmt.Fprintf(&b, "unavailable: %s\n", s.Error)
			continue
		}
		writeValue(&b, s.Data)
	}
	return b.String(), nil
}

// writeValue prints a section by converting it to generic JSON values first, so
// every feature type renders the same way without a formatter per type.
func writeValue(b *strings.Builder, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintf(b, "unavailable: %v\n", err)
		return
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Fprintf(b, "unavailable: %v\n", err)
		return
	}
	switch t := v.(type) {
	case []any:
		if len(t) == 0 {
			b.WriteString("(none)\n")
		}
		for _, item := range t {
			fmt.Fprintf(b, "- %s\n", inline(item))
		}
	case map[string]any:
		for _, k := range orderedKeys(t) {
			fmt.Fprintf(b, "%s: %s\n", k, inline(t[k]))
		}
	default:
		fmt.Fprintf(b, "%v\n", v)
	}
}

func orderedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if (keys[i] == "name") != (keys[j] == "name") {
			return keys[i] == "name"
		}
		return keys[i] < keys[j]
	})
	return keys
}

func inline(v any) string {
	switch t := v.(type) {
	case map[string]any:
		parts := make([]string, 0, len(t))
		for _, k := range orderedKeys(t) {
			if s := inline(t[k]); s != "" {
				parts = append(parts, k+"="+s)
			}
		}
		return strings.Join(parts, "; ")
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, inline(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	}
	return fmt.Sprint(v)
}

// Specs returns the report actions keyed by catalog tool ID.
func Specs(s app.Services, version string) map[string]runner.Spec {
	redactParam := runner.ParamSpec{Name: "redact", Label: "Redact", Default: "username,host,ip,mac",
		Help: "Comma list of username, host, ip, mac; or all, or none. Applied to the final file."}
	return map[string]runner.Spec{
		"report.support": {
			Params: []runner.ParamSpec{
				{Name: "format", Label: "Format", Default: "md", Choices: []string{"md", "txt", "json"}},
				redactParam,
				{Name: "output", Label: "Output file", Help: "Leave empty for Documents\\Wintonicz\\reports\\support-<time>.<format>"},
			},
			Build: func(p map[string]string) (runner.Plan, error) { return supportPlan(s, version, p) },
		},
		"report.gpresult": {
			Params: []runner.ParamSpec{
				redactParam,
				{Name: "output", Label: "Output file (.html)", Help: "Leave empty for Documents\\Wintonicz\\reports\\gpresult-<time>.html"},
			},
			Build: gpresultPlan,
		},
	}
}

func redactor(list string) (*redact.Redactor, redact.Options, error) {
	opts, err := redact.Parse(list)
	if err != nil {
		return nil, opts, err
	}
	host, _ := os.Hostname()
	return redact.New(opts, os.Getenv("USERNAME"), host), opts, nil
}

func outputPath(explicit, prefix, ext string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		path, err := maint.CleanLocalDir(strings.TrimSpace(explicit))
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(filepath.Ext(path), "."+ext) {
			return "", fmt.Errorf("the output file must end in .%s", ext)
		}
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("cannot find your user folder; pass an output file")
	}
	return filepath.Join(home, "Documents", "Wintonicz", "reports", prefix+"-"+time.Now().Format("2006-01-02_150405")+"."+ext), nil
}

func writePrivate(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func summary(c redact.Counts, opts redact.Options) string {
	if len(opts.Names()) == 0 {
		return "NO redaction: the file will contain your user name, computer name, IP and MAC addresses."
	}
	return fmt.Sprintf("Redacted %d values (user %d, host %d, ip %d, mac %d). Masking is pattern-based: review the file before sharing, and note that version numbers shaped like IP addresses are masked too.",
		c.Total(), c.Username, c.Host, c.IP, c.MAC)
}

func supportPlan(s app.Services, version string, p map[string]string) (runner.Plan, error) {
	format := p["format"]
	if format == "" {
		format = "md"
	}
	r, opts, err := redactor(p["redact"])
	if err != nil {
		return runner.Plan{}, err
	}
	dest, err := outputPath(p["output"], "support", format)
	if err != nil {
		return runner.Plan{}, err
	}
	sections := Collect(s)
	raw, err := Render(format, sections, time.Now(), version, opts.Names())
	if err != nil {
		return runner.Plan{}, err
	}
	text, counts := r.Apply(raw)
	lines := strings.Split(text, "\n")
	preview := lines
	if len(preview) > 5 {
		preview = preview[:5]
	}
	items := append([]string{"Writes " + dest, summary(counts, opts), "Sections: " + sectionNames(sections)}, "Preview:")
	for _, l := range preview {
		items = append(items, "  "+l)
	}
	return runner.Plan{Tool: "report.support", Preview: runner.Preview{
		Title:    "Create a support report",
		Summary:  "Collects read-only diagnostics into one file you can send to support. It stays on this PC; nothing is uploaded. The process list and file contents are not included.",
		Command:  "write " + dest,
		Duration: "a few seconds",
		Undo:     "Delete the file.",
		Items:    items,
	}, Do: func(ctx context.Context, emit func(string)) error {
		if err := writePrivate(dest, text); err != nil {
			return err
		}
		emit(fmt.Sprintf("Saved %s (%d bytes).", dest, len(text)))
		emit(summary(counts, opts))
		return nil
	}}, nil
}

func sectionNames(sections []Section) string {
	names := make([]string, len(sections))
	for i, s := range sections {
		names[i] = s.Name
		if s.Error != "" {
			names[i] += " (unavailable)"
		}
	}
	return strings.Join(names, ", ")
}

func gpresultPlan(p map[string]string) (runner.Plan, error) {
	exe, err := runner.SystemExe("gpresult.exe")
	if err != nil {
		return runner.Plan{}, err
	}
	r, opts, err := redactor(p["redact"])
	if err != nil {
		return runner.Plan{}, err
	}
	dest, err := outputPath(p["output"], "gpresult", "html")
	if err != nil {
		return runner.Plan{}, err
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("wintonicz-gpresult-%d.html", time.Now().UnixNano()))
	args := []string{"/h", tmp, "/f"}
	generate := runner.Plan{Tool: "report.gpresult", Exe: exe, Args: args, Preview: runner.Preview{Command: runner.CommandLine(exe, args)}}
	finish := runner.Plan{Tool: "report.gpresult", Preview: runner.Preview{Command: "redact and save " + dest},
		Do: func(ctx context.Context, emit func(string)) error {
			defer os.Remove(tmp)
			data, err := os.ReadFile(tmp)
			if err != nil {
				return fmt.Errorf("gpresult produced no report (on a PC with no policy data this can happen): %w", err)
			}
			text, counts := r.Apply(string(data))
			if err := writePrivate(dest, text); err != nil {
				return err
			}
			emit(fmt.Sprintf("Saved %s (%d bytes).", dest, len(text)))
			emit(summary(counts, opts))
			return nil
		}}
	return runner.Plan{Tool: "report.gpresult", Steps: []runner.Plan{generate, finish}, Preview: runner.Preview{
		Title:    "Group Policy report",
		Summary:  "Asks Windows for the Group Policy result and saves it as an HTML file. It can reveal user names, domain names, and security policy, so it is redacted before it is saved. Computer-level policy needs administrator rights; user-level policy does not.",
		Command:  runner.CommandLine(exe, args) + "  →  redact  →  " + dest,
		Duration: "10–60 seconds",
		Undo:     "Delete the file. The temporary copy is removed automatically.",
		Items:    []string{"Writes " + dest, "Redaction: " + strings.Join(opts.Names(), ", ") + " (pattern-based; domain and group names are NOT masked, so review before sharing)"},
	}}, nil
}
