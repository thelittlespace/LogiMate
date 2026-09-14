//go:build windows

package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInputProfileCorruptionBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	path := inputProfilePath(dir)
	original := []byte("{broken-json")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := SaveButtonMapping(dir, "wheel-1", "A", 1); err == nil {
		t.Fatal("corrupt input profile must block write")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("corrupt input profile was overwritten")
	}
}

func TestInputProfileFutureSchemaBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	path := inputProfilePath(dir)
	original := []byte(`{"version":99,"profiles":{}}`)
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := SaveButtonMapping(dir, "wheel-1", "A", 1); err == nil {
		t.Fatal("future input profile schema must block write")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("future input profile was overwritten")
	}
}

func TestNativeEngineProfileCorruptionBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	path := nativeEngineProfilePath(dir)
	original := []byte("{broken-json")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	err := SaveNativeEngineProfile(dir, "wheel-1", NativeEngineProfile{Name: "Mine", RotationDegrees: 900, MasterGainPercent: 75})
	if err == nil {
		t.Fatal("corrupt native engine profile must block write")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatal("corrupt native engine profile was overwritten")
	}
}

func TestNativeEngineProfileFutureSchemaBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "native-wheel-profiles.json")
	original := []byte(`{"version":99,"active":{},"profiles":{}}`)
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	err := SaveNativeEngineProfile(dir, "wheel-1", NativeEngineProfile{Name: "Mine", RotationDegrees: 900, MasterGainPercent: 75})
	if err == nil {
		t.Fatal("future native engine schema must block write")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatal("future native engine profile was overwritten")
	}
}

func TestNativeFFBFutureSchemaBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	path := nativeFFBConfigPath(dir)
	original := []byte(`{"version":99,"wheels":{}}`)
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := SaveNativeFFBConfig(dir, "wheel-1", NativeFFBConfig{MasterGainPercent: 50}); err == nil {
		t.Fatal("future native FFB schema must block write")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("future native FFB config was overwritten")
	}
}

func TestWheelConfirmationCorruptionBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	path := wheelModelConfirmationPath(dir)
	original := []byte("{broken-json")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	w := WheelDevice{ID: "wheel-1", SessionID: "session-1", HardwareFingerprint: "fingerprint-1", PnPVerified: true, PersistentIdentity: true}
	if err := SaveWheelModelConfirmation(dir, w, ModelG27); err == nil {
		t.Fatal("corrupt wheel confirmation file must block write")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("corrupt wheel confirmation file was overwritten")
	}
}

func TestWheelConfirmationFutureSchemaBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	path := wheelModelConfirmationPath(dir)
	original := []byte(`{"version":99,"confirmations":{}}`)
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	w := WheelDevice{ID: "wheel-1", SessionID: "session-1", HardwareFingerprint: "fingerprint-1", PnPVerified: true, PersistentIdentity: true}
	if err := SaveWheelModelConfirmation(dir, w, ModelG27); err == nil {
		t.Fatal("future wheel confirmation schema must block write")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("future wheel confirmation file was overwritten")
	}
}

func TestNativeIdentityCorruptionIsNotOverwrittenByBackgroundLearning(t *testing.T) {
	dir := t.TempDir()
	path := wheelNativeIdentityPath(dir)
	original := []byte("{broken-json")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	rememberAuthoritativeNativeIdentities(dir, []WheelDevice{{ID: "wheel-1", SessionID: "session-1", HardwareFingerprint: "fp", PnPVerified: true, Model: ModelG27, InstanceID: `USB\\VID_046D&PID_C29B\\X`}})
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("corrupt native identity history was overwritten")
	}
}
