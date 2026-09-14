//go:build windows

package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFusionC5OpenG27ImportTranslatesProfileAndKeepsSafetySeparate(t *testing.T) {
	d := t.TempDir()
	wheel := "usbloc:test-wheel"
	src := []byte(`[{"Name":"Wreckfest 2","ProcessMatch":"Wreckfest2","RotationDeg":900,"AutocenterStrength":0,"DamperStrength":20,"TelemetryFormat":"WreckfestPino","GameFfbEnabled":true,"GameFfbGain":150,"GameFfbInvert":true,"LedsFromTelemetryRpm":true}]`)
	r, err := ImportOpenG27GameProfilesJSON(d, wheel, src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Imported != 1 || r.EngineProfiles != 1 {
		t.Fatalf("bad report: %+v", r)
	}
	p, ok := FindGameProfileByExecutable(d, "Wreckfest2_x64.exe")
	if !ok {
		t.Fatal("OpenG27 substring ProcessMatch was not preserved")
	}
	if p.Source != "openg27" || p.TelemetryAdapter != TelemetryAdapterWreckfestPino || !p.GameFFBEnabled || !p.GameFFBInvert || p.GameFFBGainPercent != 150 || p.LEDPolicy != "telemetry" {
		t.Fatalf("bad translation: %+v", p)
	}
	ep, ok := ReadNativeEngineProfile(d, wheel, p.EngineProfile)
	if !ok || ep.RotationDegrees != 900 || ep.DamperGainPercent != 20 {
		t.Fatalf("engine translation: %+v %v", ep, ok)
	}
	// Import must not create/replace the independent hard safety file.
	if _, err := os.Stat(filepath.Join(d, "native-ffb.json")); !os.IsNotExist(err) {
		t.Fatalf("import touched safety config: %v", err)
	}
}

func TestFusionC5LegacyOpenG27Defaults(t *testing.T) {
	d := t.TempDir()
	r, err := ImportOpenG27GameProfilesJSON(d, "", []byte(`[{"Name":"Old","ProcessMatch":"old","RotationDeg":540,"AutocenterStrength":5,"DamperStrength":10}]`))
	if err != nil || r.Imported != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	p, ok := FindGameProfileByExecutable(d, "old_x64")
	if !ok || p.GameFFBGainPercent != 100 || p.EngineProfile != "Balanced" {
		t.Fatalf("legacy defaults: %+v %v", p, ok)
	}
}
