package change

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func useTempJournal(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "undo")
	old := Dir
	Dir = func() string { return dir }
	t.Cleanup(func() { Dir = old })
	return dir
}

func TestJournalRoundTripAndNewestUndoable(t *testing.T) {
	useTempJournal(t)
	first, err := Save(Entry{Tool: "power.set", Kind: KindPower, Time: time.Now().Add(-time.Hour), Data: map[string]string{"previous": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Save(Entry{Tool: "win.explorer.pref", Kind: KindExplorer, Data: map[string]string{"setting": "hidden-files"}})
	if got, _ := Find(""); got.ID != second.ID {
		t.Errorf("newest = %q, want %q", got.ID, second.ID)
	}
	if err := MarkUndone(second.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := Find(""); got.ID != first.ID {
		t.Errorf("after undoing the newest, got %q, want %q", got.ID, first.ID)
	}
	if _, err := Find("nope"); err == nil {
		t.Error("an unknown ID must be an error")
	}
}

func TestJournalIgnoresRenamedOrForeignFiles(t *testing.T) {
	dir := useTempJournal(t)
	e, _ := Save(Entry{Tool: "power.set", Kind: KindPower})
	if err := os.Rename(filepath.Join(dir, e.ID+".json"), filepath.Join(dir, "copy.json")); err != nil {
		t.Fatal(err)
	}
	if all, _ := List(); len(all) != 0 {
		t.Errorf("a file whose name does not match its ID must be ignored, got %d", len(all))
	}
}

func TestRestorePointIsNeverSelectedForUndo(t *testing.T) {
	useTempJournal(t)
	e, _ := Save(Entry{Tool: "repair.restorepoint", Kind: KindRestorePnt})
	if _, err := Find(""); err == nil {
		t.Error("a restore point must not be picked as the default undo target")
	}
	if _, err := undoPlan(map[string]string{"id": e.ID}); err == nil || !strings.Contains(err.Error(), "System Restore") {
		t.Errorf("err = %v", err)
	}
}

func TestUndoRejectsTamperedEntries(t *testing.T) {
	dir := useTempJournal(t)
	cases := map[string]Entry{
		"power guid":    {Kind: KindPower, Data: map[string]string{"previous": "x /setactive; calc"}},
		"explorer name": {Kind: KindExplorer, Data: map[string]string{"setting": `..\Run`, "previous": "0"}},
		"explorer data": {Kind: KindExplorer, Data: map[string]string{"setting": "hidden-files", "previous": "0 /f & calc"}},
		"wu suffix":     {Kind: KindWUReset, Data: map[string]string{"suffix": "x'; calc; '"}},
		"net outside":   {Kind: KindNetReset, Data: map[string]string{"snapshot": filepath.Join(filepath.Dir(dir), "netreset-20260101-000000.txt")}},
		"net badname":   {Kind: KindNetReset, Data: map[string]string{"snapshot": filepath.Join(dir, "evil.bat")}},
	}
	for name, entry := range cases {
		saved, err := Save(entry)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := undoPlan(map[string]string{"id": saved.ID}); err == nil {
			t.Errorf("%s: a tampered entry was accepted", name)
		}
	}
}

func TestPowerSetRecordsPriorPlanBeforeSwitching(t *testing.T) {
	useTempJournal(t)
	old := query
	query = func(name string, args ...string) (string, error) {
		return "Power Scheme GUID: 381b4222-f694-41f0-9685-ff5bb260df2e  (Balanced)\n", nil
	}
	t.Cleanup(func() { query = old })

	if _, err := powerSetPlan(map[string]string{"plan": "balanced"}); err == nil {
		t.Error("switching to the active plan must be refused")
	}
	plan, err := powerSetPlan(map[string]string{"plan": "saver"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 2 || plan.Steps[0].Do == nil || plan.Steps[1].Exe == "" {
		t.Fatalf("expected record step then command, got %+v", plan.Steps)
	}
	if err := plan.Steps[0].Do(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	e, err := Find("")
	if err != nil || e.Data["previous"] != "381b4222-f694-41f0-9685-ff5bb260df2e" {
		t.Fatalf("journal = %+v, %v", e, err)
	}
	undo, err := undoPlan(map[string]string{"id": e.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := undo.Steps[0].Args; len(got) != 2 || got[0] != "/setactive" || got[1] != e.Data["previous"] {
		t.Errorf("undo args = %v", got)
	}
}

func TestInputsAreValidatedBeforeAnyPlanExists(t *testing.T) {
	if _, err := restorePointPlan(map[string]string{"description": "x'; calc; '"}); err == nil {
		t.Error("a description with quotes must be refused")
	}
	if _, err := explorerPrefPlan(map[string]string{"setting": "run-keys", "value": "show"}); err == nil {
		t.Error("an unknown setting must be refused")
	}
	if _, err := powerSetPlan(map[string]string{"plan": "turbo"}); err == nil {
		t.Error("an unknown plan must be refused")
	}
}
