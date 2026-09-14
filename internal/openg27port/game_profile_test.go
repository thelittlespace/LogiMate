package openg27port

import "testing"

func TestOpenG27GameProfileMatchParity(t *testing.T) {
	wf := OpenG27GameProfile{Name: "Wreckfest 2", ProcessMatch: "Wreckfest2", RotationDeg: 540}
	ac := OpenG27GameProfile{Name: "Assetto Corsa", ProcessMatch: "acs", RotationDeg: 900}
	p, ok := MatchOpenG27GameProfile([]string{"acs", "WRECKFEST2_x64"}, []OpenG27GameProfile{wf, ac})
	if !ok || p.Name != "Wreckfest 2" {
		t.Fatalf("first profile/order/case/substring parity failed: %+v %v", p, ok)
	}
	if _, ok := MatchOpenG27GameProfile([]string{"explorer"}, []OpenG27GameProfile{wf}); ok {
		t.Fatal("unexpected match")
	}
	if _, ok := MatchOpenG27GameProfile([]string{"anything"}, []OpenG27GameProfile{{Name: "blank"}}); ok {
		t.Fatal("blank ProcessMatch must not match")
	}
}

func TestParseOpenG27GameProfilesLegacyDefaults(t *testing.T) {
	ps, err := ParseOpenG27GameProfiles([]byte(`[{"Name":"Old","ProcessMatch":"old","RotationDeg":540,"AutocenterStrength":1,"DamperStrength":2}]`))
	if err != nil || len(ps) != 1 {
		t.Fatalf("parse: %v %#v", err, ps)
	}
	if ps[0].GameFfbGain != 100 {
		t.Fatalf("C# default GameFfbGain parity: %d", ps[0].GameFfbGain)
	}
}

func TestWreckfestPresetParity(t *testing.T) {
	p := OpenG27Wreckfest2Preset()
	if p.RotationDeg != 900 || p.TelemetryFormat != "WreckfestPino" || !p.GameFfbEnabled || !p.GameFfbInvert || !p.LedsFromTelemetryRpm {
		t.Fatalf("preset mismatch: %+v", p)
	}
}
