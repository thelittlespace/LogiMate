//go:build windows

package system

import (
	"math"
	"strings"
	"testing"
)

func TestFusionC3SlewPreservesTwentyMsRate(t *testing.T) {
	cfg := NativeFFBConfig{MasterGainPercent: 100, ConstantLimit: 10, SpringGain: 20, DamperGain: 20, FrictionGain: 20, SlewPerTick: 2, WatchdogMS: 1200}
	got := fusionC3SlewPerTick(cfg)
	// 2% per 20ms -> 0.6% per 6ms -> normalized 0.006.
	if math.Abs(got-0.006) > 0.000001 {
		t.Fatalf("got %.9f want 0.006", got)
	}
}

func TestFusionC3DryRunPasses(t *testing.T) {
	s, err := FusionC3DryRun()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "PASS") || !strings.Contains(s, "watchdogTrips=1") {
		t.Fatalf("unexpected summary: %s", s)
	}
}
