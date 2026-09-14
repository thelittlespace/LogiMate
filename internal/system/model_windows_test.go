//go:build windows

package system

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"
)

func TestIdentifyWheelModel(t *testing.T) {
	cases := []struct{ name, id, want string }{
		{"Logitech G25 Racing Wheel USB", `HID\VID_046D&PID_C294`, "Logitech G25"},
		{"Logitech G27 Racing Wheel USB", `HID\VID_046D&PID_C294`, "Logitech G27"},
		{"USB Input Device", `HID\VID_046D&PID_C299`, "Logitech G25"},
		{"USB Input Device", `HID\VID_046D&PID_C29B`, "Logitech G27"},
		{"USB Input Device", `HID\VID_046D&PID_C29A`, "Logitech Driving Force GT"},
		{"Driving Force", `HID\VID_046D&PID_C294`, "Logitech C294 compatibility mode"},
		{"Driving Force GT", `HID\VID_046D&PID_C294`, "Logitech C294 compatibility mode"},
	}
	for _, c := range cases {
		if got := IdentifyWheelModel(c.name, c.id); got != c.want {
			t.Fatalf("IdentifyWheelModel(%q,%q)=%q want %q", c.name, c.id, got, c.want)
		}
	}
}

func TestIdentifyWheelModelKeepsGenericC294Ambiguous(t *testing.T) {
	cases := []string{
		"Logitech G25/G27 compatibility device",
		"USB Input Device",
		"Driving Force",
		"Driving Force GT",
	}
	for _, name := range cases {
		if got := IdentifyWheelModel(name, `HID\VID_046D&PID_C294`); got != "Logitech C294 compatibility mode" {
			t.Fatalf("IdentifyWheelModel(%q,C294)=%q, want compatibility mode", name, got)
		}
	}
}

func TestRawInputFallbackKeepsC294Ambiguous(t *testing.T) {
	devs := devicesFromRawInput([]string{`\\?\HID#VID_046D&PID_C294#123`})
	if len(devs) != 1 {
		t.Fatalf("devicesFromRawInput len=%d, want 1", len(devs))
	}
	if devs[0].Model != "Logitech C294 compatibility mode" {
		t.Fatalf("raw C294 model=%q", devs[0].Model)
	}
}

func TestRawInputFallbackRecognizesNativePIDs(t *testing.T) {
	devs := devicesFromRawInput([]string{
		`\\?\HID#VID_046D&PID_C299#1`,
		`\\?\HID#VID_046D&PID_C29B#2`,
		`\\?\HID#VID_046D&PID_C29A#3`,
	})
	if len(devs) != 3 {
		t.Fatalf("len=%d, want 3", len(devs))
	}
	models := map[string]bool{}
	for _, d := range devs {
		models[d.Model] = true
	}
	if !models["Logitech G25"] || !models["Logitech G27"] || !models["Logitech Driving Force GT"] {
		t.Fatalf("models=%v", models)
	}
}

func TestResolveWheelModelNativePnPBeatsStoredC294Preference(t *testing.T) {
	devs := []Device{{Name: "USB Input Device", InstanceID: `HID\VID_046D&PID_C29B`, Model: "Logitech G27"}}
	got, _, evidence := resolveWheelModel(devs, []string{`\\?\HID#VID_046D&PID_C299#OTHER`}, "Logitech G25")
	if got != "Logitech G27" {
		t.Fatalf("resolved=%q, want native G27", got)
	}
	if evidence != "SetupAPI/PnP native PID" {
		t.Fatalf("evidence=%q", evidence)
	}
}

func TestResolveWheelModelRawNativeDisambiguatesPnPC294(t *testing.T) {
	devs := []Device{{Name: "USB Input Device", InstanceID: `HID\VID_046D&PID_C294`, Model: "Logitech C294 compatibility mode"}}
	got, _, evidence := resolveWheelModel(devs, []string{`\\?\HID#VID_046D&PID_C29B#1`}, "Logitech G25")
	if got != "Logitech G27" {
		t.Fatalf("resolved=%q, want G27 from Raw Input native PID", got)
	}
	if evidence != "PnP C294 + Raw Input native PID" {
		t.Fatalf("evidence=%q", evidence)
	}
}

