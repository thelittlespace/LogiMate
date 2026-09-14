//go:build windows

package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWheelDevicePreferenceCorruptionBlocksOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wheel.models.json")
	original := []byte(`{"broken":`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if got := ReadWheelDevicePreferences(dir); len(got) != 0 {
		t.Fatalf("corrupt preference file must not produce trusted mappings: %#v", got)
	}
	if err := SaveWheelDevicePreference(dir, "usbslot:test", modelG27); err == nil {
		t.Fatal("corrupt preference file was silently overwritten")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("corrupt source changed: got %q want %q", after, original)
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatalf("corruption marker missing: %v", err)
	}
}
