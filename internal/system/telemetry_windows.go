//go:build windows

package system

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/thelittlespace/LogiMate/internal/gameadapter"
	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// TelemetryFrame remains source-compatible with the pre-D3 system API while
// the actual canonical frame/lifecycle now belongs to internal/gameadapter.
type TelemetryFrame = gameadapter.Frame

type TelemetryStatus struct {
	Running      bool
	Adapter      string
	Address      string
	Packets      uint64
	Frames       uint64
	Invalid      uint64
	LastFrame    TelemetryFrame
	LastError    string
	Stale        bool
	StartedAt    time.Time
	LastFrameAge time.Duration
	PacketRateHz float64
	FrameRateHz  float64
}

type TelemetryAdapterInfo struct {
	ID, Name        string
	Port            int
	Implemented     bool
	Automatic       bool
	ProvidesRPM     bool
	ProvidesForce   bool
	ProvidesFFBAuth bool
	Setup           string
}

var telemetryAdapters = gameadapter.NewRegistry()

var telemetryRuntime struct {
	sync.Mutex
	adapter   gameadapter.Adapter
	automatic bool
}

func canonicalTelemetryAdapterID(id string) string { return telemetryAdapters.CanonicalID(id) }
func isWreckfestPinoAdapter(id string) bool {
	return canonicalTelemetryAdapterID(id) == gameadapter.IDWreckfestPino
}

const (
	TelemetryAdapterLogiMateJSON      = gameadapter.IDLocalJSON
	TelemetryAdapterWreckfestPino     = gameadapter.IDWreckfestPino
	legacyTelemetryAdapterOpenG27Pino = gameadapter.LegacyIDOpenG27Pino
)

func telemetryInfo(d gameadapter.Descriptor) TelemetryAdapterInfo {
	return TelemetryAdapterInfo{
		ID: d.ID, Name: d.Name, Port: d.DefaultPort, Implemented: d.Implemented, Automatic: d.Automatic,
		ProvidesRPM:     d.Caps&gameadapter.CapabilityRPM != 0,
		ProvidesForce:   d.Caps&gameadapter.CapabilityForce != 0,
		ProvidesFFBAuth: d.Caps&gameadapter.CapabilityFFBAuthorization != 0,
		Setup:           d.Setup,
	}
}

func RegisteredTelemetryAdapters() []TelemetryAdapterInfo {
	d := telemetryAdapters.Descriptors()
	out := make([]TelemetryAdapterInfo, 0, len(d))
	for _, x := range d {
		out = append(out, telemetryInfo(x))
	}
	return out
}

func TelemetryAdapterByID(id string) (TelemetryAdapterInfo, bool) {
	id = canonicalTelemetryAdapterID(id)
	for _, d := range telemetryAdapters.Descriptors() {
		if d.ID == id {
			return telemetryInfo(d), true
		}
	}
	return TelemetryAdapterInfo{}, false
}

func startTelemetryAdapter(id string, port int, automatic bool) error {
	id = canonicalTelemetryAdapterID(id)
	candidate, err := telemetryAdapters.New(id)
	if err != nil {
		return err
	}
	if automatic && !candidate.Descriptor().Automatic {
		return fmt.Errorf("Telemetry-Adapter %q wird bewusst nur manuell gestartet", id)
	}
	StopTelemetry()
	if err := candidate.Start(gameadapter.StartOptions{Port: port, Automatic: automatic}); err != nil {
		return err
	}
	telemetryRuntime.Lock()
	telemetryRuntime.adapter = candidate
	telemetryRuntime.automatic = automatic
	telemetryRuntime.Unlock()
	return nil
}

func ParseLogiMateJSONTelemetry(b []byte) (TelemetryFrame, error) {
	return gameadapter.ParseLocalJSON(b, time.Now())
}
func ParseWreckfestPinoTelemetry(b []byte) (TelemetryFrame, error) {
	return gameadapter.ParseWreckfestPino(b, time.Now())
}
func ParseOpenG27PinoTelemetry(b []byte) (TelemetryFrame, error) {
	return ParseWreckfestPinoTelemetry(b)
}

func StartLocalTelemetryJSON(port int) error {
	return startTelemetryAdapter(TelemetryAdapterLogiMateJSON, port, false)
}
func StartWreckfestPinoTelemetry(port int) error {
	return startTelemetryAdapter(TelemetryAdapterWreckfestPino, port, false)
}
func startWreckfestPinoTelemetry(port int, automatic bool) error {
	return startTelemetryAdapter(TelemetryAdapterWreckfestPino, port, automatic)
}

