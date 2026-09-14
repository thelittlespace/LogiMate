//go:build windows

package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditGameProfileRoundTrip(t *testing.T) {
	d := t.TempDir()
	p := GameProfile{Name: "Race", Executables: []string{"Game.exe"}, Enabled: true, AutoApply: true, EngineProfile: "Balanced", MasterGainPercent: 100}
	if e := SaveGameProfile(d, p); e != nil {
		t.Fatal(e)
	}
	got, ok := FindGameProfileByExecutable(d, "game.exe")
	if !ok || got.Name != "Race" {
		t.Fatalf("bad profile: %#v", got)
	}
}
func TestAuditGameProfileRejectsNoExe(t *testing.T) {
	if e := SaveGameProfile(t.TempDir(), GameProfile{Name: "bad"}); e == nil {
		t.Fatal("expected rejection")
	}
}
func TestAuditTelemetrySafety(t *testing.T) {
	f, e := ParseLogiMateJSONTelemetry([]byte(`{"force":4,"physics":true,"playerControl":true}`))
	if e != nil {
		t.Fatal(e)
	}
	if f.Force != 1 || TelemetryForcePercent(f) != 100 {
		t.Fatalf("clamp failed %#v", f)
	}
	f.PlayerControl = false
	if TelemetryForcePercent(f) != 0 {
		t.Fatal("force must be zero without player control")
	}
}
func TestAuditTelemetryRegistry(t *testing.T) {
	a := RegisteredTelemetryAdapters()
	if len(a) < 2 || !a[0].Implemented {
		t.Fatalf("bad registry %#v", a)
	}
}
func TestAuditConfigSnapshotRoundTrip(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "game-profiles.json")
	if e := os.WriteFile(p, []byte("one"), 0644); e != nil {
		t.Fatal(e)
	}
	z, e := CreateConfigSnapshot(d)
	if e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(p, []byte("two"), 0644)
	if e = RestoreConfigSnapshot(d, z); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "one" {
		t.Fatalf("restore=%q", b)
	}
}
func TestAuditSchemaMigration(t *testing.T) {
	d := t.TempDir()
	if e := EnsureDataSchema(d); e != nil {
		t.Fatal(e)
	}
	if e := EnsureDataSchema(d); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(d, "data-schema.json"))
	if e != nil || !strings.Contains(string(b), `"version": 2`) {
		t.Fatalf("schema %v %s", e, b)
	}
}
func TestAuditFFBRequiresLeaseInvariant(t *testing.T) {
	old := NativeFFBSnapshot()
	updateNativeFFBStatus(func(s *NativeFFBStatus) { s.Active = true })
	defer updateNativeFFBStatus(func(s *NativeFFBStatus) { *s = old })
	issues := ValidateStateInvariants(State{})
	found := false
	for _, x := range issues {
		if x.Code == "ffb-without-lease" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing ffb-without-lease invariant")
	}
}
func TestAuditSelfTestsGreen(t *testing.T) {
	for _, r := range RunInternalSelfTests(t.TempDir()) {
		if !r.Passed {
			t.Fatalf("%s: %s", r.Name, r.Detail)
		}
	}
}
func TestAuditStableGateTracksExternalBlockers(t *testing.T) {
	g := EvaluateStableReleaseGate(State{DataDir: t.TempDir()}, false, false, false, false)
	if g.Ready || len(g.Blockers) < 3 {
		t.Fatalf("unexpected gate %#v", g)
	}
}
