// Package actionlog appends one JSON line per action to a local monthly file.
// Output bodies are never stored. Failure to log never fails an action.
package actionlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Time       time.Time `json:"time"`
	Tool       string    `json:"tool"`
	Command    string    `json:"command"`
	Elevated   bool      `json:"elevated"`
	ExitCode   int       `json:"exitCode"`
	DurationMS int64     `json:"durationMs"`
	Outcome    string    `json:"outcome"`
}

// Dir returns the log directory, or "" when no per-user location is known.
func Dir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return ""
	}
	return filepath.Join(base, "Wintonicz", "logs")
}

// Append writes the entry and returns the file path used ("" if nothing was written).
func Append(entry Entry) string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	path := filepath.Join(dir, "actions-"+entry.Time.Format("2006-01")+".jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return ""
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(entry); err != nil {
		return ""
	}
	return path
}
