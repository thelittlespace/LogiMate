//go:build windows

package system

import (
	"reflect"
	"testing"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

func d53ActionableCompatG27(pref, mode string) State {
	w := WheelDevice{
		ID: "usbloc:test", SessionID: "container:test", Name: "G27 Racing Wheel",
		Model: modelG27 + " (HID-Produkt bestätigt / C294)", ModelKind: WheelModelG27,
		Mode: "Generic HID / Modern", Supported: true, ModelConfirmed: true, PnPVerified: true,
		InstanceID: `HID\VID_046D&PID_C294\TEST`,
	}
	return State{
		Wheels: []WheelDevice{w}, SelectedWheelID: w.ID, WheelModel: w.Model,
		OperatingPreference: pref, ActiveMode: mode,
		RawInputDevices: []string{`\\?\hid#vid_046d&pid_c294#test#{guid}`},
	}
}

func TestD53FirstRunConfirmedG27AutoPreparesNative(t *testing.T) {
	s := d53ActionableCompatG27("", "Generic HID / Modern")
	if !shouldAutoPrepareNativeWheel(s) {
		t.Fatal("confirmed first-run C294 G27 must auto-prepare native mode")
	}
}

func TestD53ExplicitLegacyOptOutBlocksAutoNative(t *testing.T) {
	s := d53ActionableCompatG27("legacy", "Logitech Legacy")
	if shouldAutoPrepareNativeWheel(s) {
		t.Fatal("explicit Legacy preference must block automatic native switch")
	}
}

func TestD53G27NativeSwitchMatchesOpenG27Exactly(t *testing.T) {
	got := nativeModeReportsForModel(modelG27, 0x04)
	want := [][]byte{wheelengine.WithReportID([]byte{0xF8, 0x09, 0x04, 0x01, 0x00, 0x00, 0x00})}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("G27 native switch=% X want OpenG27-compatible % X", got, want)
	}
}
