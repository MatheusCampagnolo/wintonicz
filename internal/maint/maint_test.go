package maint

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRemoveOldDeletesOnlyOldFilesAndEmptyFolders(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Temp")
	if err := os.MkdirAll(filepath.Join(root, "old-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(root, "old-dir", "a.tmp")
	newFile := filepath.Join(root, "recent.tmp")
	for _, f := range []string{oldFile, newFile} {
		if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(oldFile, past, past); err != nil {
		t.Fatal(err)
	}

	scan, err := scanTemp(root, 24*time.Hour)
	if err != nil || len(scan.files) != 1 || scan.bytes != 4 {
		t.Fatalf("scan = %+v, %v; want exactly the one old file", scan, err)
	}
	var lines []string
	if err := removeOld(context.Background(), root, 24*time.Hour, func(s string) { lines = append(lines, s) }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old file was not deleted")
	}
	if _, err := os.Stat(filepath.Join(root, "old-dir")); !os.IsNotExist(err) {
		t.Error("emptied folder was not removed")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Error("a recent file must survive")
	}
	if len(lines) == 0 {
		t.Error("expected a summary line")
	}
}

func TestValidTempRootRefusesRiskyFolders(t *testing.T) {
	for _, bad := range []string{"", `C:\`, `C:\Users`, `C:\Users\me`, `C:\Users\me\Documents`} {
		if validTempRoot(bad) == nil {
			t.Errorf("validTempRoot(%q) accepted a risky folder", bad)
		}
	}
	if err := validTempRoot(`C:\Users\me\AppData\Local\Temp`); err != nil {
		t.Errorf("a real temp folder was refused: %v", err)
	}
}

func TestCleanLocalDirRefusesUNCRelativeAndRoot(t *testing.T) {
	for _, bad := range []string{`\\server\share\x`, `relative\path`, `C:\`} {
		if _, err := CleanLocalDir(bad); err == nil {
			t.Errorf("CleanLocalDir(%q) should fail", bad)
		}
	}
	if got, err := CleanLocalDir(`C:\Users\me\logs\`); err != nil || got != `C:\Users\me\logs` {
		t.Errorf("CleanLocalDir = %q, %v", got, err)
	}
}

func TestSafeFileNameStripsPathTricks(t *testing.T) {
	for in, want := range map[string]string{"../../evil.exe": "evil.exe", "a b?.zip": "a_b_.zip", "/": "download.bin", "...": "download.bin"} {
		if got := safeFileName(filepathBase(in)); got != want {
			t.Errorf("safeFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func filepathBase(p string) string { return filepath.Base(p) }
