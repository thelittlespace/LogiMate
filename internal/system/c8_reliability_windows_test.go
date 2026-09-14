//go:build windows

package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC8AxisValuesClampBelowMinimumToZero(t *testing.T) {
	got := AxisValues(JoyState{Y: 99, YMin: 100, YMax: 200})["Y"]
	if got != 0 {
		t.Fatalf("below-minimum axis normalized to %v, want 0", got)
	}
}

func TestC8AmbiguousC294PnPIsNotModelConfirmation(t *testing.T) {
	d := Device{Name: "Logitech G27 Racing Wheel USB", InstanceID: `USB\VID_046D&PID_C294\6&PORT&0&1`, StableID: "usbslot:test", Model: modelG27}
	wheels := BuildWheelDevicesWithPreferences([]Device{d}, "", nil)
	if len(wheels) != 1 {
		t.Fatalf("wheel count=%d, want 1", len(wheels))
	}
	w := wheels[0]
	if !w.PnPVerified {
		t.Fatalf("real SetupAPI/PnP target should be verified: %+v", w)
	}
	if w.ModelConfirmed {
		t.Fatalf("ambiguous C294 friendly name must not authorize a model: %+v", w)
	}
}

func TestC8USBTopologyTokenIsNotPersistentHardwareIdentity(t *testing.T) {
	if got := usbSerialToken(`USB\VID_046D&PID_C294\6&371EE768&0&18`); got != "" {
		t.Fatalf("topology token was accepted as serial: %q", got)
	}
	if got := usbSerialToken(`USB\VID_046D&PID_C294\SERIAL123`); got != "serial123" {
		t.Fatalf("serial token=%q, want serial123", got)
	}
}

func TestC8SessionOnlyC294ConfirmationIsNotPersisted(t *testing.T) {
	InvalidateEphemeralWheelConfirmations()
	defer InvalidateEphemeralWheelConfirmations()
	dir := t.TempDir()
	w := WheelDevice{ID: "usbslot:test", SessionID: "container:test", Model: modelCompat, PnPVerified: true, PersistentIdentity: false}
	if err := SaveWheelModelConfirmation(dir, w, modelG27); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wheelModelConfirmationPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("session-only C294 confirmation must not be persisted, stat err=%v", err)
	}
	prefs := trustedWheelDevicePreferences(dir, []WheelDevice{w})
	if prefs[strings.ToLower(w.ID)] != modelG27 {
		t.Fatalf("current session confirmation missing: %#v", prefs)
	}
	InvalidateEphemeralWheelConfirmations()
	prefs = trustedWheelDevicePreferences(dir, []WheelDevice{w})
	if prefs[strings.ToLower(w.ID)] != "" {
		t.Fatalf("session confirmation survived invalidation: %#v", prefs)
	}
}

func TestC8CorruptConfigIsPreservedAndBlocksOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "native-wheel-profiles.json")
	bad := []byte(`{"version":1,broken`)
	if err := os.WriteFile(path, bad, 0644); err != nil {
		t.Fatal(err)
	}
	var dst map[string]any
	found, err := ReadJSONConfigStrict(path, &dst)
	if !found || err == nil {
		t.Fatalf("found=%v err=%v; corrupt JSON must be surfaced", found, err)
	}
	if err := ConfigWriteAllowed(path); err == nil {
		t.Fatal("corrupt config should block normal overwrite")
	}
	preserved, err := os.ReadFile(path)
	if err != nil || string(preserved) != string(bad) {
		t.Fatalf("original corrupt bytes changed: err=%v got=%q", err, preserved)
	}
}

func TestC8CapabilitiesAreCanonicalForIntegratedModels(t *testing.T) {
	cases := []struct {
		model    string
		pid      string
		selector byte
	}{
		{modelG25, pidG25, 0x02},
		{modelDFGT, pidDFGT, 0x03},
		{modelG27, pidG27, 0x04},
	}
	for _, tc := range cases {
		c, ok := CapabilitiesForWheelModel(tc.model)
		if !ok || c.NativePID != tc.pid || c.NativeModeSelector != tc.selector || !c.HasNativeFFB {
			t.Fatalf("capabilities(%q)=%+v ok=%v", tc.model, c, ok)
		}
	}
	if _, ok := CapabilitiesForWheelModel(modelCompat); ok {
		t.Fatal("ambiguous C294 must not have executable native capabilities")
	}
}

func TestC8RuntimeOutputMarkerCarriesStrongTargetFingerprint(t *testing.T) {
	dir := t.TempDir()
	w := WheelDevice{
		ID: "usbloc:test", SessionID: "container:test", Model: modelG27,
		PnPVerified: true, ModelConfirmed: true,
		HardwareFingerprint: "usbserial:0011223344556677", PersistentIdentity: true,
	}
	if err := MarkRuntimeOutputActiveTarget(dir, w, "test-force"); err != nil {
		t.Fatal(err)
	}
	m, err := readRuntimeOutputRecoveryMarker(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.WheelID != w.ID || m.SessionID != w.SessionID || m.Model != w.Model || m.HardwareFingerprint != w.HardwareFingerprint {
		t.Fatalf("recovery marker lost target identity: %+v", m)
	}
}
