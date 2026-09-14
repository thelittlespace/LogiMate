//go:build windows

package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFileReplacesWholeFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	if err := AtomicWriteFile(p, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(p, []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "second" {
		t.Fatalf("got %q", string(b))
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file remained after replacement: %v", err)
	}
}

func TestMigrationJournalSuccessRoundTrip(t *testing.T) {
	dir := t.TempDir()
	id, err := BeginMigration(dir, "setup-modern-g27")
	if err != nil {
		t.Fatal(err)
	}
	AppendMigrationStep(dir, id, "Treiber sichern", "done", "ok")
	CompleteMigration(dir, id, true, "fertig", nil)
	m, err := LoadMigration(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Completed || !m.Success || m.Status != "success" || m.Message != "fertig" {
		t.Fatalf("unexpected migration result: %+v", m)
	}
	if len(m.Steps) != 1 || m.Steps[0].Name != "Treiber sichern" {
		t.Fatalf("unexpected steps: %+v", m.Steps)
	}
}

func TestProfilerBackupHashValidationDetectsTamper(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "files", "settings.dat")
	if err := os.MkdirAll(filepath.Dir(payload), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(payload, []byte("safe"), 0644); err != nil {
		t.Fatal(err)
	}
	sum, err := fileSHA256(payload)
	if err != nil {
		t.Fatal(err)
	}
	m := profilerBackupManifest{Complete: true, SHA256: map[string]string{"files/settings.dat": sum}}
	if err := validateProfilerBackup(dir, m); err != nil {
		t.Fatalf("valid backup rejected: %v", err)
	}
	if err := os.WriteFile(payload, []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateProfilerBackup(dir, m); err == nil {
		t.Fatal("tampered profiler backup was accepted")
	}
}
