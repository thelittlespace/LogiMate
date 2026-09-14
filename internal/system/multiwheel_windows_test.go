//go:build windows

package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testWheel(id, pid, name string) Device {
	return Device{
		Status:     "Present",
		Name:       name,
		Service:    "HID",
		InstanceID: `HID\VID_046D&PID_` + pid + `\` + id,
		Model:      name,
	}
}

func TestReconcileNeverSilentlyReplacesMissingStoredWheel(t *testing.T) {
	dir := t.TempDir()
	missing := testWheel("MISSING", pidG27, "Logitech G27")
	present := testWheel("PRESENT", pidG25, "Logitech G25")
	if err := SaveSelectedWheelID(dir, missing.InstanceID); err != nil {
		t.Fatal(err)
	}
	wheels, selected, model, _, mode, status := reconcileWheelSelection(dir, []Device{present}, "", "Logitech G25", "native", "Generic HID / Modern")
	if len(wheels) != 1 {
		t.Fatalf("wheels=%d, want 1", len(wheels))
	}
	if selected != "" {
		t.Fatalf("selected=%q, must not jump to another physical wheel", selected)
	}
	if mode != "Generic HID / Modern" || model != "Logitech G25" || !strings.Contains(status, "Ziel fehlt") {
		t.Fatalf("unexpected missing-target state: model=%q mode=%q status=%q", model, mode, status)
	}
	if got := ReadSelectedWheelID(dir); !strings.EqualFold(got, missing.InstanceID) {
		t.Fatalf("stored target changed from %q to %q", missing.InstanceID, got)
	}
}

func TestReconcileMultipleWheelsRequiresExplicitChoice(t *testing.T) {
	dir := t.TempDir()
	devs := []Device{
		testWheel("A", pidG27, "Logitech G27"),
		testWheel("B", pidG25, "Logitech G25"),
	}
	wheels, selected, _, _, mode, status := reconcileWheelSelection(dir, devs, "", "", "", "Generic HID / Modern")
	if len(wheels) != 2 || selected != "" {
		t.Fatalf("wheels=%d selected=%q, want two unselected", len(wheels), selected)
	}
	if mode != "Generic HID / Modern" || !strings.Contains(status, "Mehrere Ziele") {
		t.Fatalf("mode=%q status=%q", mode, status)
	}
	if got := ReadSelectedWheelID(dir); got != "" {
		t.Fatalf("selection unexpectedly persisted: %q", got)
	}
}

func TestReconcileAfterResetAutoSelectsOnlyWheel(t *testing.T) {
	dir := t.TempDir()
	dev := testWheel("ONLY", pidG27, "Logitech G27")
	wheels, selected, model, _, _, status := reconcileWheelSelection(dir, []Device{dev}, "", "Logitech G27", "native", "Generic HID / Modern")
	if len(wheels) != 1 || !strings.EqualFold(selected, dev.InstanceID) {
		t.Fatalf("selected=%q wheels=%d", selected, len(wheels))
	}
	if model != "Logitech G27" || !strings.Contains(status, "automatisch ausgewählt") {
		t.Fatalf("model=%q status=%q", model, status)
	}
	if got := ReadSelectedWheelID(dir); !strings.EqualFold(got, dev.InstanceID) {
		t.Fatalf("persisted selection=%q", got)
	}
}

func TestPerDeviceC294PreferencesStayIndependent(t *testing.T) {
	dir := t.TempDir()
	a := testWheel("A", pidCompat, "USB Input Device")
	b := testWheel("B", pidCompat, "USB Input Device")
	if err := SaveWheelDevicePreference(dir, a.InstanceID, modelG27); err != nil {
		t.Fatal(err)
	}
	if err := SaveWheelDevicePreference(dir, b.InstanceID, modelG25); err != nil {
		t.Fatal(err)
	}
	wheels := BuildWheelDevicesWithPreferences([]Device{a, b}, "", ReadWheelDevicePreferences(dir))
	if len(wheels) != 2 {
		t.Fatalf("wheels=%d", len(wheels))
	}
	models := map[string]string{}
	for _, w := range wheels {
		models[strings.ToLower(w.ID)] = w.Model
	}
	if !IsG27Model(models[strings.ToLower(a.InstanceID)]) {
		t.Fatalf("A model=%q", models[strings.ToLower(a.InstanceID)])
	}
	if !IsG25Model(models[strings.ToLower(b.InstanceID)]) {
		t.Fatalf("B model=%q", models[strings.ToLower(b.InstanceID)])
	}
}

func TestSingleC294PerDevicePreferenceBeatsAggregateFallback(t *testing.T) {
	dir := t.TempDir()
	dev := testWheel("A", pidCompat, "USB Input Device")
	if err := SaveWheelDevicePreference(dir, dev.InstanceID, modelG25); err != nil {
		t.Fatal(err)
	}
	_, _, model, evidence, _, _ := reconcileWheelSelection(dir, []Device{dev}, modelG27, modelG27+" (manuell bestätigt / C294)", "global fallback", "Generic HID / Modern")
	if !IsG25Model(model) {
		t.Fatalf("model=%q, per-device G25 preference must win", model)
	}
	if !strings.Contains(evidence, "Gerätebestätigung") {
		t.Fatalf("evidence=%q", evidence)
	}
}

func TestPerDevicePedalMappingsStayIndependent(t *testing.T) {
	dir := t.TempDir()
	model, mode := modelG27, "Generic HID / Modern"
	a := `HID\VID_046D&PID_C29B\A`
	b := `HID\VID_046D&PID_C29B\B`
	ma := PedalMapping{Gas: "Y", Brake: "R", Clutch: "Z"}
	mb := PedalMapping{Gas: "Z", Brake: "Y", Clutch: "R"}
	if err := SavePedalMappingForWheel(dir, a, model, mode, ma); err != nil {
		t.Fatal(err)
	}
	if err := SavePedalMappingForWheel(dir, b, model, mode, mb); err != nil {
		t.Fatal(err)
	}
	if got := ReadPedalMappingForWheel(dir, a, model, mode); got != ma {
		t.Fatalf("A mapping=%+v", got)
	}
	if got := ReadPedalMappingForWheel(dir, b, model, mode); got != mb {
		t.Fatalf("B mapping=%+v", got)
	}
}

func TestPerDevicePedalMappingFallsBackToLegacyMapping(t *testing.T) {
	dir := t.TempDir()
	legacy := PedalMapping{Gas: "Y", Brake: "Z", Clutch: "R"}
	if err := SavePedalMapping(dir, modelG27, "Logitech Legacy", legacy); err != nil {
		t.Fatal(err)
	}
	if got := ReadPedalMappingForWheel(dir, "new-device", modelG27, "Logitech Legacy"); got != legacy {
		t.Fatalf("fallback mapping=%+v", got)
	}
}

func TestClearWheelDetectionStatePreservesOperationalData(t *testing.T) {
	dir := t.TempDir()
	id := `HID\VID_046D&PID_C294\A`
	if err := SaveSelectedWheelID(dir, id); err != nil {
		t.Fatal(err)
	}
	if err := SaveWheelPreference(dir, modelG27); err != nil {
		t.Fatal(err)
	}
	if err := SaveWheelDevicePreference(dir, id, modelG27); err != nil {
		t.Fatal(err)
	}
	if err := SaveOperatingPreference(dir, "modern"); err != nil {
		t.Fatal(err)
	}
	if err := SavePedalMappingForWheel(dir, id, modelG27, "Generic HID / Modern", PedalMapping{Gas: "Y"}); err != nil {
		t.Fatal(err)
	}
	backupMarker := filepath.Join(dir, "DriverBackups", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(backupMarker), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupMarker, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ClearWheelDetectionState(dir); err != nil {
		t.Fatal(err)
	}
	if ReadSelectedWheelID(dir) != "" || ReadWheelPreference(dir) != "" || len(ReadWheelDevicePreferences(dir)) != 0 {
		t.Fatal("detection identity was not fully cleared")
	}
	if got := ReadOperatingPreference(dir); got != "modern" {
		t.Fatalf("operating preference was cleared: %q", got)
	}
	if got := ReadPedalMappingForWheel(dir, id, modelG27, "Generic HID / Modern"); got.Gas != "Y" {
		t.Fatalf("pedal mapping was cleared: %+v", got)
	}
	if _, err := os.Stat(backupMarker); err != nil {
		t.Fatalf("backup marker removed: %v", err)
	}
}

func TestDriverlessG27USBAndHIDNodesCollapseToOnePhysicalWheel(t *testing.T) {
	physical := `USB\VID_046D&PID_C294\6&371EE768&0&18`
	devs := []Device{
		{Status: "OK", Name: "USB-Eingabegerät", Service: "HidUsb", InstanceID: physical, PhysicalID: physical, StableID: DeriveStableWheelID(physical, ""), Model: modelCompat},
		{Status: "OK", Name: "HID-konformer Gamecontroller", Service: "HidUsb", InstanceID: `HID\VID_046D&PID_C294\7&29423A3&3&0000`, ParentID: physical, PhysicalID: physical, StableID: DeriveStableWheelID("", physical), Model: modelCompat},
	}
	wheels := BuildWheelDevices(devs, "")
	if len(wheels) != 1 {
		t.Fatalf("logical wheels=%d, want 1 for USB+HID interfaces of one physical G27: %+v", len(wheels), wheels)
	}
	wantID := "usbslot:6&371ee768&0&18"
	if !strings.EqualFold(wheels[0].ID, wantID) {
		t.Fatalf("stable wheel id=%q want %q", wheels[0].ID, wantID)
	}
	if len(wheels[0].InterfaceIDs) != 2 {
		t.Fatalf("interfaces=%d want 2: %+v", len(wheels[0].InterfaceIDs), wheels[0].InterfaceIDs)
	}
}

func TestTwoPhysicalDriverlessWheelsRemainTwoLogicalWheels(t *testing.T) {
	p1 := `USB\VID_046D&PID_C294\PORT_A`
	p2 := `USB\VID_046D&PID_C294\PORT_B`
	devs := []Device{
		{Status: "OK", Name: "USB Input Device", InstanceID: p1, PhysicalID: p1, Model: modelCompat},
		{Status: "OK", Name: "HID-compliant game controller", InstanceID: `HID\VID_046D&PID_C294\COL_A`, PhysicalID: p1, Model: modelCompat},
		{Status: "OK", Name: "USB Input Device", InstanceID: p2, PhysicalID: p2, Model: modelCompat},
		{Status: "OK", Name: "HID-compliant game controller", InstanceID: `HID\VID_046D&PID_C294\COL_B`, PhysicalID: p2, Model: modelCompat},
	}
	wheels := BuildWheelDevices(devs, "")
	if len(wheels) != 2 {
		t.Fatalf("logical wheels=%d, want 2 physical wheels: %+v", len(wheels), wheels)
	}
}

func TestOldInterfaceSelectionMigratesToPhysicalWheelID(t *testing.T) {
	dir := t.TempDir()
	physical := `USB\VID_046D&PID_C294\PHYSICAL`
	hid := `HID\VID_046D&PID_C294\COLLECTION`
	if err := SaveSelectedWheelID(dir, hid); err != nil {
		t.Fatal(err)
	}
	devs := []Device{
		{Name: "USB Input Device", InstanceID: physical, PhysicalID: physical, StableID: DeriveStableWheelID(physical, ""), Model: modelCompat},
		{Name: "HID-compliant game controller", InstanceID: hid, ParentID: physical, PhysicalID: physical, StableID: DeriveStableWheelID("", physical), Model: modelCompat},
	}
	_, selected, _, _, _, status := reconcileWheelSelection(dir, devs, "", modelCompat, "C294", "Generic HID / Modern")
	want := "usbslot:physical"
	if !strings.EqualFold(selected, want) {
		t.Fatalf("selected=%q want migrated stable id %q", selected, want)
	}
	if got := ReadSelectedWheelID(dir); !strings.EqualFold(got, want) {
		t.Fatalf("persisted selection=%q want %q", got, want)
	}
	if !strings.Contains(status, "migriert") {
		t.Fatalf("status=%q", status)
	}
}

func TestSameWindowsContainerCollapsesEvenWhenPnPParentsDiffer(t *testing.T) {
	container := "d1524e77-b59b-4df0-8aa3-13ab23f5a121"
	devs := []Device{
		{Status: "OK", Name: "USB-Eingabegerät", InstanceID: `USB\VID_046D&PID_C294\6&371EE768&0&18`, PhysicalID: `USB\VID_046D&PID_C294\6&371EE768&0&18`, ContainerID: container, Model: modelCompat},
		{Status: "OK", Name: "HID-konformer Gamecontroller", InstanceID: `HID\VID_046D&PID_C294\7&29423A3&3&0000`, PhysicalID: `HID\VID_046D&PID_C294\7&29423A3&3&0000`, ContainerID: container, Model: modelCompat},
	}
	wheels := BuildWheelDevices(devs, "")
	if len(wheels) != 1 {
		t.Fatalf("same Windows container produced %d wheels, want 1: %+v", len(wheels), wheels)
	}
	if wheels[0].ID != "usbslot:6&371ee768&0&18" {
		t.Fatalf("wheel id=%q want stable USB-slot identity", wheels[0].ID)
	}
	if wheels[0].SessionID != "container:"+container {
		t.Fatalf("session id=%q want current Windows container", wheels[0].SessionID)
	}
	if len(wheels[0].InterfaceIDs) != 2 {
		t.Fatalf("interfaces=%d want 2", len(wheels[0].InterfaceIDs))
	}
}

func TestDifferentWindowsContainersRemainSeparateWheels(t *testing.T) {
	devs := []Device{
		{Name: "G27 A USB", InstanceID: `USB\VID_046D&PID_C29B\A`, ContainerID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Model: modelG27},
		{Name: "G27 A HID", InstanceID: `HID\VID_046D&PID_C29B\A`, ContainerID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Model: modelG27},
		{Name: "G27 B USB", InstanceID: `USB\VID_046D&PID_C29B\B`, ContainerID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Model: modelG27},
		{Name: "G27 B HID", InstanceID: `HID\VID_046D&PID_C29B\B`, ContainerID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Model: modelG27},
	}
	wheels := BuildWheelDevices(devs, "")
	if len(wheels) != 2 {
		t.Fatalf("different Windows containers produced %d wheels, want 2: %+v", len(wheels), wheels)
	}
}

func TestStableIDSurvivesG27CompatibilityToNativeModeSwitch(t *testing.T) {
	compatUSB := `USB\VID_046D&PID_C294\6&371EE768&0&18`
	nativeUSB := `USB\VID_046D&PID_C29B\6&371EE768&0&18`
	if got, want := DeriveStableWheelID(compatUSB, ""), DeriveStableWheelID(nativeUSB, ""); got == "" || got != want {
		t.Fatalf("stable IDs differ across C294->C29B: %q vs %q", got, want)
	}
}

func TestStoredStableIDSurvivesContainerReplacement(t *testing.T) {
	dir := t.TempDir()
	stable := "usbslot:6&371ee768&0&18"
	if err := SaveSelectedWheelID(dir, stable); err != nil {
		t.Fatal(err)
	}
	usb := `USB\VID_046D&PID_C29B\6&371EE768&0&18`
	devs := []Device{
		{Name: "USB-Eingabegerät", InstanceID: usb, StableID: stable, ContainerID: "new-container", Model: modelG27},
		{Name: "HID-konformer Gamecontroller", InstanceID: `HID\VID_046D&PID_C29B\COL`, ParentID: usb, StableID: stable, ContainerID: "new-container", Model: modelG27},
	}
	_, selected, model, _, mode, status := reconcileWheelSelection(dir, devs, "", modelG27, "native", "Generic HID / Modern")
	if selected != stable || model != modelG27 || mode != "Generic HID / Modern" || !strings.Contains(status, "verbunden") {
		t.Fatalf("selected=%q model=%q mode=%q status=%q", selected, model, mode, status)
	}
}

func TestCompatibilityWheelReadableButNotActionable(t *testing.T) {
	w := WheelDevice{ID: "usbslot:a", Model: modelCompat, Supported: true}
	s := State{Wheels: []WheelDevice{w}, SelectedWheelID: w.ID}
	if !HasReadableSelectedWheel(s) {
		t.Fatal("C294 wheel should remain available for generic diagnostics")
	}
	if HasActionableSelectedWheel(s) || CanChangeSelectedWheelMode(s) {
		t.Fatal("ambiguous C294 wheel must not authorize model-specific/destructive actions")
	}
}

func TestOldContainerSelectionRepairsToStableNativeWheel(t *testing.T) {
	dir := t.TempDir()
	oldContainer := "container:11111111-2222-3333-4444-555555555555"
	if err := SaveSelectedWheelID(dir, oldContainer); err != nil {
		t.Fatal(err)
	}
	usb := `USB\VID_046D&PID_C29B\6&371EE768&0&18`
	stable := "usbslot:6&371ee768&0&18"
	devs := []Device{
		{Name: "USB-Eingabegerät", InstanceID: usb, StableID: stable, ContainerID: "494c8178-9250-11f1-8775-6245b4fe1312", Model: modelG27},
		{Name: "HID-konformer Gamecontroller", InstanceID: `HID\VID_046D&PID_C29B\7&26EF9B37&2&0000`, ParentID: usb, StableID: stable, ContainerID: "494c8178-9250-11f1-8775-6245b4fe1312", Model: modelG27},
	}
	wheels, selected, model, evidence, mode, status := reconcileWheelSelection(dir, devs, modelG27, modelG27, "SetupAPI/PnP native PID", "Generic HID / Modern")
	if len(wheels) != 1 || selected != stable {
		t.Fatalf("wheels=%d selected=%q want one stable target %q", len(wheels), selected, stable)
	}
	if model != modelG27 || mode != "Generic HID / Modern" || !strings.Contains(evidence, "native PID") || !strings.Contains(status, "Container-ID") {
		t.Fatalf("model=%q mode=%q evidence=%q status=%q", model, mode, evidence, status)
	}
	if got := ReadSelectedWheelID(dir); got != stable {
		t.Fatalf("stored selection=%q want %q", got, stable)
	}
}

func TestTransientDirectIdentityIsNotPersisted(t *testing.T) {
	dir := t.TempDir()
	devs := []Device{{Name: "Logitech G27 (Direct HID)", InstanceID: `HID\VID_046D&PID_C29B\DIRECT`, PhysicalID: "direct:g27", StableID: "direct:g27", Model: modelG27}}
	_, selected, _, _, _, _ := reconcileWheelSelection(dir, devs, "", modelG27, "Direct HID", "Generic HID / Modern")
	if selected != "direct:g27" {
		t.Fatalf("selected=%q want transient direct:g27", selected)
	}
	if got := ReadSelectedWheelID(dir); got != "" {
		t.Fatalf("transient direct identity must not be persisted, got %q", got)
	}
}

func TestLocationStableIDSurvivesPIDSwitch(t *testing.T) {
	loc := `PCIROOT(0)#PCI(1400)#USBROOT(0)#USB(4)#USB(2)`
	compat := DeriveStableWheelIDWithLocation(`USB\VID_046D&PID_C294\OLD`, "", loc)
	native := DeriveStableWheelIDWithLocation(`USB\VID_046D&PID_C29B\NEW`, "", loc)
	if compat == "" || compat != native || !strings.HasPrefix(compat, "usbloc:") {
		t.Fatalf("location stable IDs differ or invalid: compat=%q native=%q", compat, native)
	}
}

func TestOldUSBSlotSelectionAliasesToNewLocationIdentity(t *testing.T) {
	dir := t.TempDir()
	usb := `USB\VID_046D&PID_C29B\6&371EE768&0&18`
	old := DeriveStableWheelID(usb, "")
	loc := `PCIROOT(0)#PCI(1400)#USBROOT(0)#USB(4)`
	newID := DeriveStableWheelIDWithLocation(usb, "", loc)
	if err := SaveSelectedWheelID(dir, old); err != nil {
		t.Fatal(err)
	}
	devs := []Device{{Name: "USB-Eingabegerät", InstanceID: usb, LocationPath: loc, StableID: newID, ContainerID: "c", Model: modelG27}}
	_, selected, _, _, _, status := reconcileWheelSelection(dir, devs, "", modelG27, "native", "Generic HID / Modern")
	if selected != newID || !strings.Contains(status, "migriert") {
		t.Fatalf("selected=%q new=%q status=%q", selected, newID, status)
	}
}

func TestOldContainerSelectionRepairsToLocationStableNativeWheel(t *testing.T) {
	dir := t.TempDir()
	if err := SaveSelectedWheelID(dir, "container:old-c294-container"); err != nil {
		t.Fatal(err)
	}
	usb := `USB\VID_046D&PID_C29B\6&371EE768&0&18`
	loc := `PCIROOT(0)#PCI(1400)#USBROOT(0)#USB(4)#USB(2)`
	stable := DeriveStableWheelIDWithLocation(usb, "", loc)
	devs := []Device{
		{Name: "USB-Eingabegerät", InstanceID: usb, LocationPath: loc, StableID: stable, ContainerID: "494c8178-9250-11f1-8775-6245b4fe1312", Model: modelG27},
		{Name: "HID-konformer Gamecontroller", InstanceID: `HID\VID_046D&PID_C29B\7&26EF9B37&2&0000`, ParentID: usb, LocationPath: loc + "#HID", StableID: stableWheelLocationID(loc + "#HID"), ContainerID: "494c8178-9250-11f1-8775-6245b4fe1312", Model: modelG27},
	}
	wheels, selected, model, _, mode, status := reconcileWheelSelection(dir, devs, modelG27, modelG27, "SetupAPI/PnP native PID", "Generic HID / Modern")
	if len(wheels) != 1 || selected != stable || model != modelG27 || mode != "Generic HID / Modern" {
		t.Fatalf("wheels=%d selected=%q stable=%q model=%q mode=%q status=%q", len(wheels), selected, stable, model, mode, status)
	}
	if got := ReadSelectedWheelID(dir); got != stable {
		t.Fatalf("stored selection=%q want %q", got, stable)
	}
}

func TestLogicalWheelPrefersUSBLocationIdentityOverHIDCollectionLocation(t *testing.T) {
	usb := `USB\VID_046D&PID_C29B\6&371EE768&0&18`
	usbLoc := `PCIROOT(0)#PCI(1400)#USBROOT(0)#USB(4)`
	hidLoc := usbLoc + `#HID-COLLECTION`
	usbStable := DeriveStableWheelIDWithLocation(usb, "", usbLoc)
	devs := []Device{
		{Name: "HID-konformer Gamecontroller", InstanceID: `HID\VID_046D&PID_C29B\CHILD`, ParentID: usb, LocationPath: hidLoc, StableID: stableWheelLocationID(hidLoc), ContainerID: "same", Model: modelG27},
		{Name: "USB-Eingabegerät", InstanceID: usb, LocationPath: usbLoc, StableID: usbStable, ContainerID: "same", Model: modelG27},
	}
	wheels := BuildWheelDevices(devs, "")
	if len(wheels) != 1 || wheels[0].ID != usbStable {
		t.Fatalf("wheels=%+v want USB stable ID %q", wheels, usbStable)
	}
}

func TestLegacyGlobalC294PreferenceIsNeverAppliedToNewPhysicalWheel(t *testing.T) {
	dir := t.TempDir()
	if err := SaveWheelPreference(dir, modelG27); err != nil {
		t.Fatal(err)
	}
	dev := Device{Name: "USB Input Device", InstanceID: `USB\VID_046D&PID_C294\NEWPORT`, StableID: "usbslot:newport", Model: modelCompat}
	wheels, selected, model, _, _, _ := reconcileWheelSelection(dir, []Device{dev}, ReadWheelPreference(dir), modelCompat, "C294", "Generic HID / Modern")
	if len(wheels) != 1 || selected == "" {
		t.Fatalf("wheel should be readable/autoselected: wheels=%d selected=%q", len(wheels), selected)
	}
	if !IsCompatibilityModel(model) || HasActionableSelectedWheel(State{Wheels: wheels, SelectedWheelID: selected}) {
		t.Fatalf("legacy global preference leaked into new C294 wheel: model=%q wheels=%+v", model, wheels)
	}
	if got := ReadWheelPreference(dir); got != "" {
		t.Fatalf("legacy global preference was not retired: %q", got)
	}
	if len(ReadWheelDevicePreferences(dir)) != 0 {
		t.Fatal("legacy global preference must not be silently copied to a physical C294 wheel")
	}
}

func TestUserReportOldContainerNativeG27RepairsAndRetiresGlobalPreference(t *testing.T) {
	dir := t.TempDir()
	old := "container:old-c294-container"
	if err := SaveSelectedWheelID(dir, old); err != nil {
		t.Fatal(err)
	}
	if err := SaveWheelPreference(dir, modelG27); err != nil {
		t.Fatal(err)
	}
	usb := `USB\VID_046D&PID_C29B\6&371EE768&0&18`
	loc := `PCIROOT(0)#PCI(1400)#USBROOT(0)#USB(4)#USB(2)`
	stable := DeriveStableWheelIDWithLocation(usb, "", loc)
	container := "494c8178-9250-11f1-8775-6245b4fe1312"
	devs := []Device{
		{Name: "USB-Eingabegerät", Service: "HidUsb", INF: "input.inf", InstanceID: usb, LocationPath: loc, StableID: stable, ContainerID: container, Model: modelG27},
		{Name: "HID-konformer Gamecontroller", INF: "input.inf", InstanceID: `HID\VID_046D&PID_C29B\7&26EF9B37&2&0000`, ParentID: usb, LocationPath: loc + "#HID", ContainerID: container, Model: modelG27},
	}
	wheels, selected, model, evidence, mode, status := reconcileWheelSelection(dir, devs, ReadWheelPreference(dir), modelG27, "SetupAPI/PnP native PID", "Generic HID / Modern")
	if len(wheels) != 1 || selected != stable || model != modelG27 || mode != "Generic HID / Modern" {
		t.Fatalf("report migration failed: wheels=%d selected=%q stable=%q model=%q mode=%q status=%q", len(wheels), selected, stable, model, mode, status)
	}
	if !strings.Contains(evidence, "native PID") || !strings.Contains(status, "Container-ID") {
		t.Fatalf("evidence=%q status=%q", evidence, status)
	}
	if got := ReadSelectedWheelID(dir); got != stable {
		t.Fatalf("stored selection=%q want=%q", got, stable)
	}
	if got := ReadWheelPreference(dir); got != "" {
		t.Fatalf("legacy global model was not retired: %q", got)
	}
}

func TestNativeModeSpecsForIntegratedWheels(t *testing.T) {
	cases := []struct {
		model string
		pid   string
		sel   byte
	}{
		{modelG25, pidG25, 0x02},
		{modelDFGT, pidDFGT, 0x03},
		{modelG27, pidG27, 0x04},
	}
	for _, tc := range cases {
		pid, sel, _, ok := nativeModeSpec(tc.model)
		if !ok || pid != tc.pid || sel != tc.sel {
			t.Fatalf("nativeModeSpec(%q)=(%q,%x,%v), want (%q,%x,true)", tc.model, pid, sel, ok, tc.pid, tc.sel)
		}
	}
	if _, _, _, ok := nativeModeSpec(modelCompat); ok {
		t.Fatal("ambiguous C294 must never have a native-mode selector")
	}
}

func TestC294FriendlyNameDoesNotAuthorizeModelSpecificActions(t *testing.T) {
	dev := Device{
		Name:       "Logitech G27 Racing Wheel USB",
		InstanceID: `USB\VID_046D&PID_C294\PORT_FRIENDLY`,
		StableID:   "usbslot:port_friendly",
		Model:      modelG27,
	}
	wheels := BuildWheelDevicesWithPreferences([]Device{dev}, "", nil)
	if len(wheels) != 1 {
		t.Fatalf("len(wheels)=%d want 1", len(wheels))
	}
	w := wheels[0]
	if w.ModelConfirmed || !w.PnPVerified {
		t.Fatalf("friendly-name C294 must be real PnP but remain model-unconfirmed: %+v", w)
	}
	s := State{Wheels: wheels, SelectedWheelID: w.ID}
	if !HasReadableSelectedWheel(s) {
		t.Fatal("friendly-name C294 should remain readable for diagnostics")
	}
	if HasActionableSelectedWheel(s) || CanChangeSelectedWheelMode(s) {
		t.Fatal("friendly-name C294 must not authorize model-specific/destructive actions")
	}
}

func TestNativePIDAndPerWheelConfirmationAuthorizeKnownModel(t *testing.T) {
	native := Device{
		Name:       "USB-Eingabegerät",
		InstanceID: `USB\VID_046D&PID_C29B\PORT_NATIVE`,
		StableID:   "usbslot:port_native",
		Model:      modelG27,
	}
	wheels := BuildWheelDevicesWithPreferences([]Device{native}, "", nil)
	if len(wheels) != 1 || !wheels[0].ModelConfirmed || !wheels[0].PnPVerified {
		t.Fatalf("native G27 should be PnP verified and confirmed: %+v", wheels)
	}
	if !CanChangeSelectedWheelMode(State{Wheels: wheels, SelectedWheelID: wheels[0].ID}) {
		t.Fatal("single native G27 should authorize mode changes")
	}

	compat := Device{
		Name:       "USB-Eingabegerät",
		InstanceID: `USB\VID_046D&PID_C294\PORT_CONFIRMED`,
		StableID:   "usbslot:port_confirmed",
		Model:      modelCompat,
	}
	prefs := map[string]string{"usbslot:port_confirmed": modelG27}
	wheels = BuildWheelDevicesWithPreferences([]Device{compat}, "", prefs)
	if len(wheels) != 1 || !wheels[0].ModelConfirmed || !wheels[0].PnPVerified {
		t.Fatalf("per-wheel C294 confirmation should require real PnP and explicit model confirmation: %+v", wheels)
	}
	if !CanChangeSelectedWheelMode(State{Wheels: wheels, SelectedWheelID: wheels[0].ID}) {
		t.Fatal("single explicitly confirmed C294 G27 should authorize guarded mode changes")
	}
}
