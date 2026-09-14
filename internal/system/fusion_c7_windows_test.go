//go:build windows

package system

import (
	"strings"
	"testing"
)

func TestFusionC7CoreGatePassesWithoutHardware(t *testing.T) {
	if err := FusionC7CoreGate(); err != nil {
		t.Fatal(err)
	}
}

func TestFusionC7ReportDoesNotRequireExternalOpenG27(t *testing.T) {
	w := WheelDevice{ID: "usbloc:test", Name: "Logitech G27", Model: "Logitech G27", Mode: "Generic HID", Supported: true, ModelConfirmed: true, PnPVerified: true}
	s := State{Wheels: []WheelDevice{w}, SelectedWheelID: w.ID, WheelModel: w.Model, ActiveMode: "Generic HID / Modern"}
	r := BuildFusionC7ParityReport(s)
	for _, c := range r.Checks {
		if c.Name == "Eigenständige Runtime" && c.Status != "PASS" {
			t.Fatalf("standalone runtime gate failed: %+v", c)
		}
	}
	if !strings.Contains(FusionC7ParitySummary(s), "Eigenständige Runtime") {
		t.Fatal("summary missing standalone runtime state")
	}
}

func TestFusionC7BlocksNonG27Cutover(t *testing.T) {
	w := WheelDevice{ID: "usbloc:g25", Name: "Logitech G25", Model: "Logitech G25", Mode: "Generic HID", Supported: true, ModelConfirmed: true, PnPVerified: true}
	s := State{Wheels: []WheelDevice{w}, SelectedWheelID: w.ID, WheelModel: w.Model, ActiveMode: "Generic HID / Modern"}
	r := BuildFusionC7ParityReport(s)
	if r.Ready || r.Block == 0 {
		t.Fatalf("G25 must not be promoted by G27-only C7 cutover: %+v", r)
	}
}

func TestFusionC7CurrentReadinessBlocksLegacyMode(t *testing.T) {
	w := WheelDevice{ID: "usbloc:g27", Name: "Logitech G27", Model: "Logitech G27", Mode: "Logitech Legacy", Supported: true, ModelConfirmed: true, PnPVerified: true}
	s := State{Wheels: []WheelDevice{w}, SelectedWheelID: w.ID, WheelModel: w.Model, ActiveMode: "Logitech Legacy"}
	r := BuildFusionC7ParityReport(s)
	if r.Ready || r.Block == 0 {
		t.Fatalf("legacy mode must not report current Native readiness: %+v", r)
	}
	if err := FusionC7CoreGate(); err != nil {
		t.Fatalf("hardware-free pre-migration gate must remain usable in Legacy: %v", err)
	}
}

func TestFusionC7ReflectsCurrentLegacyBuilderParity(t *testing.T) {
	w := WheelDevice{ID: "usbloc:g27", Name: "Logitech G27", Model: "Logitech G27", Mode: "Generic HID", Supported: true, ModelConfirmed: true, PnPVerified: true}
	s := State{Wheels: []WheelDevice{w}, SelectedWheelID: w.ID, WheelModel: w.Model, ActiveMode: "Generic HID / Modern"}
	r := BuildFusionC7ParityReport(s)
	found := false
	for _, c := range r.Checks {
		if c.Name == "C2 Legacy Builder Shadow" {
			found = true
			if c.Status != "PASS" {
				t.Fatalf("current C2 shadow should be fully byte-aligned, got %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("C7 report missing legacy-builder shadow status")
	}
}
