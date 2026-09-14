//go:build windows

package system

import "testing"

func TestClassicRangeReport(t *testing.T) {
	r, err := BuildClassicRangeReport(540)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0xF8, 0x81, 0x1C, 0x02, 0, 0, 0}
	if len(r) != len(want) {
		t.Fatalf("len=%d", len(r))
	}
	for i := range want {
		if r[i] != want[i] {
			t.Fatalf("byte %d=%02X want %02X", i, r[i], want[i])
		}
	}
	if _, err := BuildClassicRangeReport(901); err == nil {
		t.Fatal("range >900 accepted")
	}
}

func TestG27LEDReportBounds(t *testing.T) {
	r, err := BuildG27LEDReport(0x1F)
	if err != nil {
		t.Fatal(err)
	}
	if r[1] != 0xF8 || r[2] != 0x12 || r[3] != 0x1F || r[7] != 0x00 {
		t.Fatalf("unexpected LED report: %v", r)
	}
	if _, err := BuildG27LEDReport(0x20); err == nil {
		t.Fatal("invalid LED mask accepted")
	}
}

func TestAutocenterExperimentalLimits(t *testing.T) {
	if _, err := BuildClassicAutocenterReport(30, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildClassicAutocenterReport(31, 1); err == nil {
		t.Fatal("unsafe autocenter strength accepted")
	}
	if _, err := BuildClassicAutocenterReport(10, 8); err == nil {
		t.Fatal("invalid ramp accepted")
	}
}

func TestAutocenterUsesCanonicalEnableAndOffPackets(t *testing.T) {
	enable := BuildClassicAutocenterEnableReport()
	off := BuildClassicAutocenterOffReport()
	wantEnable := []byte{0, 0x14, 0, 0, 0, 0, 0, 0}
	wantOff := []byte{0, 0xF5, 0, 0, 0, 0, 0, 0}
	if len(enable) != len(wantEnable) || len(off) != len(wantOff) {
		t.Fatalf("unexpected report lengths enable=%v off=%v", enable, off)
	}
	for i := range wantEnable {
		if enable[i] != wantEnable[i] {
			t.Fatalf("enable[%d]=%02X want %02X", i, enable[i], wantEnable[i])
		}
		if off[i] != wantOff[i] {
			t.Fatalf("off[%d]=%02X want %02X", i, off[i], wantOff[i])
		}
	}
}
