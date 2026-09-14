//go:build windows

package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thelittlespace/LogiMate/internal/gameadapter"
)

func TestD3RegistryUsesTypedAdapters(t *testing.T) {
	seen := map[string]TelemetryAdapterInfo{}
	for _, a := range RegisteredTelemetryAdapters() {
		seen[a.ID] = a
	}
	for _, id := range []string{TelemetryAdapterLogiMateJSON, TelemetryAdapterWreckfestPino} {
		a, ok := seen[id]
		if !ok || !a.Implemented {
			t.Fatalf("adapter %s missing: %+v", id, a)
		}
		if !a.ProvidesForce || !a.ProvidesRPM || !a.ProvidesFFBAuth {
			t.Fatalf("adapter %s capabilities incomplete: %+v", id, a)
		}
	}
	if got := canonicalTelemetryAdapterID(legacyTelemetryAdapterOpenG27Pino); got != TelemetryAdapterWreckfestPino {
		t.Fatalf("legacy id=%q", got)
	}
}

func TestD3GameOutputSourceIsAdapterGeneric(t *testing.T) {
	f := TelemetryFrame{Adapter: TelemetryAdapterLogiMateJSON, Game: "Fixture", Force: .4, Physics: true, PlayerControl: true, FFBEnabled: true, ReceivedAt: time.Now()}
	telemetryRuntime.Lock()
	telemetryRuntime.adapter = &c6TestAdapter{snap: gameadapter.Snapshot{
		Descriptor: gameadapter.Descriptor{ID: TelemetryAdapterLogiMateJSON, Implemented: true, Caps: gameadapter.CapabilityTelemetry | gameadapter.CapabilityForce | gameadapter.CapabilityRPM | gameadapter.CapabilityPlayerControl | gameadapter.CapabilityFFBAuthorization},
		Health:     gameadapter.Health{Running: true, Frames: 1, LastFrame: f.ReceivedAt, StartedAt: time.Now().Add(-time.Second)}, LastFrame: f,
	}}
	telemetryRuntime.automatic = false
	telemetryRuntime.Unlock()
	gp := normalizeGameProfile(GameProfile{Name: "Fixture", TelemetryAdapter: TelemetryAdapterLogiMateJSON, GameFFBEnabled: true, GameFFBGainPercent: 100})
	ep := normalizeNativeEngineProfile(NativeEngineProfile{Name: "x", RotationDegrees: 900, MasterGainPercent: 100, ConstantGainPercent: 100, SpringGainPercent: 100, DamperGainPercent: 100, FrictionGainPercent: 100})
	cfg := normalizeNativeFFBConfig(NativeFFBConfig{MasterGainPercent: 100, ConstantLimit: 100, SpringGain: 100, DamperGain: 100, FrictionGain: 100, SlewPerTick: 2, WatchdogMS: 1200})
	src := &fusionC6TelemetrySource{active: true, adapterID: TelemetryAdapterLogiMateJSON, game: gp, engine: ep, cfg: cfg}
	fr, ok := src.TryGetFrame()
	if !ok || fr.Constant <= 0 {
		t.Fatalf("generic adapter frame not consumed: %+v %v", fr, ok)
	}
}

func TestD3FutureGameProfileSchemaIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := gameProfilesPath(dir)
	if err := os.WriteFile(path, []byte(`{"version":999,"profiles":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	err := SaveGameProfile(dir, GameProfile{Name: "x", Executables: []string{"x"}, Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "Schema") {
		t.Fatalf("future schema must block write, err=%v", err)
	}
	b, err2 := os.ReadFile(path)
	if err2 != nil {
		t.Fatal(err2)
	}
	if !strings.Contains(string(b), `"version":999`) {
		t.Fatalf("future file was overwritten: %s", b)
	}
}

func TestD3AdaptersCannotLiveInSystemPackage(t *testing.T) {
	// Architecture guard: production adapter implementations belong to the
	// sibling package, which cannot depend on system without creating an import
	// cycle. This source scan additionally protects against accidental raw HID
	// implementation inside the adapter package.
	root := filepath.Join("..", "gameadapter")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, forbidden := range []string{"nativeHIDTransport", "openNativeHIDTransport", "WriteReport(", "CreateFileW"} {
			if strings.Contains(s, forbidden) {
				t.Fatalf("adapter file %s contains forbidden hardware primitive %q", e.Name(), forbidden)
			}
		}
	}
}
