//go:build windows

package system

import (
	"testing"
	"time"
)

func TestConstantForceReportBoundsAndEncoding(t *testing.T) {
	left, err := BuildClassicConstantForceReport(-10, false)
	if err != nil {
		t.Fatal(err)
	}
	neutral, err := BuildClassicConstantForceReport(0, true)
	if err != nil {
		t.Fatal(err)
	}
	right, err := BuildClassicConstantForceReport(10, true)
	if err != nil {
		t.Fatal(err)
	}
	if left[1] != 0x11 || neutral[1] != 0x1c || right[1] != 0x1c {
		t.Fatalf("bad slot opcodes: %v %v %v", left, neutral, right)
	}
	if !(left[3] < neutral[3] && neutral[3] <= right[3]) {
		t.Fatalf("force translation not monotonic: %d %d %d", left[3], neutral[3], right[3])
	}
	if _, err := BuildClassicConstantForceReport(11, false); err == nil {
		t.Fatal("unsafe positive force accepted")
	}
	if _, err := BuildClassicConstantForceReport(-11, false); err == nil {
		t.Fatal("unsafe negative force accepted")
	}
}

func TestEffectStopSlots(t *testing.T) {
	want := []byte{0x13, 0x23, 0x43, 0x83}
	for slot, cmd := range want {
		r, err := BuildClassicEffectStopReport(slot)
		if err != nil {
			t.Fatal(err)
		}
		if r[1] != cmd {
			t.Fatalf("slot %d cmd=%02x want=%02x", slot, r[1], cmd)
		}
	}
	if _, err := BuildClassicEffectStopReport(4); err == nil {
		t.Fatal("invalid slot accepted")
	}
}

func TestConditionBuildersAreBounded(t *testing.T) {
	spring, err := BuildClassicSpringReport(20, false)
	if err != nil {
		t.Fatal(err)
	}
	damper, err := BuildClassicDamperReport(20, false)
	if err != nil {
		t.Fatal(err)
	}
	friction, err := BuildClassicFrictionReport(20, false)
	if err != nil {
		t.Fatal(err)
	}
	if spring[1] != 0x21 || spring[2] != 0x0b {
		t.Fatalf("spring=%v", spring)
	}
	if damper[1] != 0x41 || damper[2] != 0x0c {
		t.Fatalf("damper=%v", damper)
	}
	if friction[1] != 0x81 || friction[2] != 0x0e {
		t.Fatalf("friction=%v", friction)
	}
	if _, err := BuildClassicSpringReport(31, false); err == nil {
		t.Fatal("unsafe spring accepted")
	}
	if _, err := BuildClassicDamperReport(31, false); err == nil {
		t.Fatal("unsafe damper accepted")
	}
	if _, err := BuildClassicFrictionReport(31, false); err == nil {
		t.Fatal("unsafe friction accepted")
	}
}

func TestFFBSlewNeverOvershoots(t *testing.T) {
	v, limited := applyFFBSlew(0, 10, 2)
	if v != 2 || !limited {
		t.Fatalf("v=%d limited=%v", v, limited)
	}
	v, limited = applyFFBSlew(9, 10, 2)
	if v != 10 || limited {
		t.Fatalf("v=%d limited=%v", v, limited)
	}
	v, limited = applyFFBSlew(2, -10, 3)
	if v != -1 || !limited {
		t.Fatalf("v=%d limited=%v", v, limited)
	}
}

func TestFFBConfigHardSafetyCaps(t *testing.T) {
	c := normalizeNativeFFBConfig(NativeFFBConfig{MasterGainPercent: 500, ConstantLimit: 99, SpringGain: 80, DamperGain: 80, FrictionGain: 80, SlewPerTick: 50, WatchdogMS: 9999})
	if c.MasterGainPercent != 100 || c.ConstantLimit != 10 || c.SpringGain != 30 || c.DamperGain != 30 || c.FrictionGain != 30 || c.SlewPerTick != 5 || c.WatchdogMS != 2000 {
		t.Fatalf("unsafe config survived normalization: %+v", c)
	}
}

func TestHeartbeatInactiveIsHealthy(t *testing.T) {
	nativeFFB.Lock()
	old := nativeFFB.status
	nativeFFB.status = NativeFFBStatus{}
	nativeFFB.Unlock()
	defer func() { nativeFFB.Lock(); nativeFFB.status = old; nativeFFB.Unlock() }()
	if !NativeFFBHeartbeatHealthy(time.Now()) {
		t.Fatal("inactive engine reported unhealthy")
	}
}

func TestD62NativeConstantLiveTestUsesCanonicalLg4ffPacket(t *testing.T) {
	r, err := BuildNativeConstantForceReport(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0x11, 0x08, 0x80, 0x80, 0, 0, 0}
	if len(r) != len(want) {
		t.Fatalf("len=%d want=%d", len(r), len(want))
	}
	for i := range want {
		if r[i] != want[i] {
			t.Fatalf("byte %d=%02X want %02X", i, r[i], want[i])
		}
	}
}