func TestResolveWheelModelNoDeviceHasEvidence(t *testing.T) {
	got, devs, evidence := resolveWheelModel(nil, nil, "")
	if got != "Kein kompatibles Lenkrad" || len(devs) != 0 {
		t.Fatalf("got=%q devs=%v", got, devs)
	}
	if evidence == "" {
		t.Fatal("missing no-device detection evidence")
	}
}

func TestSelectWinMMJoystickSingleDrivingForceUsesConfirmedWheelFallback(t *testing.T) {
	devices := []JoyState{{Found: true, ID: 2, Name: "Logitech Driving Force USB"}}
	got := selectWinMMJoystick(devices, "Logitech G27")
	if !got.Found || got.ID != 2 {
		t.Fatalf("selected %+v, want ID 2", got)
	}
	if got.Selection == "WinMM-Gerätename" {
		t.Fatalf("broad Driving Force name must be correlation, not identity: %+v", got)
	}
}

func TestSelectWinMMJoystickDoesNotGuessDrivingForceAmongMultipleControllers(t *testing.T) {
	devices := []JoyState{
		{Found: true, ID: 0, Name: "Logitech Driving Force USB"},
		{Found: true, ID: 1, Name: "Logitech Gamepad F710"},
	}
	got := selectWinMMJoystick(devices, "Logitech G27")
	if got.Found {
		t.Fatalf("unexpected broad-name guess: %+v", got)
	}
}

func TestSummarizeWheelModel(t *testing.T) {
	if got := SummarizeWheelModel([]Device{{Model: "Logitech G25"}}); got != "Logitech G25" {
		t.Fatalf("got %q", got)
	}
	if got := SummarizeWheelModel([]Device{{Model: "Logitech G27"}}); got != "Logitech G27" {
		t.Fatalf("got %q", got)
	}
	if got := SummarizeWheelModel([]Device{{Model: "Logitech C294 compatibility mode"}}); got != "Logitech C294 (Kompatibilitätsmodus)" {
		t.Fatalf("got %q", got)
	}
}

func TestDrivingForceGTNativePIDAndWinMM(t *testing.T) {
	if got := IdentifyWheelModel("Driving Force GT", `HID\VID_046D&PID_C29A`); got != "Logitech Driving Force GT" {
		t.Fatalf("DFGT native model=%q", got)
	}
	devices := []JoyState{{Found: true, ID: 4, Name: "Logitech Driving Force GT USB"}}
	got := selectWinMMJoystick(devices, "Logitech Driving Force GT")
	if !got.Found || got.ID != 4 || got.Selection != "WinMM-Gerätename" {
		t.Fatalf("DFGT WinMM selection=%+v", got)
	}
}

func TestSelectWinMMJoystickPrefersExplicitWheelName(t *testing.T) {
	devices := []JoyState{
		{Found: true, ID: 0, Name: "Logitech Gamepad F710"},
		{Found: true, ID: 1, Name: "Logitech G27 Racing Wheel USB"},
	}
	got := selectWinMMJoystick(devices, "Logitech G27")
	if !got.Found || got.ID != 1 {
		t.Fatalf("selected %+v, want ID 1 G27", got)
	}
	if got.Selection != "WinMM-Gerätename" {
		t.Fatalf("selection = %q", got.Selection)
	}
}

func TestSelectWinMMJoystickSingleGenericUsesPnPFallback(t *testing.T) {
	devices := []JoyState{{Found: true, ID: 3, Name: "USB Input Device"}}
	got := selectWinMMJoystick(devices, "Logitech G25")
	if !got.Found || got.ID != 3 {
		t.Fatalf("selected %+v, want generic ID 3", got)
	}
	if got.Selection == "" {
		t.Fatal("expected fallback selection description")
	}
}

