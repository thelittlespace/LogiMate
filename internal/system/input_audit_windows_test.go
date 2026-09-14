//go:build windows

package system

import (
	"strings"
	"testing"
)

func TestParseClassicG25DirectHID(t *testing.T) {
	// Windows report-ID byte + 12-byte G25 payload.
	r := make([]byte, 13)
	r[0] = 0
	base := 1
	r[base] = 0x08   // neutral D-pad
	r[base+1] = 0x0B // right+left paddle + wheel button
	r[base+2] = 0x08
	r[base+3], r[base+4] = 0x00, 0x80 // centered steering
	r[base+5] = 0xFF                  // gas released
	r[base+6] = 0x00                  // brake full
	r[base+8] = 80                    // shifter x left column
	r[base+9] = 220                   // upper row => gear 1
	r[base+11] = 0x7F
	j, ok := parseClassicNativeReport(modelG25, r)
	if !ok {
		t.Fatal("G25 report rejected")
	}
	if j.X != 32768 || j.Gear != 1 || !j.PaddleLeft || !j.PaddleRight {
		t.Fatalf("unexpected G25 parse: X=%d gear=%d paddles=%v/%v", j.X, j.Gear, j.PaddleLeft, j.PaddleRight)
	}
	if j.Y != 0 || j.Z != 65535 {
		t.Fatalf("unexpected pedal normalization gas=%d brake=%d", j.Y, j.Z)
	}
	if !j.NativeControls {
		t.Fatal("G25 semantic controls should be available")
	}
}

func TestParseClassicDFGTDirectHID(t *testing.T) {
	r := make([]byte, 12)
	r[0] = 0x08
	r[3], r[4] = 0x34, 0x12
	r[5], r[6] = 0xFF, 0x80
	j, ok := parseClassicNativeReport(modelDFGT, r)
	if !ok {
		t.Fatal("DFGT report rejected")
	}
	if j.X != 0x1234 || j.NumAxes != 3 || j.Name == "" {
		t.Fatalf("unexpected DFGT state: %+v", j)
	}
	if j.Y != 0 {
		t.Fatalf("released throttle should normalize to zero, got %d", j.Y)
	}
}

func TestFirstPressedButtonRequiresSingleNewBit(t *testing.T) {
	before := JoyState{Buttons: 1 << 2}
	after := JoyState{Buttons: (1 << 2) | (1 << 7)}
	bit, ok := FirstPressedButton(before, after)
	if !ok || bit != 7 {
		t.Fatalf("got bit=%d ok=%v", bit, ok)
	}
	after.Buttons |= 1 << 8
	if _, ok := FirstPressedButton(before, after); ok {
		t.Fatal("two simultaneous new buttons must be rejected")
	}
}

func TestSteeringCalibrationHandlesReversedAxis(t *testing.T) {
	c := SteeringCalibration{Left: 60000, Center: 32768, Right: 5000, RangeDegrees: 900}
	if d, ok := steeringDegrees(c, c.Left); !ok || d != -450 {
		t.Fatalf("left=%v ok=%v", d, ok)
	}
	if d, ok := steeringDegrees(c, c.Center); !ok || d != 0 {
		t.Fatalf("center=%v ok=%v", d, ok)
	}
	if d, ok := steeringDegrees(c, c.Right); !ok || d != 450 {
		t.Fatalf("right=%v ok=%v", d, ok)
	}
}

func TestPedalCalibrationInvertDeadzoneCurve(t *testing.T) {
	c := AxisCalibration{Min: 100, Max: 900, Inverted: true, Deadzone: .02, Curve: "linear"}
	if v, ok := calibratedAxisPercent(900, c); !ok || v != 0 {
		t.Fatalf("rest should be 0, got %v", v)
	}
	if v, ok := calibratedAxisPercent(100, c); !ok || v != 1 {
		t.Fatalf("pressed should be 1, got %v", v)
	}
}

func TestControlProfilePersistsButtonGearAndSteering(t *testing.T) {
	dir := t.TempDir()
	id := "usbloc:test-wheel"
	if err := SaveButtonMapping(dir, id, "Paddle L", 5); err != nil {
		t.Fatal(err)
	}
	if err := SaveGearMapping(dir, id, map[string]int{"g27:01:00": 1}); err != nil {
		t.Fatal(err)
	}
	if err := SaveSteeringCalibration(dir, id, SteeringCalibration{Left: 0, Center: 32768, Right: 65535, RangeDegrees: 900}); err != nil {
		t.Fatal(err)
	}
	p := ReadControlProfile(dir, id)
	if p.Buttons["Paddle L"] != 5 || p.Gears["g27:01:00"] != 1 || p.Steering.RangeDegrees != 900 {
		t.Fatalf("profile mismatch: %+v", p)
	}
}

func TestRawReportPrettyContainsByteTable(t *testing.T) {
	j := JoyState{Selection: "LogiMate Direct HID", RawReport: []byte{0, 0xF8, 0x09}}
	s := RawReportPretty(j)
	if !strings.Contains(s, "0xF8") || !strings.Contains(s, "[02]") {
		t.Fatalf("unexpected raw dump: %q", s)
	}
	if RawReportHex(j) != "00F809" {
		t.Fatalf("unexpected raw hex: %s", RawReportHex(j))
	}
}

func TestPedalCalibrationClampsOutsideRangeBeforeInvert(t *testing.T) {
	c := AxisCalibration{Min: 100, Max: 900, Inverted: true, Deadzone: 0, Curve: "linear"}
	if v, ok := calibratedAxisPercent(50, c); !ok || v != 1 {
		t.Fatalf("below-range inverted value should clamp to full press, got %v ok=%v", v, ok)
	}
	if v, ok := calibratedAxisPercent(950, c); !ok || v != 0 {
		t.Fatalf("above-range inverted value should clamp to released, got %v ok=%v", v, ok)
	}
}

func TestLearnedButtonOverridesOnlyMappedControls(t *testing.T) {
	j := JoyState{Buttons: 1 << 5, PaddleRight: true, NativeControls: true}
	p := ControlProfile{Buttons: map[string]uint32{"Paddle L": 5}}
	applyLearnedButtons(p, &j)
	if !j.PaddleLeft {
		t.Fatal("learned Paddle L should follow mapped bit")
	}
	if !j.PaddleRight {
		t.Fatal("unmapped native Paddle R must not be erased by a partial learned profile")
	}
}
