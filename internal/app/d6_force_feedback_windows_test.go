//go:build windows

package app

import (
	"testing"

	"github.com/thelittlespace/LogiMate/internal/system"
)

func d62TestDraft() d6FFBDraft {
	return d6FFBDraft{
		valid:        true,
		wheelID:      "test-wheel",
		profile:      system.NativeEngineProfile{Name: "Balanced", RotationDegrees: 900, MasterGainPercent: 75, ConstantGainPercent: 75, SpringGainPercent: 65, DamperGainPercent: 55, FrictionGainPercent: 50},
		advanced:     system.AdvancedFFBConfig{Enabled: true, Preset: "Neutral", ConstantGainPercent: 100, TransientGainPercent: 100, ResponseExponent: 1, OutputLimitPercent: 100},
		testStrength: system.NativeManualFFBTestDefaultPercent,
	}
}

func TestD62WheelSliderOnlyMarksWheelProfileCustom(t *testing.T) {
	d6Draft = d62TestDraft()
	d6SetSliderValue(d6Master, 88)
	if d6Draft.profile.MasterGainPercent != 88 || d6Draft.profile.Name != "Custom" {
		t.Fatalf("wheel tuning did not update owned profile: %+v", d6Draft.profile)
	}
	if d6Draft.advanced.Preset != "Neutral" {
		t.Fatalf("wheel slider must not dirty D4 preset: %+v", d6Draft.advanced)
	}
}

func TestD62SignalSliderOnlyMarksSignalPresetCustom(t *testing.T) {
	d6Draft = d62TestDraft()
	d6SetSliderValue(d6GameConstant, 0)
	d6SetSliderValue(d6MinimumForce, 6)
	d6SetSliderValue(d6Response, 40) // .90
	if d6Draft.advanced.ConstantGainPercent != 0 || d6Draft.advanced.MinimumForcePercent != 6 || d6Draft.advanced.ResponseExponent < .899 || d6Draft.advanced.ResponseExponent > .901 {
		t.Fatalf("signal slider mapping wrong: %+v", d6Draft.advanced)
	}
	if d6Draft.advanced.Preset != "Custom" {
		t.Fatalf("manual signal tuning must switch D4 preset to Custom")
	}
	if d6Draft.profile.Name != "Balanced" {
		t.Fatalf("signal slider must not dirty wheel profile: %+v", d6Draft.profile)
	}
}

func TestD62RotationMappingIsDiscrete(t *testing.T) {
	d6Draft = d62TestDraft()
	d6SetSliderValue(d6Rotation, 2)
	if d6Draft.profile.RotationDegrees != 540 {
		t.Fatalf("rotation mapping wrong: %+v", d6Draft.profile)
	}
}

func TestD62TransientIsNotPresentedAsActiveWithoutAdapterChannel(t *testing.T) {
	if d62SliderEnabled(d6Transient) {
		t.Fatal("transient slider must remain disabled until an adapter publishes a transient channel")
	}
	if d62SliderBadge(d6Transient) != "NICHT AKTIV" {
		t.Fatalf("unexpected transient badge: %q", d62SliderBadge(d6Transient))
	}
}

func TestD62FocusOrderIsViewSpecificAndSkipsUnavailableTransient(t *testing.T) {
	oldPage, oldTab, oldView := currentPage, wheelSubtab, d62ActiveView
	defer func() { currentPage, wheelSubtab, d62ActiveView = oldPage, oldTab, oldView }()
	currentPage, wheelSubtab = pageWheel, wheelSubtabFFB

	d62ActiveView = d62ViewBasis
	basis := d6FocusOrder()
	if len(basis) != 2+2+d62ViewCount+3+2 {
		t.Fatalf("basis focus order unexpected: %d / %v", len(basis), basis)
	}
	if basis[0] != d6FocusTabLive || basis[1] != d6FocusTabFFB {
		t.Fatalf("wheel tabs must lead focus order: %v", basis[:2])
	}

	d62ActiveView = d62ViewSignal
	signal := d6FocusOrder()
	for _, f := range signal {
		if f == d6FocusSliderBase+int(d6Transient) {
			t.Fatal("disabled transient slider must not be keyboard-focusable")
		}
	}
	wantGameConstant := d6FocusSliderBase + int(d6GameConstant)
	found := false
	for _, f := range signal {
		if f == wantGameConstant {
			found = true
		}
	}
	if !found {
		t.Fatal("game constant gain missing from signal focus order")
	}
}

func TestBuild009LiveTestButtonsHaveIndependentHitTargets(t *testing.T) {
	oldPage, oldTab := currentPage, wheelSubtab
	oldRects := d6TestRects
	defer func() { currentPage, wheelSubtab, d6TestRects = oldPage, oldTab, oldRects }()
	currentPage, wheelSubtab = pageWheel, wheelSubtabFFB
	for i := 0; i < 6; i++ {
		d6TestRects[i] = RECT{int32(10 + i*50), 100, int32(50 + i*50), 135}
	}
	for i, r := range d6TestRects {
		kind, idx := d6FFBHitTest((r.Left+r.Right)/2, (r.Top+r.Bottom)/2)
		if kind != "test" || idx != i {
			t.Fatalf("button %d hit=%q/%d", i, kind, idx)
		}
	}
}

func TestBuild011TestStrengthUsesDedicatedThirtyPercentRange(t *testing.T) {
	d6Draft = d62TestDraft()
	_, minV, maxV, value, _ := d6SliderMeta(d6TestStrength)
	if minV != 1 || maxV != system.NativeManualFFBTestMaxPercent || value != system.NativeManualFFBTestDefaultPercent {
		t.Fatalf("test strength meta min=%d max=%d value=%d", minV, maxV, value)
	}
	d6SetSliderValue(d6TestStrength, 30)
	if d6Draft.testStrength != 30 {
		t.Fatalf("test strength=%d", d6Draft.testStrength)
	}
}