func TestSelectWinMMJoystickDoesNotGuessAmongMultipleGenericControllers(t *testing.T) {
	devices := []JoyState{
		{Found: true, ID: 0, Name: "USB Input Device"},
		{Found: true, ID: 1, Name: "Controller"},
	}
	got := selectWinMMJoystick(devices, "Logitech G27")
	if got.Found {
		t.Fatalf("unexpected guess: %+v", got)
	}
	if len(got.Connected) != 2 {
		t.Fatalf("connected list = %+v", got.Connected)
	}
}

func TestJoyNameScoreRejectsWrongModel(t *testing.T) {
	if got := joyNameScore("Logitech G25 Racing Wheel", "Logitech G27"); got >= 0 {
		t.Fatalf("wrong-model score = %d, want negative", got)
	}
}

func TestWinMMStructSizes(t *testing.T) {
	if got := unsafe.Sizeof(JOYINFOEX{}); got != 52 {
		t.Fatalf("JOYINFOEX size = %d, want 52", got)
	}
	if got := unsafe.Sizeof(JOYCAPSW{}); got != 728 {
		t.Fatalf("JOYCAPSW size = %d, want 728", got)
	}
}

func TestSelectWinMMJoystickDoesNotTreatNamedGamepadAsWheel(t *testing.T) {
	devices := []JoyState{{Found: true, ID: 0, Name: "Logitech Gamepad F710"}}
	got := selectWinMMJoystick(devices, "Logitech G27")
	if got.Found {
		t.Fatalf("named non-wheel controller was selected: %+v", got)
	}
}

func TestOperatingPreferencePersists(t *testing.T) {
	dir := t.TempDir()
	if got := ReadOperatingPreference(dir); got != "" {
		t.Fatalf("initial operating preference=%q", got)
	}
	if err := SaveOperatingPreference(dir, "modern"); err != nil {
		t.Fatal(err)
	}
	if got := ReadOperatingPreference(dir); got != "modern" {
		t.Fatalf("modern preference=%q", got)
	}
	if err := SaveOperatingPreference(dir, "legacy"); err != nil {
		t.Fatal(err)
	}
	if got := ReadOperatingPreference(dir); got != "legacy" {
		t.Fatalf("legacy preference=%q", got)
	}
	if err := SaveOperatingPreference(dir, "unknown"); err == nil {
		t.Fatal("invalid operating preference should fail")
	}
}

func TestValidateBackupHashes(t *testing.T) {
	dir := t.TempDir()
	inf := filepath.Join(dir, "driver.inf")
	if err := os.WriteFile(inf, []byte("driver-v1"), 0644); err != nil {
		t.Fatal(err)
	}
	sum, err := fileSHA256(inf)
	if err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"complete":true,"sha256":{"driver.inf":"%s"}}`, sum)
	if err := os.WriteFile(filepath.Join(dir, "backup-manifest.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateBackupHashes(dir); err != nil {
		t.Fatalf("valid backup rejected: %v", err)
	}
	if err := os.WriteFile(inf, []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateBackupHashes(dir); err == nil {
		t.Fatal("tampered backup was accepted")
	}
}

func TestValidateBackupHashesRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"complete":true,"sha256":{"../outside.inf":"deadbeef"}}`
	if err := os.WriteFile(filepath.Join(dir, "backup-manifest.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateBackupHashes(dir); err == nil {
		t.Fatal("path traversal manifest was accepted")
	}
}

func TestStrongestMovedAxis(t *testing.T) {
	before := JoyState{Y: 65535, YMin: 0, YMax: 65535, Z: 65535, ZMin: 0, ZMax: 65535, R: 65535, RMin: 0, RMax: 65535}
	after := before
	after.Z = 0
	if got := StrongestMovedAxis(before, after, map[string]bool{}); got != "Z" {
		t.Fatalf("StrongestMovedAxis = %q, want Z", got)
	}
	if got := StrongestMovedAxis(before, after, map[string]bool{"Z": true}); got != "" {
		t.Fatalf("excluded axis selected: %q", got)
	}
}

