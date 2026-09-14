//go:build windows

package system

import (
	"bytes"
	"testing"
)

func TestBuild011SpringUsesDedicatedSlot1(t *testing.T) {
	start, refresh, stop, err := nativeConditionReports(nativeSpringKind, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(start) != 2 {
		t.Fatalf("spring start reports=%d want 2", len(start))
	}
	wantPreamble := []byte{0, 0x0D, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(start[0], wantPreamble) {
		t.Fatalf("fixed-loop preamble=%v", start[0])
	}
	if start[1][1] != 0x21 || start[1][2] != 0x0B {
		t.Fatalf("spring start=%v", start[1])
	}
	if refresh[1] != 0x2C || refresh[2] != 0x0B {
		t.Fatalf("spring refresh=%v", refresh)
	}
	if stop[1] != 0x23 {
		t.Fatalf("spring stop=%v", stop)
	}
}

func TestBuild011DamperUsesDedicatedSlot2(t *testing.T) {
	start, refresh, stop, err := nativeConditionReports(nativeDamperKind, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(start) != 2 || start[1][1] != 0x41 || start[1][2] != 0x0C {
		t.Fatalf("damper start=%v", start)
	}
	if refresh[1] != 0x4C || refresh[2] != 0x0C {
		t.Fatalf("damper refresh=%v", refresh)
	}
	if stop[1] != 0x43 {
		t.Fatalf("damper stop=%v", stop)
	}
}

func TestBuild011FrictionUsesDedicatedSlot3(t *testing.T) {
	start, refresh, stop, err := nativeConditionReports(nativeFrictionKind, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(start) != 2 || start[1][1] != 0x81 || start[1][2] != 0x0E {
		t.Fatalf("friction start=%v", start)
	}
	if refresh[1] != 0x8C || refresh[2] != 0x0E {
		t.Fatalf("friction refresh=%v", refresh)
	}
	if stop[1] != 0x83 {
		t.Fatalf("friction stop=%v", stop)
	}
}

func TestBuild011ConstantUsesSlot0AndThirtyPercentManualCeiling(t *testing.T) {
	start, err := BuildClassicConstantForceSlotReport(0, 30, false)
	if err != nil {
		t.Fatal(err)
	}
	update, err := BuildClassicConstantForceSlotReport(0, 30, true)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := BuildClassicEffectStopReport(0)
	if err != nil {
		t.Fatal(err)
	}
	if start[1] != 0x11 || start[2] != 0x00 {
		t.Fatalf("constant start=%v", start)
	}
	if update[1] != 0x1C || update[2] != 0x00 {
		t.Fatalf("constant update=%v", update)
	}
	if stop[1] != 0x13 {
		t.Fatalf("constant stop=%v", stop)
	}
	if _, err := BuildClassicConstantForceSlotReport(0, 31, false); err == nil {
		t.Fatal("manual constant test accepted >30%")
	}
}

func TestBuild011ConvenienceConditionBuildersUseIndependentSlots(t *testing.T) {
	s, _ := BuildClassicSpringReport(20, false)
	d, _ := BuildClassicDamperReport(20, false)
	f, _ := BuildClassicFrictionReport(20, false)
	if s[1] != 0x21 || d[1] != 0x41 || f[1] != 0x81 {
		t.Fatalf("slots spring=%02x damper=%02x friction=%02x", s[1], d[1], f[1])
	}
}

func TestBuild011AutocenterUsesUsableClipInsteadOfRampByte(t *testing.T) {
	r, err := BuildClassicAutocenterReport(30, 2)
	if err != nil {
		t.Fatal(err)
	}
	if r[1] != 0xFE || r[2] != 0x0D || r[5] != 0x80 {
		t.Fatalf("autocenter report=%v", r)
	}
	if r[3] == 0 || r[4] != r[3] {
		t.Fatalf("autocenter slope=%v", r)
	}
}
