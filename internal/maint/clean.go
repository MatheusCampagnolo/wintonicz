package maint

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MatheusCampagnolo/wintonicz/internal/processes"
	"github.com/MatheusCampagnolo/wintonicz/internal/runner"
)

// ---- temp cleanup ----

type tempScan struct {
	files []string
	dirs  []string
	bytes int64
}

// scanTemp lists regular files under root older than the cutoff. It never follows
// links or junctions out of root and never lists the root itself.
func scanTemp(root string, olderThan time.Duration) (tempScan, error) {
	var scan tempScan
	cutoff := time.Now().Add(-olderThan)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if path == root {
			return nil
		}
		if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			scan.dirs = append(scan.dirs, path)
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().After(cutoff) {
			return nil
		}
		scan.files = append(scan.files, path)
		scan.bytes += info.Size()
		return nil
	})
	return scan, err
}

// validTempRoot refuses anything that is not clearly a temp folder.
func validTempRoot(root string) error {
	if root == "" {
		return errors.New("the temp folder is not set")
	}
	clean := filepath.Clean(root)
	if len(clean) <= 3 || strings.Count(clean, string(filepath.Separator)) < 2 {
		return fmt.Errorf("refusing to clean %q: it is too close to a drive root", clean)
	}
	base := strings.ToLower(filepath.Base(clean))
	if base != "temp" && base != "tmp" {
		return fmt.Errorf("refusing to clean %q: its name is not Temp", clean)
	}
	return nil
}

func tempPlan(p map[string]string, system bool) (runner.Plan, error) {
	olderThan, err := parseHours(p)
	if err != nil {
		return runner.Plan{}, err
	}
	root, tool, title := os.TempDir(), "clean.temp", "Clean your Temp folder"
	if system {
		root, tool, title = filepath.Join(os.Getenv("SystemRoot"), "Temp"), "clean.systemtemp", "Clean the Windows Temp folder"
	}
	if err := validTempRoot(root); err != nil {
		return runner.Plan{}, err
	}
	scan, err := scanTemp(root, olderThan)
	if err != nil {
		return runner.Plan{}, fmt.Errorf("scan %s: %w", root, err)
	}
	sample := scan.files
	if len(sample) > 8 {
		sample = sample[:8]
	}
	items := append([]string{fmt.Sprintf("%d files, %s, older than %d hours, in %s", len(scan.files), humanBytes(scan.bytes), int(olderThan.Hours()), root)}, sample...)
	if len(scan.files) > len(sample) {
		items = append(items, fmt.Sprintf("… and %d more", len(scan.files)-len(sample)))
	}
	return runner.Plan{Tool: tool, Preview: runner.Preview{
		Title:       title,
		Summary:     "Permanently deletes the files listed below. Files in use are skipped. Folders left empty are removed. Nothing outside this folder is touched, and links are never followed.",
		Command:     "delete files older than " + strconv.Itoa(int(olderThan.Hours())) + "h under " + root,
		Admin:       system,
		Duration:    "seconds to a minute",
		Undo:        "Not reversible: deleted files do not go to the Recycle Bin. Apps recreate the temporary files they need.",
		Items:       items,
		ConfirmWord: "DELETE",
	}, Do: func(ctx context.Context, emit func(string)) error {
		return removeOld(ctx, root, olderThan, emit)
	}}, nil
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// ---- recycle bin ----

func recycleBinPlan() (runner.Plan, error) {
	items, size, err := recycleBinInfo()
	if err != nil {
		return runner.Plan{}, fmt.Errorf("read Recycle Bin: %w", err)
	}
	return runner.Plan{Tool: "clean.recyclebin", Preview: runner.Preview{
		Title:       "Empty the Recycle Bin",
		Summary:     "Permanently deletes everything in the Recycle Bin on all drives.",
		Command:     "empty Recycle Bin (all drives)",
		Duration:    "seconds",
		Undo:        "Not reversible. Check the bin first if you are unsure.",
		Items:       []string{fmt.Sprintf("%d items, %s", items, humanBytes(size))},
		ConfirmWord: "EMPTY",
	}, Do: func(ctx context.Context, emit func(string)) error {
		if err := emptyRecycleBin(); err != nil {
			return err
		}
		emit("Recycle Bin emptied.")
		return nil
	}}, nil
}

// ---- end a process ----

var protectedProcesses = map[string]bool{
	"system": true, "system idle process": true, "[system process]": true, "registry": true, "memory compression": true,
	"secure system": true, "smss.exe": true, "csrss.exe": true, "wininit.exe": true, "winlogon.exe": true,
	"services.exe": true, "lsass.exe": true, "lsaiso.exe": true, "svchost.exe": true, "dwm.exe": true,
	"fontdrvhost.exe": true, "wudfhost.exe": true, "sihost.exe": true,
}

func endProcessPlan(p map[string]string, list func() ([]processes.Entry, error)) (runner.Plan, error) {
	pid64, err := strconv.ParseUint(strings.TrimSpace(p["pid"]), 10, 32)
	if err != nil || pid64 == 0 {
		return runner.Plan{}, errors.New("pid must be a positive number")
	}
	pid := uint32(pid64)
	if pid <= 4 || int(pid) == os.Getpid() {
		return runner.Plan{}, errors.New("that process cannot be ended (system process or Wintonicz itself)")
	}
	all, err := list()
	if err != nil {
		return runner.Plan{}, fmt.Errorf("look up process %d: %w", pid, err)
	}
	var target *processes.Entry
	for i := range all {
		if all[i].PID == pid {
			target = &all[i]
		}
	}
	if target == nil {
		return runner.Plan{}, fmt.Errorf("no running process has PID %d", pid)
	}
	if protectedProcesses[strings.ToLower(target.Name)] {
		return runner.Plan{}, fmt.Errorf("%s is a protected Windows process and will not be ended", target.Name)
	}
	path := target.Path
	if path == "" {
		path = "(path unavailable)"
	}
	return runner.Plan{Tool: "tools.process.end", Preview: runner.Preview{
		Title:       fmt.Sprintf("End %s (PID %d)", target.Name, pid),
		Summary:     "Stops this process immediately, like Task Manager's End task. Unsaved work in it is lost. Other processes it started keep running.",
		Command:     fmt.Sprintf("terminate process %d", pid),
		Admin:       false,
		Duration:    "instant",
		Undo:        "Not reversible. Start the program again if you need it.",
		Items:       []string{target.Name, path, "PID " + strconv.FormatUint(pid64, 10)},
		ConfirmWord: strconv.FormatUint(pid64, 10),
	}, Do: func(ctx context.Context, emit func(string)) error {
		// Confirm it is still the same program before killing: PIDs get reused.
		now, err := list()
		if err != nil {
			return err
		}
		for _, e := range now {
			if e.PID == pid {
				if !strings.EqualFold(e.Name, target.Name) {
					return fmt.Errorf("PID %d now belongs to %s, not %s; nothing was ended", pid, e.Name, target.Name)
				}
				if err := terminateProcess(pid); err != nil {
					return err
				}
				emit(fmt.Sprintf("Ended %s (PID %d).", target.Name, pid))
				return nil
			}
		}
		return fmt.Errorf("PID %d is no longer running", pid)
	}}, nil
}

// ---- download and verify ----

var sha256Hex = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func downloadPlan(p map[string]string) (runner.Plan, error) {
	raw := strings.TrimSpace(p["url"])
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return runner.Plan{}, errors.New("only https:// addresses are allowed")
	}
	expected := strings.ToLower(strings.TrimSpace(p["sha256"]))
	if expected != "" && !sha256Hex.MatchString(expected) {
		return runner.Plan{}, errors.New("sha256 must be 64 hexadecimal characters")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return runner.Plan{}, err
	}
	dir := filepath.Join(home, "Downloads")
	name := safeFileName(filepath.Base(u.Path))
	dest := uniquePath(filepath.Join(dir, name))
	hashNote := "No expected hash was given; the SHA-256 will be shown so you can compare it with the publisher."
	if expected != "" {
		hashNote = "The file is deleted if its SHA-256 is not " + expected + "."
	}
	return runner.Plan{Tool: "report.download", Preview: runner.Preview{
		Title:    "Download and verify a file",
		Summary:  "Downloads one file over HTTPS into your Downloads folder and shows its SHA-256. It is never opened or run. " + hashNote,
		Command:  "GET " + u.String() + " → " + dest,
		Duration: "depends on size and connection",
		Undo:     "Delete the downloaded file.",
		Items:    []string{u.Host, dest},
	}, Do: func(ctx context.Context, emit func(string)) error {
		return download(ctx, u.String(), dest, expected, emit)
	}}, nil
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeFileName(name string) string {
	name = unsafeName.ReplaceAllString(name, "_")
	name = strings.Trim(name, "._")
	if name == "" {
		return "download.bin"
	}
	return name
}

func uniquePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return path
}