func TestPedalMappingPersistsPerMode(t *testing.T) {
	dir := t.TempDir()
	legacy := PedalMapping{Gas: "Y", Brake: "R", Clutch: "U"}
	generic := PedalMapping{Gas: "Z", Brake: "Y", Clutch: "V"}
	if err := SavePedalMapping(dir, "Logitech G27", "legacy", legacy); err != nil {
		t.Fatal(err)
	}
	if err := SavePedalMapping(dir, "Logitech G27", "generic", generic); err != nil {
		t.Fatal(err)
	}
	if got := ReadPedalMapping(dir, "Logitech G27", "legacy"); got != legacy {
		t.Fatalf("legacy mapping = %+v, want %+v", got, legacy)
	}
	if got := ReadPedalMapping(dir, "Logitech G27", "generic"); got != generic {
		t.Fatalf("generic mapping = %+v, want %+v", got, generic)
	}
}

func TestParseG27NativeReport(t *testing.T) {
	// report-id + native G27 payload. This exercises semantic wheel/shifter
	// decoding as well as steering/pedal values.
	report := []byte{0, 0x88, 0x0B, 0x41, 0x00, 0x80, 0xFF, 0x80, 0x00, 0x00, 0x00, 0x00}
	j, ok := parseG27NativeReport(report)
	if !ok {
		t.Fatal("valid native G27 report rejected")
	}
	if j.X < 32760 || j.X > 32775 {
		t.Fatalf("calibrated steering=%d want center", j.X)
	}
	if j.Y != 0 || j.Z == 0 || j.R != 65535 {
		t.Fatalf("calibrated pedals Y/Z/R=%d/%d/%d", j.Y, j.Z, j.R)
	}
	if !j.NativeControls || j.Gear != 1 || j.DPad != 0 {
		t.Fatalf("native controls not decoded: %+v", j)
	}
	if !j.PaddleLeft || !j.PaddleRight || !j.WheelButtons[0] || !j.WheelButtons[4] || !j.ShifterButtons[0] {
		t.Fatalf("semantic buttons not decoded: %+v", j)
	}
	if j.Selection == "" || !j.Found {
		t.Fatalf("direct HID metadata missing: %+v", j)
	}
}

func TestC294DrivingForceGTFallbackAndNativeOverride(t *testing.T) {
	compat := []Device{{Name: "Driving Force GT", InstanceID: `HID\VID_046D&PID_C294`, Model: modelCompat}}
	got, _, evidence := resolveWheelModel(compat, nil, modelDFGT)
	if got != modelDFGT+" (manuell bestätigt / C294)" {
		t.Fatalf("C294 DFGT fallback=%q", got)
	}
	if evidence != "C294 + gespeicherte Bestätigung" {
		t.Fatalf("evidence=%q", evidence)
	}

	nativeG27 := []Device{{Name: "USB Input Device", InstanceID: `HID\VID_046D&PID_C29B`, Model: modelG27}}
	got, _, _ = resolveWheelModel(nativeG27, nil, modelDFGT)
	if got != modelG27 {
		t.Fatalf("native G27 must override DFGT fallback, got %q", got)
	}
}

func TestOpenG27CalibrationDefaults(t *testing.T) {
	cfg := defaultNativeInputDefaults()
	if got := normalizeNativeSteering(0x8000, cfg.Steering, cfg.Deadzone); got != 0 {
		t.Fatalf("center steering=%f, want 0", got)
	}
	if got := normalizeNativePedal(0xFF, cfg.Throttle); got != 0 {
		t.Fatalf("released throttle=%f, want 0", got)
	}
	if got := normalizeNativePedal(0x00, cfg.Throttle); got != 1 {
		t.Fatalf("pressed throttle=%f, want 1", got)
	}
}

