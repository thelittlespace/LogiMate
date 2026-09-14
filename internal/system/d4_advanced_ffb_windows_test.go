//go:build windows

package system

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

func TestD4PipelineRunsBeforeHardSafetyCeiling(t *testing.T) {
	adv := AdvancedFFBConfig{Enabled: true, Preset: "test", ConstantGainPercent: 150, TransientGainPercent: 100, DeadbandPercent: 0, MinimumForcePercent: 20, ResponseExponent: .5, OutputLimitPercent: 100}
	p, err := wheelengine.NewFFBPipeline(AdvancedFFBTuning(adv, WheelModelG27))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	shaped := p.ProcessFrame(wheelengine.ForceFrame{Constant: 1}, 1, now, now)
	if shaped <= 0 {
		t.Fatalf("shaped=%f", shaped)
	}
	profile := normalizeNativeEngineProfile(NativeEngineProfile{Name: "x", RotationDegrees: 900, MasterGainPercent: 100, ConstantGainPercent: 100, SpringGainPercent: 100, DamperGainPercent: 100, FrictionGainPercent: 100})
	cfg := normalizeNativeFFBConfig(NativeFFBConfig{MasterGainPercent: 100, ConstantLimit: 8, SpringGain: 20, DamperGain: 20, FrictionGain: 20, SlewPerTick: 2, WatchdogMS: 1200})
	mix := MixNativeEffects(NativeEffectRequest{Constant: int(shaped * 100)}, profile, cfg)
	if mix.Constant > cfg.ConstantLimit || mix.Constant < -cfg.ConstantLimit {
		t.Fatalf("D4 bypassed safety ceiling: %+v cfg=%+v", mix, cfg)
	}
}

func TestD4FutureSchemaIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := advancedFFBPath(dir)
	if err := os.WriteFile(path, []byte(`{"version":999,"wheels":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	err := SaveAdvancedFFBConfig(dir, "wheel", WheelModelG27, defaultAdvancedFFBConfig(WheelModelG27))
	if err == nil || !strings.Contains(err.Error(), "Schema") {
		t.Fatalf("future schema must block write, err=%v", err)
	}
}

func TestD4PresetsRemainWithinNonSafetyBounds(t *testing.T) {
	for _, model := range []WheelModelKind{WheelModelG25, WheelModelG27, WheelModelDFGT} {
		for _, preset := range BuiltinAdvancedFFBPresets(model) {
			tuning := AdvancedFFBTuning(preset.Config, model)
			if _, err := wheelengine.NormalizeFFBTuning(tuning); err != nil {
				t.Fatalf("model=%s preset=%s: %v", model, preset.Name, err)
			}
			if tuning.OutputLimit > 1 {
				t.Fatalf("preset can exceed normalized output: %+v", tuning)
			}
		}
	}
}

func TestD62ZeroGameGainsRoundTripInSchemaV2(t *testing.T) {
	dir := t.TempDir()
	cfg := defaultAdvancedFFBConfig(WheelModelG27)
	cfg.Preset = "Zero gain"
	cfg.ConstantGainPercent = 0
	cfg.TransientGainPercent = 0
	if err := SaveAdvancedFFBConfig(dir, "wheel-zero", WheelModelG27, cfg); err != nil {
		t.Fatal(err)
	}
	got := ReadAdvancedFFBConfig(dir, "wheel-zero", WheelModelG27)
	if got.ConstantGainPercent != 0 || got.TransientGainPercent != 0 {
		t.Fatalf("schema v2 must preserve intentional zero gains: %+v", got)
	}

	var stored advancedFFBFile
	exists, err := ReadJSONConfigStrict(advancedFFBPath(dir), &stored)
	if err != nil || !exists {
		t.Fatalf("stored config missing: exists=%v err=%v", exists, err)
	}
	if stored.Version != advancedFFBSchemaVersion {
		t.Fatalf("stored schema=%d want=%d", stored.Version, advancedFFBSchemaVersion)
	}
}

func TestD62SchemaV1ZeroGameGainsMigrateToHistoricalDefaults(t *testing.T) {
	dir := t.TempDir()
	key := inputProfileKey("wheel-v1")
	legacy := advancedFFBFile{
		Version: 1,
		Wheels: map[string]AdvancedFFBConfig{
			key: {Enabled: true, Preset: "legacy", ConstantGainPercent: 0, TransientGainPercent: 0, ResponseExponent: 1, OutputLimitPercent: 100},
		},
	}
	if err := WriteJSONConfigStrict(advancedFFBPath(dir), legacy, 0644); err != nil {
		t.Fatal(err)
	}
	got := ReadAdvancedFFBConfig(dir, "wheel-v1", WheelModelG27)
	if got.ConstantGainPercent != 100 || got.TransientGainPercent != 100 {
		t.Fatalf("v1 zero represented historical missing/default values, got %+v", got)
	}
}
