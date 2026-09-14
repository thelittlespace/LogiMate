package gameadapter

import (
	"testing"
	"time"
)

func TestRegistryCanonicalizesLegacyPino(t *testing.T) {
	r := NewRegistry()
	if got := r.CanonicalID(LegacyIDOpenG27Pino); got != IDWreckfestPino {
		t.Fatalf("got %q", got)
	}
	a, err := r.New(LegacyIDOpenG27Pino)
	if err != nil {
		t.Fatal(err)
	}
	if a.Descriptor().ID != IDWreckfestPino {
		t.Fatalf("descriptor=%q", a.Descriptor().ID)
	}
}

func TestLocalJSONParserNormalizes(t *testing.T) {
	f, err := ParseLocalJSON([]byte(`{"game":"Fixture","rpm":-1,"rpmMax":8000,"rpmRedline":7000,"speedKph":12.5,"force":2,"physics":true,"playerControl":true,"ffbEnabled":true}`), time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	if f.RPM != 0 || f.Force != 1 || !f.FFBEnabled || !f.Physics || !f.PlayerControl {
		t.Fatalf("frame=%+v", f)
	}
}

func TestAdaptersExposeSemanticCapabilitiesOnly(t *testing.T) {
	r := NewRegistry()
	for _, d := range r.Descriptors() {
		if !d.Implemented || d.ID == "" || d.DefaultPort <= 0 {
			t.Fatalf("bad descriptor: %+v", d)
		}
		if d.Caps&CapabilityTelemetry == 0 {
			t.Fatalf("%s missing telemetry capability", d.ID)
		}
	}
}
