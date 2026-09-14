//go:build windows

package system

import (
	"strings"
	"testing"
)

func TestConditionSlotsAreIndependent(t *testing.T) {
	s, err := BuildClassicSpringSlotReport(1, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	d, err := BuildClassicDamperSlotReport(2, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	f, err := BuildClassicFrictionSlotReport(3, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	if s[1] != 0x21 || d[1] != 0x41 || f[1] != 0x81 {
		t.Fatalf("unexpected start opcodes spring=%02x damper=%02x friction=%02x", s[1], d[1], f[1])
	}
	s, _ = BuildClassicSpringSlotReport(1, 20, true)
	d, _ = BuildClassicDamperSlotReport(2, 20, true)
	f, _ = BuildClassicFrictionSlotReport(3, 20, true)
	if s[1] != 0x2c || d[1] != 0x4c || f[1] != 0x8c {
		t.Fatalf("unexpected update opcodes spring=%02x damper=%02x friction=%02x", s[1], d[1], f[1])
	}
	if _, err := BuildClassicSpringSlotReport(0, 10, false); err == nil {
		t.Fatal("condition accepted reserved constant slot")
	}
}

func TestForceMixerNeverExceedsSafetyConfig(t *testing.T) {
	profile := NativeEngineProfile{Name: "Unsafe", RotationDegrees: 900, MasterGainPercent: 100, ConstantGainPercent: 100, SpringGainPercent: 100, DamperGainPercent: 100, FrictionGainPercent: 100}
	cfg := NativeFFBConfig{MasterGainPercent: 100, ConstantLimit: 8, SpringGain: 12, DamperGain: 13, FrictionGain: 14, SlewPerTick: 2, WatchdogMS: 1200}
	m := MixNativeEffects(NativeEffectRequest{Constant: 100, Spring: 100, Damper: 100, Friction: 100}, profile, cfg)
	if m.Constant != 8 || m.Spring != 12 || m.Damper != 13 || m.Friction != 14 {
		t.Fatalf("mixer escaped safety caps: %+v", m)
	}
	if m.ClipEvents != 4 {
		t.Fatalf("clip events=%d want 4", m.ClipEvents)
	}
}

func TestForceMixerAppliesProfileBeforeSafety(t *testing.T) {
	profile := NativeEngineProfile{Name: "Gentle-ish", RotationDegrees: 900, MasterGainPercent: 50, ConstantGainPercent: 50, SpringGainPercent: 50, DamperGainPercent: 50, FrictionGainPercent: 50}
	cfg := NativeFFBConfig{MasterGainPercent: 100, ConstantLimit: 10, SpringGain: 30, DamperGain: 30, FrictionGain: 30, SlewPerTick: 2, WatchdogMS: 1200}
	m := MixNativeEffects(NativeEffectRequest{Constant: 20, Spring: 20, Damper: 20, Friction: 20}, profile, cfg)
	if m.Constant != 5 || m.Spring != 5 || m.Damper != 5 || m.Friction != 5 {
		t.Fatalf("unexpected profile mix: %+v", m)
	}
	if m.ClipEvents != 0 {
		t.Fatalf("unexpected clipping: %+v", m)
	}
}

func TestNativeEngineProfilesPersistPerWheel(t *testing.T) {
	dir := t.TempDir()
	wheelA := "location:pciroot(0)#usbroot(0)#usb(3)"
	wheelB := "location:pciroot(0)#usbroot(0)#usb(4)"
	custom := NativeEngineProfile{Name: "Custom", RotationDegrees: 540, MasterGainPercent: 80, ConstantGainPercent: 90, SpringGainPercent: 70, DamperGainPercent: 60, FrictionGainPercent: 50}
	if err := SaveNativeEngineProfile(dir, wheelA, custom); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveNativeEngineProfile(dir, wheelA, "Custom"); err != nil {
		t.Fatal(err)
	}
	got := ReadActiveNativeEngineProfile(dir, wheelA)
	if got.Name != "Custom" || got.RotationDegrees != 540 || got.MasterGainPercent != 80 {
		t.Fatalf("wheel A profile=%+v", got)
	}
	other := ReadActiveNativeEngineProfile(dir, wheelB)
	if other.Name != "Balanced" {
		t.Fatalf("wheel B inherited wheel A profile: %+v", other)
	}
}

func TestEngineDryRunPassesWithoutHardware(t *testing.T) {
	out := NativeEngineDryRun(t.TempDir(), "wheel:test")
	if !strings.Contains(out, "PASS") || !strings.Contains(out, "slot3-friction") {
		t.Fatalf("dry run output: %s", out)
	}
}

func TestStaleFFBGenerationCannotOverwriteNewOwner(t *testing.T) {
	nativeFFB.Lock()
	oldStatus, oldGen := nativeFFB.status, nativeFFB.generation
	nativeFFB.generation = 22
	nativeFFB.status = NativeFFBStatus{Generation: 22, Effect: "spring-test"}
	nativeFFB.Unlock()
	defer func() {
		nativeFFB.Lock()
		nativeFFB.status, nativeFFB.generation = oldStatus, oldGen
		nativeFFB.Unlock()
	}()
	if updateNativeFFBStatusGeneration(21, func(st *NativeFFBStatus) { st.Effect = "STALE" }) {
		t.Fatal("stale generation was accepted")
	}
	if got := NativeFFBSnapshot().Effect; got != "spring-test" {
		t.Fatalf("stale update changed status: %q", got)
	}
	if !updateNativeFFBStatusGeneration(22, func(st *NativeFFBStatus) { st.Effect = "damper-test" }) {
		t.Fatal("current generation rejected")
	}
}
