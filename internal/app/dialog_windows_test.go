//go:build windows

package app

import (
	"testing"

	"github.com/thelittlespace/LogiMate/internal/system"
)

func TestSplitDialogChoice(t *testing.T) {
	title, desc := splitDialogChoice("Pedale kalibrieren\nAchse, Minimum und Maximum erfassen")
	if title != "Pedale kalibrieren" || desc != "Achse, Minimum und Maximum erfassen" {
		t.Fatalf("unexpected split: %q / %q", title, desc)
	}
}

func TestMigrationProgressStatsUsesTerminalPhases(t *testing.T) {
	m := system.MigrationResult{Action: "setup-modern-g27", Status: "running"}
	m.Steps = []system.MigrationStep{
		{Name: "Zielgerät prüfen", Status: "done"},
		{Name: "LogiMate Native Engine prüfen", Status: "running"},
		{Name: "LogiMate Native Engine prüfen", Status: "done"},
		{Name: "Profiler sichern", Status: "running"},
	}
	done, total, current := migrationProgressStats(m)
	if done != 2 {
		t.Fatalf("expected 2 terminal unique phases, got %d", done)
	}
	if total < done {
		t.Fatalf("invalid total %d < done %d", total, done)
	}
	if current != "Profiler sichern" {
		t.Fatalf("expected current phase Profiler sichern, got %q", current)
	}
}

func TestMigrationCommandViewIncludesActionAndDetails(t *testing.T) {
	m := system.MigrationResult{ID: "abc", Action: "setup-legacy-profiler", Status: "running"}
	m.Steps = []system.MigrationStep{{Name: "Treiber sichern", Status: "done", Detail: "Driver Store exportiert", Timestamp: "2026-09-12T14:00:00+02:00"}}
	text := migrationCommandViewText(m)
	for _, want := range []string{"--admin-action setup-legacy-profiler", "Journal: abc", "Treiber sichern", "Driver Store exportiert"} {
		if !containsText(text, want) {
			t.Fatalf("command view missing %q in %q", want, text)
		}
	}
}

func containsText(s, part string) bool {
	if len(part) == 0 {
		return true
	}
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
