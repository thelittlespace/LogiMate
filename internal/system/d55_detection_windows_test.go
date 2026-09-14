//go:build windows

package system

import "testing"

func TestD55MergeDeviceEvidenceKeepsHIDProductForSameDevnode(t *testing.T) {
	id := `HID\VID_046D&PID_C294&MI_00\8&TEST&0&0000`
	primary := []Device{{
		Status: "OK", Name: "Logitech Driving Force USB", InstanceID: id,
		ContainerID: "{TEST}", StableID: "usbloc:test", Model: modelCompat,
	}}
	secondary := []Device{{
		Status: "OK", Name: "Logitech Driving Force USB", InstanceID: id,
		ContainerID: "{TEST}", StableID: "usbloc:test", Model: modelG27,
		HIDProduct: "G27 Racing Wheel", HIDSerial: "SERIAL-1",
	}}
	got := mergeDeviceEvidence(primary, secondary)
	if len(got) != 1 {
		t.Fatalf("got %d records, want one enriched devnode", len(got))
	}
	if got[0].HIDProduct != "G27 Racing Wheel" {
		t.Fatalf("HIDProduct=%q, want G27 Racing Wheel", got[0].HIDProduct)
	}
	if got[0].HIDSerial != "SERIAL-1" {
		t.Fatalf("HIDSerial=%q, want preserved HID serial", got[0].HIDSerial)
	}
	if !IsG27Model(got[0].Model) {
		t.Fatalf("Model=%q, want HID-enriched G27 model", got[0].Model)
	}
}

func TestD55MergedHIDProductAutomaticallyConfirmsC294G27(t *testing.T) {
	id := `HID\VID_046D&PID_C294&MI_00\8&TEST&0&0000`
	devices := mergeDeviceEvidence(
		[]Device{{Status: "OK", Name: "Logitech Driving Force USB", InstanceID: id, PhysicalID: "container:{TEST}", ContainerID: "{TEST}", StableID: "usbloc:test", Model: modelCompat}},
		[]Device{{Status: "OK", Name: "Logitech Driving Force USB", InstanceID: id, PhysicalID: "container:{TEST}", ContainerID: "{TEST}", StableID: "usbloc:test", Model: modelG27, HIDProduct: "G27 Racing Wheel"}},
	)
	wheels := []WheelDevice{{
		ID: "usbloc:test", SessionID: "container:{TEST}", Name: "Logitech Driving Force USB",
		Model: modelCompat, ModelKind: WheelModelUnknown, InstanceID: id,
		Supported: true, PnPVerified: true, ModelConfirmed: false,
	}}
	got := applySessionHIDProductConsensus(wheels, devices)
	if len(got) != 1 || !got[0].ModelConfirmed || wheelModelKind(got[0]) != WheelModelG27 {
		t.Fatalf("expected automatic current-session G27 confirmation after fused HID evidence, got %+v", got)
	}
}

func TestD55FusedSnapshotAutoPreparesUnconfirmedC294G27WithoutManualClick(t *testing.T) {
	container := "{11111111-2222-3333-4444-555555555555}"
	stable := "usbloc:test-g27"
	usbID := `USB\VID_046D&PID_C294\8&ABC&0&1`
	hidID := `HID\VID_046D&PID_C294&MI_00\8&DEF&0&0000`
	primary := []Device{
		{Status: "OK", Name: "Logitech Driving Force USB", InstanceID: usbID, ContainerID: container, PhysicalID: "container:" + container, StableID: stable, Model: modelCompat},
		{Status: "OK", Name: "Logitech Driving Force USB", InstanceID: hidID, ContainerID: container, PhysicalID: "container:" + container, StableID: stable, Model: modelCompat},
	}
	secondary := []Device{{
		Status: "OK", Name: "Logitech Driving Force USB", InstanceID: hidID,
		ContainerID: container, PhysicalID: "container:" + container, StableID: stable,
		Model: modelG27, HIDProduct: "G27 Racing Wheel",
	}}
	devices := mergeDeviceEvidence(primary, secondary)
	dir := t.TempDir()
	wheels, selected, model, evidence, mode, _ := reconcileWheelSelection(dir, devices, "", modelCompat, "SetupAPI/PnP C294", "Generic HID / Modern")
	state := State{
		Devices: devices, Wheels: wheels, SelectedWheelID: selected,
		WheelModel: model, DetectionEvidence: evidence, ActiveMode: mode,
		RawInputDevices: []string{`\\?\hid#vid_046d&pid_c294&mi_00#8&def&0&0000#{guid}`},
	}
	if len(wheels) != 1 {
		t.Fatalf("wheels=%d, want exactly one logical G27", len(wheels))
	}
	if !wheels[0].ModelConfirmed || wheelModelKind(wheels[0]) != WheelModelG27 {
		t.Fatalf("wheel not automatically confirmed as G27: %+v", wheels[0])
	}
	if !shouldAutoPrepareNativeWheel(state) {
		t.Fatalf("fused HID evidence should auto-prepare C294 G27 without manual confirmation: model=%q evidence=%q selected=%q wheel=%+v", model, evidence, selected, wheels[0])
	}
}