func TestDirectHIDConnectionUsesHandleNotActivity(t *testing.T) {
	if !directHIDHandleConnected(syscall.Handle(123), `\\?\hid#vid_046d&pid_c29b`) {
		t.Fatal("valid Direct-HID handle/path must remain connected independently of input activity")
	}
	if directHIDHandleConnected(syscall.InvalidHandle, `\\?\hid#vid_046d&pid_c29b`) {
		t.Fatal("invalid HID handle must not be connected")
	}
	if directHIDHandleConnected(syscall.Handle(123), "") {
		t.Fatal("empty HID path must not be connected")
	}
}

func TestBuildWheelDevicesMultiC294DoesNotApplyGlobalPreference(t *testing.T) {
	devs := []Device{
		{Name: "USB Input Device", InstanceID: `HID\VID_046D&PID_C294\A`, Model: modelCompat},
		{Name: "USB Input Device", InstanceID: `HID\VID_046D&PID_C294\B`, Model: modelCompat},
	}
	wheels := BuildWheelDevices(devs, modelG27)
	if len(wheels) != 2 {
		t.Fatalf("wheel count=%d, want 2", len(wheels))
	}
	for _, w := range wheels {
		if IsG27Model(w.Model) {
			t.Fatalf("global C294 preference leaked into multi-wheel model: %+v", w)
		}
		if !IsCompatibilityModel(w.Model) {
			t.Fatalf("multi-wheel C294 must remain ambiguous: %+v", w)
		}
	}
}

func TestCanChangeSelectedWheelModeRequiresExactlyOneWheel(t *testing.T) {
	one := WheelDevice{ID: `HID\VID_046D&PID_C29B\A`, Model: modelG27, Supported: true, ModelConfirmed: true, PnPVerified: true}
	s := State{Wheels: []WheelDevice{one}, SelectedWheelID: one.ID}
	if !CanChangeSelectedWheelMode(s) {
		t.Fatal("single selected supported wheel should allow a mode transition")
	}
	two := WheelDevice{ID: `HID\VID_046D&PID_C299\B`, Model: modelG25, Supported: true}
	s.Wheels = append(s.Wheels, two)
	if CanChangeSelectedWheelMode(s) {
		t.Fatal("driver-package transition must stay blocked while multiple wheels are attached")
	}
}

func TestRawPathsForPIDDeduplicates(t *testing.T) {
	paths := []string{
		`\\?\HID#VID_046D&PID_C29B#ONE`,
		`\\?\hid#vid_046d&pid_c29b#one`,
		`\\?\HID#VID_046D&PID_C299#OTHER`,
	}
	got := rawPathsForPID(paths, pidG27)
	if len(got) != 1 {
		t.Fatalf("native G27 paths=%v, want one unique path", got)
	}
}

func TestRawInputFallbackDoesNotCollapseTwoSamePIDWheels(t *testing.T) {
	devs := devicesFromRawInput([]string{
		`\\?\HID#VID_046D&PID_C29B#WHEEL_A#{4d1e55b2-f16f-11cf-88cb-001111000030}`,
		`\\?\HID#VID_046D&PID_C29B#WHEEL_B#{4d1e55b2-f16f-11cf-88cb-001111000030}`,
	})
	if len(devs) != 2 {
		t.Fatalf("same-PID Raw Input fallback collapsed %d devices, want 2: %+v", len(devs), devs)
	}
}

func TestC294ExplicitUnsupportedModelFailsClosed(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"Logitech G29 Racing Wheel", "Logitech G29 (nicht integriert)"},
		{"Logitech Driving Force Pro", "Logitech Driving Force Pro (nicht integriert)"},
	} {
		model := IdentifyWheelModel(tc.name, `HID\VID_046D&PID_C294\X`)
		if model != tc.want {
			t.Fatalf("IdentifyWheelModel(%q)=%q want %q", tc.name, model, tc.want)
		}
		wheels := BuildWheelDevices([]Device{{Name: tc.name, InstanceID: `HID\VID_046D&PID_C294\X`, Model: model}}, "")
		if len(wheels) != 1 || wheels[0].Supported {
			t.Fatalf("explicit unsupported C294 must remain visible but non-actionable: %+v", wheels)
		}
	}
}
