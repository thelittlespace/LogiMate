//go:build windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFutureUISettingsSchemaBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	oldPath, oldBlock := uiPrefsPath, uiSettingsWriteBlocked
	defer func() {
		uiPrefsPath = oldPath
		uiSettingsWriteBlocked = oldBlock
	}()

	uiPrefsPath = filepath.Join(dir, "settings.json")
	original := []byte(`{"schemaVersion":99,"themeMode":"dark"}`)
	if err := os.WriteFile(uiPrefsPath, original, 0644); err != nil {
		t.Fatal(err)
	}
	uiSettingsWriteBlocked = "settings.json verwendet Schema 99"
	if err := saveUISettings(defaultUISettings()); err == nil || !strings.Contains(err.Error(), "schreibgeschützt") {
		t.Fatalf("future settings schema must block write, got %v", err)
	}
	got, err := os.ReadFile(uiPrefsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("future settings file was overwritten")
	}
}
