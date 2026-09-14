package openg27port

import (
	"bytes"
	"testing"
)

func eqReport(t *testing.T, name string, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("%s=% X want % X", name, got, want)
	}
}

func TestLg4ffOpenG27GoldenVectors(t *testing.T) {
	eqReport(t, "constant center", ConstantForce(0x80), []byte{0x11, 0x08, 0x80, 0x80, 0, 0, 0})
	eqReport(t, "constant left", ConstantForce(0x00), []byte{0x11, 0x08, 0x00, 0x80, 0, 0, 0})
	eqReport(t, "constant right", ConstantForce(0xFF), []byte{0x11, 0x08, 0xFF, 0x80, 0, 0, 0})
	eqReport(t, "stop", Stop(), []byte{0xF3, 0, 0, 0, 0, 0, 0})
	eqReport(t, "range 900", SetRange(900), []byte{0xF8, 0x81, 0x84, 0x03, 0, 0, 0})
	eqReport(t, "range 200", SetRange(200), []byte{0xF8, 0x81, 0xC8, 0x00, 0, 0, 0})
	eqReport(t, "native switch", NativeSwitch(), []byte{0xF8, 0x09, 0x04, 0x01, 0, 0, 0})
	eqReport(t, "spring set", SpringSet(0x07, 0x80), []byte{0xFE, 0x0D, 0x07, 0x07, 0x80, 0, 0})
	eqReport(t, "spring enable", SpringEnable(), []byte{0x14, 0, 0, 0, 0, 0, 0})
	eqReport(t, "spring off", SpringOff(), []byte{0xF5, 0, 0, 0, 0, 0, 0})
	eqReport(t, "damper", Damper(0x07, 0x80), []byte{0x21, 0x0C, 0x07, 0x00, 0x07, 0x00, 0x80})
	eqReport(t, "damper zero", Damper(0, 0xFF), []byte{0x21, 0x0C, 0x00, 0x00, 0x00, 0x00, 0xFF})
	eqReport(t, "damper clamp", Damper(0xFF, 0x80), []byte{0x21, 0x0C, 0x0F, 0x00, 0x0F, 0x00, 0x80})
	eqReport(t, "damper off", DamperOff(), []byte{0x23, 0, 0, 0, 0, 0, 0})
	eqReport(t, "friction", Friction(0x40, 0xFF), []byte{0x21, 0x0E, 0x40, 0x40, 0xFF, 0, 0})
	eqReport(t, "friction zero", Friction(0, 0x80), []byte{0x21, 0x0E, 0x00, 0x00, 0x80, 0, 0})
	eqReport(t, "friction full", Friction(0xFF, 0xFF), []byte{0x21, 0x0E, 0xFF, 0xFF, 0xFF, 0, 0})
	eqReport(t, "friction off", FrictionOff(), []byte{0x23, 0, 0, 0, 0, 0, 0})
	eqReport(t, "led all", SetLeds(0x1F), []byte{0xF8, 0x12, 0x1F, 0, 0, 0, 0})
	eqReport(t, "led off", LedsOff(), []byte{0xF8, 0x12, 0x00, 0, 0, 0, 0})
}

func TestLedBarForPercentOpenG27Thresholds(t *testing.T) {
	vectors := []struct {
		p    int
		want byte
	}{
		{0, 0}, {7, 0}, {8, 1}, {24, 1}, {25, 3}, {49, 3},
		{50, 7}, {74, 7}, {75, 15}, {89, 15}, {90, 31}, {100, 31},
	}
	for _, v := range vectors {
		if got := LedBarForPercent(v.p); got != v.want {
			t.Fatalf("LedBarForPercent(%d)=%05b want %05b", v.p, got, v.want)
		}
	}
}

func TestWithReportIDDoesNotAlias(t *testing.T) {
	cmd := SetRange(900)
	report := WithReportID(cmd)
	eqReport(t, "windows report", report, []byte{0, 0xF8, 0x81, 0x84, 0x03, 0, 0, 0})
	report[1] = 0
	if cmd[0] != 0xF8 {
		t.Fatal("WithReportID aliased command buffer")
	}
}
