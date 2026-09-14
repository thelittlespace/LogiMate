//go:build windows

package system

import (
	"strings"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

var nativeWheelManager = wheelengine.NewManager()

// NativeWheelEngineSnapshot exposes D0's canonical runtime state to diagnostics
// and tests without leaking the manager's mutable internals.
func NativeWheelEngineSnapshot() wheelengine.Snapshot { return nativeWheelManager.Snapshot() }

func wheelEngineDevice(w WheelDevice) wheelengine.Device {
	return wheelengine.Device{
		StableID: w.ID, SessionID: w.SessionID, HardwareFingerprint: w.HardwareFingerprint,
		PersistentIdentity: w.PersistentIdentity, Name: w.Name, Model: ClassifyWheelModel(w.Model),
		Mode: ClassifyOperatingMode(w.Mode), Supported: w.Supported, ModelConfirmed: w.ModelConfirmed,
		PnPVerified: w.PnPVerified,
	}
}

func syncNativeWheelManager(s *State) {
	if s == nil {
		return
	}
	devices := make([]wheelengine.Device, 0, len(s.Wheels))
	for _, w := range s.Wheels {
		devices = append(devices, wheelEngineDevice(w))
	}
	snap := nativeWheelManager.Reconcile(devices, s.SelectedWheelID, time.Now())
	s.EngineGeneration = snap.Generation
	s.SelectedModel = WheelModelUnknown
	s.SelectedMode = ClassifyOperatingMode(s.ActiveMode)
	if w, ok := SelectedWheel(*s); ok {
		s.SelectedModel = ClassifyWheelModel(w.Model)
		s.SelectedMode = ClassifyOperatingMode(w.Mode)
	}
}

func normalizeAxis(raw, min, max uint32) float64 {
	if max <= min {
		return 0
	}
	x := (float64(raw) - float64(min)) / (float64(max) - float64(min))
	if x < 0 {
		x = 0
	}
	if x > 1 {
		x = 1
	}
	return x
}

func inputConnectionState(s string, found bool) wheelengine.ConnectionState {
	v := strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(v, "ambiguous"):
		return wheelengine.ConnectionAmbiguous
	case strings.Contains(v, "reconnect"):
		return wheelengine.ConnectionReconnecting
	case strings.Contains(v, "disconnect"):
		return wheelengine.ConnectionDisconnected
	case strings.Contains(v, "error"), strings.Contains(v, "fehler"):
		return wheelengine.ConnectionError
	case found:
		return wheelengine.ConnectionConnected
	default:
		return wheelengine.ConnectionUnknown
	}
}

func inputSourceKind(s string) wheelengine.InputSource {
	v := strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(v, "direct"), strings.Contains(v, "hid"):
		return wheelengine.InputSourceDirectHID
	case strings.Contains(v, "winmm"):
		return wheelengine.InputSourceWinMM
	default:
		return wheelengine.InputSourceUnknown
	}
}

func canonicalInputState(s State, j JoyState) wheelengine.InputState {
	model := ClassifyWheelModel(s.WheelModel)
	if w, ok := SelectedWheel(s); ok {
		model = ClassifyWheelModel(w.Model)
	}
	steering := normalizeAxis(j.X, j.XMin, j.XMax)*2 - 1
	if j.XMax <= j.XMin {
		steering = 0
	}
	gas, _ := PedalPercent(j, s.Pedals, "gas")
	brake, _ := PedalPercent(j, s.Pedals, "brake")
	clutch, _ := PedalPercent(j, s.Pedals, "clutch")
	return wheelengine.InputState{
		WheelID: j.WheelID, SessionID: j.SessionID, Model: model,
		Source: inputSourceKind(j.InputSource), LayoutID: j.LayoutID,
		Connection: inputConnectionState(j.ConnectionState, j.Found), SampleValid: j.SampleValid,
		SampleGeneration: j.SampleGeneration, SampleAt: j.SampleAt,
		Steering: steering, SteeringDegrees: j.SteeringDegrees,
		Throttle: gas, Brake: brake, Clutch: clutch, Buttons: uint64(j.Buttons), DPad: j.DPad, Gear: j.Gear,
		PaddleLeft: j.PaddleLeft, PaddleRight: j.PaddleRight,
		Error: strings.TrimSpace(firstNonEmpty(j.LastInputError, j.Error)),
	}
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func updateNativeWheelInput(s State, j JoyState) {
	if strings.TrimSpace(j.WheelID) == "" || strings.TrimSpace(j.SessionID) == "" {
		return
	}
	_ = nativeWheelManager.UpdateInput(canonicalInputState(s, j), time.Now())
}

func updateNativeWheelOutput(t nativeOutputLeaseToken, command, lastErr string, active bool) {
	if strings.TrimSpace(t.WheelID) == "" || strings.TrimSpace(t.SessionID) == "" {
		return
	}
	recovery := false
	if strings.TrimSpace(t.DataDir) != "" {
		recovery, _ = RuntimeOutputRecoveryNeeded(t.DataDir)
	}
	_ = nativeWheelManager.UpdateOutput(t.WheelID, t.SessionID, wheelengine.OutputState{
		LeaseActive: active, Generation: t.Generation, Motor: t.Motor, Purpose: t.Purpose,
		LastCommand: command, LastError: lastErr, UpdatedAt: time.Now(),
	}, recovery, time.Now())
}