const maxDownload = 4 << 30

func download(ctx context.Context, address, dest, expected string, emit func(string)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	client := &http.Client{CheckRedirect: func(r *http.Request, _ []*http.Request) error {
		if r.URL.Scheme != "https" {
			return errors.New("refusing to follow a redirect to a non-HTTPS address")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server answered %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	part := dest + ".part"
	out, err := os.OpenFile(part, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(resp.Body, maxDownload+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written == 0 || written > maxDownload {
		os.Remove(part)
		switch {
		case copyErr != nil:
			return copyErr
		case written == 0:
			return errors.New("the download was empty")
		case written > maxDownload:
			return errors.New("the file is larger than 4 GB; refusing to keep it")
		}
		return closeErr
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if expected != "" && sum != expected {
		os.Remove(part)
		return fmt.Errorf("SHA-256 mismatch: expected %s but the file is %s. The file was deleted.", expected, sum)
	}
	if err := os.Rename(part, dest); err != nil {
		os.Remove(part)
		return err
	}
	emit("Saved " + dest + " (" + humanBytes(written) + ")")
	emit("SHA-256 " + sum)
	if expected != "" {
		emit("Matches the expected hash.")
	} else {
		emit("Compare this hash with the one the publisher lists before you run the file.")
	}
	return nil
}

// removeOld deletes the files a fresh scan finds, then removes folders left empty.
// The scan is repeated at run time because the folder may have changed since the preview.
func removeOld(ctx context.Context, root string, olderThan time.Duration, emit func(string)) error {
	fresh, err := scanTemp(root, olderThan)
	if err != nil {
		return err
	}
	removed, failed := 0, 0
	var freed int64
	for i, f := range fresh.files {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		info, statErr := os.Lstat(f)
		if err := os.Remove(f); err != nil {
			failed++
		} else {
			removed++
			if statErr == nil {
				freed += info.Size()
			}
		}
		if (i+1)%500 == 0 {
			emit(fmt.Sprintf("… %d of %d files handled", i+1, len(fresh.files)))
		}
	}
	sort.Slice(fresh.dirs, func(i, j int) bool { return len(fresh.dirs[i]) > len(fresh.dirs[j]) })
	for _, d := range fresh.dirs {
		_ = os.Remove(d) // fails harmlessly when the folder is not empty
	}
	emit(fmt.Sprintf("Removed %d files (%s). Skipped %d that are in use or protected.", removed, humanBytes(freed), failed))
	return nil
}