func StopTelemetry() {
	telemetryRuntime.Lock()
	a := telemetryRuntime.adapter
	telemetryRuntime.adapter = nil
	telemetryRuntime.automatic = false
	telemetryRuntime.Unlock()
	if a != nil {
		_ = a.Stop()
	}
}

func TelemetrySnapshot() TelemetryStatus {
	telemetryRuntime.Lock()
	a := telemetryRuntime.adapter
	telemetryRuntime.Unlock()
	if a == nil {
		return TelemetryStatus{}
	}
	s := a.Snapshot()
	out := TelemetryStatus{Running: s.Health.Running, Adapter: s.Descriptor.ID, Address: s.Health.Address, Packets: s.Health.Packets, Frames: s.Health.Frames, Invalid: s.Health.Invalid, LastFrame: s.LastFrame, LastError: s.Health.LastError, Stale: s.Stale, StartedAt: s.Health.StartedAt}
	if !s.LastFrame.ReceivedAt.IsZero() {
		out.LastFrameAge = time.Since(s.LastFrame.ReceivedAt)
	}
	if !s.Health.StartedAt.IsZero() {
		secs := time.Since(s.Health.StartedAt).Seconds()
		if secs > 0 {
			out.PacketRateHz = float64(s.Health.Packets) / secs
			out.FrameRateHz = float64(s.Health.Frames) / secs
		}
	}
	return out
}

func TelemetryRecentFrames() []TelemetryFrame {
	telemetryRuntime.Lock()
	a := telemetryRuntime.adapter
	telemetryRuntime.Unlock()
	if a == nil {
		return nil
	}
	return a.RecentFrames()
}

func EnsureGameProfileTelemetry(p GameProfile) error {
	want := canonicalTelemetryAdapterID(p.TelemetryAdapter)
	telemetryRuntime.Lock()
	a := telemetryRuntime.adapter
	automatic := telemetryRuntime.automatic
	telemetryRuntime.Unlock()
	if want == "" {
		if a != nil && automatic {
			StopTelemetry()
		}
		return nil
	}
	info, ok := TelemetryAdapterByID(want)
	if !ok || !info.Implemented {
		return fmt.Errorf("Telemetry-Adapter %q ist nicht verfügbar", want)
	}
	if a != nil && canonicalTelemetryAdapterID(a.Descriptor().ID) == want && a.Snapshot().Health.Running {
		return nil
	}
	if a != nil && automatic {
		StopTelemetry()
	}
	if !info.Automatic {
		return fmt.Errorf("Telemetry-Adapter %q unterstützt keinen automatischen Lifecycle", want)
	}
	port := p.TelemetryPort
	if port <= 0 {
		port = info.Port
	}
	if err := startTelemetryAdapter(want, port, true); err != nil {
		return fmt.Errorf("Telemetry-Adapter %s konnte nicht gestartet werden: %w", want, err)
	}
	return nil
}

func StopAutomaticGameTelemetry() {
	telemetryRuntime.Lock()
	a := telemetryRuntime.adapter
	automatic := telemetryRuntime.automatic
	telemetryRuntime.Unlock()
	if a != nil && automatic {
		StopTelemetry()
	}
}

func RPMToG27LEDMask(rpm, redline, max int) byte { return NativeRPMLEDMask(rpm, redline, max) }
func NativeRPMLEDMask(rpm, redline, max int) byte {
	top := redline
	if top <= 0 {
		top = max
	}
	if top <= 0 || rpm <= 0 {
		return 0
	}
	percent := rpm * 100 / top
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return wheelengine.LedBarForPercent(percent)
}
func OpenG27RPMLEDMask(rpm, redline, max int) byte { return NativeRPMLEDMask(rpm, redline, max) }

func TelemetryForcePercent(f TelemetryFrame) int {
	if !f.Physics || !f.PlayerControl {
		return 0
	}
	v := int(math.Round(f.Force * 100))
	return clampInt(v, -100, 100)
}

func telemetryAdapterSupportsForce(id string) bool {
	info, ok := TelemetryAdapterByID(id)
	return ok && info.ProvidesForce
}
func telemetryAdapterSupportsRPM(id string) bool {
	info, ok := TelemetryAdapterByID(id)
	return ok && info.ProvidesRPM
}
func telemetryAdapterRequiresFFBAuthorization(id string) bool {
	info, ok := TelemetryAdapterByID(id)
	return ok && info.ProvidesFFBAuth
}

// kept for older tests/callers that compare normalized ids.
func sameTelemetryAdapter(a, b string) bool {
	return strings.EqualFold(canonicalTelemetryAdapterID(a), canonicalTelemetryAdapterID(b))
}
