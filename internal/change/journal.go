// Package change holds the reversible changes of Phase 7: power plans, Explorer
// preferences, restore points, Windows Update reset, and network reset. Every change
// first writes a journal entry with the prior state, and `undo` replays that entry.
//
// The journal is data a user-level process can edit, and undo may run elevated, so
// an entry is never a command. It is a typed record (kind plus a few fields), and
// each field is validated again when the undo plan is built.
package change

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Kinds of journal entry. Each has its own undo plan.
const (
	KindPower      = "power"     // Data: previous = GUID of the previously active plan
	KindExplorer   = "explorer"  // Data: setting, previous = previous value or "" when it was not set
	KindWUReset    = "wureset"   // Data: suffix = timestamp used in the renamed folders
	KindNetReset   = "netreset"  // Data: snapshot = path of the netsh dump
	KindRestorePnt = "restorept" // informational: undo is System Restore, not Wintonicz
)

// Entry is one recorded change.
type Entry struct {
	ID      string            `json:"id"`
	Tool    string            `json:"tool"`
	Kind    string            `json:"kind"`
	Time    time.Time         `json:"time"`
	Data    map[string]string `json:"data"`
	Undone  bool              `json:"undone,omitempty"`
	Summary string            `json:"summary"`
}

// Dir is the journal folder. Tests replace it.
var Dir = func() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return ""
	}
	return filepath.Join(base, "Wintonicz", "undo")
}

var errNoDir = errors.New("cannot find a per-user folder for the undo journal (LOCALAPPDATA is not set)")

// Save writes the entry and returns it with its ID filled in.
func Save(e Entry) (Entry, error) {
	dir := Dir()
	if dir == "" {
		return e, errNoDir
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return e, err
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if e.ID == "" {
		e.ID = e.Time.Format("20060102-150405")
		// Two changes in one second get distinct IDs.
		for n := 2; exists(dir, e.ID); n++ {
			e.ID = fmt.Sprintf("%s-%d", e.Time.Format("20060102-150405"), n)
		}
	}
	return e, write(dir, e)
}

func exists(dir, id string) bool {
	_, err := os.Stat(filepath.Join(dir, id+".json"))
	return err == nil
}

func write(dir string, e Entry) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, e.ID+".json"), data, 0o600)
}

// List returns every entry, newest first.
func List() ([]Entry, error) {
	dir := Dir()
	if dir == "" {
		return nil, errNoDir
	}
	files, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			continue
		}
		var e Entry
		if json.Unmarshal(data, &e) != nil || e.ID == "" || e.ID != strings.TrimSuffix(f.Name(), ".json") {
			continue // not ours, or renamed by hand: ignore it
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// Find returns the entry with this ID, or the newest one that can still be undone
// when id is empty.
func Find(id string) (Entry, error) {
	all, err := List()
	if err != nil {
		return Entry{}, err
	}
	for _, e := range all {
		if id == "" && !e.Undone && e.Kind != KindRestorePnt {
			return e, nil
		}
		if id != "" && e.ID == id {
			return e, nil
		}
	}
	if id == "" {
		return Entry{}, errors.New("there is nothing to undo")
	}
	return Entry{}, fmt.Errorf("no journal entry %q; run `undo list`", id)
}

// MarkUndone records that the entry was reversed.
func MarkUndone(id string) error {
	e, err := Find(id)
	if err != nil {
		return err
	}
	e.Undone = true
	return write(Dir(), e)
}
