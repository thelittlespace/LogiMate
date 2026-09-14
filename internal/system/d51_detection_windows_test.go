//go:build windows

package system

import "testing"

func TestD51RealWindowsInstanceIDsArePnPVerified(t *testing.T) {
	for _, id := range []string{
		`USB\VID_046D&PID_C29B\6&371EE768&0&18`,
		`HID\VID_046D&PID_C29B\7&26EF9B37&2&0000`,
	} {
		if !deviceIsPnPVerified(Device{Status: "OK", InstanceID: id}) {
			t.Fatalf("real Windows instance ID rejected as non-PnP: %q", id)
		}
	}
	if deviceIsPnPVerified(Device{Status: "Synthetic", InstanceID: `HID\VID_046D&PID_C29B\DIRECT`, PhysicalID: "direct:g27"}) {
		t.Fatal("synthetic direct-HID continuity record must not become PnP verified")
	}
}

func TestD51C294G27WindowsConsensusIsSessionActionable(t *testing.T) {
	wheels := []WheelDevice{{
		ID: "usbslot:6&port&0&1", SessionID: "usbslot:6&port&0&1",
		InstanceID: `USB\VID_046D&PID_C294\6&PORT&0&1`,
		Name:       "Logitech G27 Racing Wheel USB", Model: modelG27 + " (Gerätename / C294)",
		ModelKind: WheelModelG27, Supported: true, PnPVerified: true,
	}}
	wheels = applySessionModelConsensus(wheels, modelG27, "SetupAPI/PnP C294 + WinMM Modellkonsens")
	if len(wheels) != 1 || !wheels[0].ModelConfirmed || !IsG27Model(wheels[0].Model) {
		t.Fatalf("G27 C294 consensus did not become session-actionable: %+v", wheels)
	}
	if !HasActionableSelectedWheel(State{Wheels: wheels, SelectedWheelID: wheels[0].ID}) {
		t.Fatal("session-confirmed G27 should be actionable for guarded native restore")
	}
}

func TestD51DFGTC294IsNotAutoAuthorizedFromWinMM(t *testing.T) {
	wheels := []WheelDevice{{
		ID: "usbslot:test", SessionID: "usbslot:test",
		InstanceID: `USB\VID_046D&PID_C294\PORT`,
		Name:       "Driving Force GT", Model: modelCompat,
		ModelKind: WheelModelCompatibility, Supported: true, PnPVerified: true,
	}}
	wheels = applySessionModelConsensus(wheels, modelDFGT, "PnP/Raw C294 + expliziter WinMM-Name")
	if wheels[0].ModelConfirmed {
		t.Fatalf("DFGT C294 must remain manual/native-PID-only: %+v", wheels[0])
	}
}

func TestD51USBAndHIDChildGroupBySameUSBSlot(t *testing.T) {
	usb := Device{InstanceID: `USB\VID_046D&PID_C29B\6&371EE768&0&18`}
	hid := Device{InstanceID: `HID\VID_046D&PID_C29B\7&ABC&0&0000`, ParentID: usb.InstanceID}
	if a, b := deviceSessionGroupID(usb), deviceSessionGroupID(hid); a == "" || a != b {
		t.Fatalf("USB/HID session grouping mismatch: usb=%q hid=%q", a, b)
	}
}

func TestD51MergeDiscoveryPathsDeduplicatesCaseInsensitively(t *testing.T) {
	a := `\\?\HID#VID_046D&PID_C29B#ABC#{GUID}`
	b := `\\?\hid#vid_046d&pid_c29b#abc#{guid}`
	got := mergeHIDPaths([]string{a}, []string{b})
	if len(got) != 1 {
		t.Fatalf("dedupe len=%d want 1: %#v", len(got), got)
	}
}
