//go:build windows

package system

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/thelittlespace/LogiMate/internal/gameadapter"
)

type c6TestAdapter struct{ snap gameadapter.Snapshot }

func (a *c6TestAdapter) Descriptor() gameadapter.Descriptor   { return a.snap.Descriptor }
func (a *c6TestAdapter) Start(gameadapter.StartOptions) error { return nil }
func (a *c6TestAdapter) Stop() error                          { return nil }
func (a *c6TestAdapter) Snapshot() gameadapter.Snapshot       { return a.snap }
func (a *c6TestAdapter) RecentFrames() []gameadapter.Frame {
	return []gameadapter.Frame{a.snap.LastFrame}
}

func installC6TestTelemetry(f TelemetryFrame) {
	telemetryRuntime.Lock()
	telemetryRuntime.adapter = &c6TestAdapter{snap: gameadapter.Snapshot{
		Descriptor: gameadapter.Descriptor{ID: TelemetryAdapterWreckfestPino, Implemented: true, Automatic: true, Caps: gameadapter.CapabilityTelemetry | gameadapter.CapabilityRPM | gameadapter.CapabilityForce | gameadapter.CapabilityPlayerControl | gameadapter.CapabilityFFBAuthorization},
		Health:     gameadapter.Health{Running: true, Frames: 1, LastFrame: f.ReceivedAt},
		LastFrame:  f,
	}}
	telemetryRuntime.automatic = false
	telemetryRuntime.Unlock()
}

func makePinoTestPacket(force float32, flags uint16, rpm, redline int) []byte {
	b := make([]byte, 1218)
	binary.LittleEndian.PutUint32(b[0:4], 1869769584)
	b[4] = 0
	binary.LittleEndian.PutUint32(b[435:439], uint32(rpm))
	binary.LittleEndian.PutUint32(b[439:443], uint32(5600))
	binary.LittleEndian.PutUint32(b[443:447], uint32(redline))
	binary.LittleEndian.PutUint32(b[447:451], uint32(800))
	binary.LittleEndian.PutUint16(b[1090:1092], flags)
	binary.LittleEndian.PutUint32(b[1093:1097], math.Float32bits(force))
	return b
}

func TestFusionC6PinoTranslation(t *testing.T) {
	f, err := ParseOpenG27PinoTelemetry(makePinoTestPacket(-0.25, (1<<2)|(1<<3), 5000, 5320))
	if err != nil {
		t.Fatal(err)
	}
	if f.Adapter != TelemetryAdapterWreckfestPino || f.RPM != 5000 || f.RPMRedline != 5320 || !f.Physics || !f.PlayerControl || math.Abs(f.Force+0.25) > 1e-6 {
		t.Fatalf("bad frame: %+v", f)
	}
}

func TestFusionC6SourceHonorsInvertGainAndSafety(t *testing.T) {
	f := TelemetryFrame{Adapter: TelemetryAdapterWreckfestPino, Force: 0.5, Physics: true, PlayerControl: true, FFBEnabled: true, ReceivedAt: time.Now()}
	installC6TestTelemetry(f)
	gp := normalizeGameProfile(GameProfile{Name: "WF2", Source: "openg27", GameFFBEnabled: true, GameFFBGainPercent: 100, GameFFBInvert: true})
	ep := normalizeNativeEngineProfile(NativeEngineProfile{Name: "x", RotationDegrees: 900, MasterGainPercent: 100, ConstantGainPercent: 100, SpringGainPercent: 100, DamperGainPercent: 100, FrictionGainPercent: 100})
	cfg := normalizeNativeFFBConfig(NativeFFBConfig{MasterGainPercent: 100, ConstantLimit: 10, SpringGain: 20, DamperGain: 20, FrictionGain: 20, SlewPerTick: 2, WatchdogMS: 1200})
	src := &fusionC6TelemetrySource{active: true, game: gp, engine: ep, cfg: cfg, adapterID: TelemetryAdapterWreckfestPino}
	fr, ok := src.TryGetFrame()
	if !ok {
		t.Fatal("no frame")
	}
	if math.Abs(fr.Constant-(-0.10)) > 0.001 {
		t.Fatalf("want safety-clamped -0.10 got %.3f", fr.Constant)
	}
	f.Physics = false
	installC6TestTelemetry(f)
	fr, ok = src.TryGetFrame()
	if !ok || fr.Constant != 0 {
		t.Fatalf("physics gate failed: %+v %v", fr, ok)
	}
}

func TestFusionC6LEDUsesOpenG27BarThresholds(t *testing.T) {
	if got := OpenG27RPMLEDMask(5320, 5320, 5600); got != 0x1f {
		t.Fatalf("redline mask %02x", got)
	}
	if got := OpenG27RPMLEDMask(2660, 5320, 5600); got != 0x07 {
		t.Fatalf("50%% mask %02x", got)
	}
}
