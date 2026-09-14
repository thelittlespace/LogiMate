//go:build windows

package system

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// FusionC3Status is intentionally separate from NativeFFBStatus. NativeFFBStatus
// remains LogiMate's global safety/owner view; this structure exposes the
// archived scheduler-reference internals for shadow validation.
type FusionC3Status struct {
	Active          bool
	Generation      uint64
	WheelID         string
	Model           string
	Requested       int
	Applied         int
	Pumps           uint64
	SourceFrames    uint64
	Writes          uint64
	SlewLimited     uint64
	WatchdogTrips   uint64
	LastHeartbeat   time.Time
	StartedAt       time.Time
	StoppedReason   string
	LastError       string
	SchedulerTick   time.Duration
	HardwareRouting bool
}

var fusionC3 = struct {
	sync.Mutex
	status FusionC3Status
}{}

func updateFusionC3StatusGeneration(generation uint64, fn func(*FusionC3Status)) bool {
	fusionC3.Lock()
	defer fusionC3.Unlock()
	if fusionC3.status.Generation != generation {
		return false
	}
	fn(&fusionC3.status)
	return true
}

func FusionC3Snapshot() FusionC3Status {
	fusionC3.Lock()
	defer fusionC3.Unlock()
	return fusionC3.status
}

type fusionC3ConstantSource struct {
	frame  wheelengine.ForceFrame
	active bool
	mu     sync.Mutex
}

func (s *fusionC3ConstantSource) Start() { s.mu.Lock(); s.active = true; s.mu.Unlock() }
func (s *fusionC3ConstantSource) Stop()  { s.mu.Lock(); s.active = false; s.mu.Unlock() }
func (s *fusionC3ConstantSource) TryGetFrame() (wheelengine.ForceFrame, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frame, s.active
}

var (
	winmmC3             = syscall.NewLazyDLL("winmm.dll")
	procTimeBeginPeriod = winmmC3.NewProc("timeBeginPeriod")
	procTimeEndPeriod   = winmmC3.NewProc("timeEndPeriod")
)

func fusionC3BeginTimerResolution() { _, _, _ = procTimeBeginPeriod.Call(1) }
func fusionC3EndTimerResolution()   { _, _, _ = procTimeEndPeriod.Call(1) }

// fusionC3SlewPerTick preserves LogiMate's configured slew RATE. The existing
// safety profile expresses percent change per 20 ms; the historical C3 reference pumps every
// 6 ms, so using the old numeric value unchanged would accelerate force ramps.
func fusionC3SlewPerTick(cfg NativeFFBConfig) float64 {
	cfg = normalizeNativeFFBConfig(cfg)
	per20ms := float64(cfg.SlewPerTick) / 100.0
	v := per20ms * float64(wheelengine.SchedulerTick) / float64(20*time.Millisecond)
	if v < 0.0005 {
		v = 0.0005
	}
	return v
}

// StartFusionC3ConstantForceTest is the first live hardware route that uses
// historical scheduler/output reference bytes. It is deliberately G27-only,
// explicit, capped at +/-5%, and hard-stopped after 900 ms. It does NOT replace
// the established LogiMate native FFB paths yet.
func StartFusionC3ConstantForceTest(s State, requestedPercent int) error {
	w, ok := SelectedWheel(s)
	if !ok || !IsG27Model(w.Model) {
		return errors.New("Der FFB-Live-Test ist ausschließlich für einen ausgewählten G27 freigegeben")
	}
	if requestedPercent == 0 || requestedPercent < -5 || requestedPercent > 5 {
		return errors.New("Der Scheduler-Test ist auf ±5 % begrenzt")
	}
	// C8 removes the second live HID writer. The historical C3 scheduler remains
	// as a dry-run/parity oracle, while every real motor test is routed through
	// the single authoritative Native FFB engine and its output lease.
	if err := StartNativeConstantForceTest(s, requestedPercent); err != nil {
		return err
	}
	ns := NativeFFBSnapshot()
	fusionC3.Lock()
	fusionC3.status = FusionC3Status{
		Active: true, Generation: ns.Generation, WheelID: w.ID, Model: w.Model,
		Requested: requestedPercent, Applied: ns.Applied, StartedAt: time.Now(),
		LastHeartbeat: ns.LastHeartbeat, SchedulerTick: 20 * time.Millisecond,
		HardwareRouting: true, StoppedReason: "delegated-to-unified-native-engine",
	}
	fusionC3.Unlock()
	go func(g uint64) {
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			st := NativeFFBSnapshot()
			updateFusionC3StatusGeneration(g, func(x *FusionC3Status) {
				x.Applied = st.Applied
				x.Pumps = st.Frames
				x.SlewLimited = st.SlewLimited
				x.LastHeartbeat = st.LastHeartbeat
				x.LastError = st.LastError
				x.Active = st.Active && st.Generation == g
			})
			if st.Generation != g || !st.Active {
				return
			}
		}
	}(ns.Generation)
	return nil
}

// FusionC3DryRun validates the translated scheduler without opening HID.
func FusionC3DryRun() (string, error) {
	transport := &fusionC3MemoryTransport{}
	now := time.Unix(100, 0)
	clock := func() time.Time { return now }
	output, err := wheelengine.NewWheelOutput(transport, wheelengine.WheelOutputOptions{MaxSlewPerTick: 1, Watchdog: 100 * time.Millisecond, DefaultRangeDeg: 900}, clock)
	if err != nil {
		return "", err
	}
	engine, err := wheelengine.NewFFBEngine(output)
	if err != nil {
		return "", err
	}
	source := &fusionC3DrySource{frame: wheelengine.ForceFrame{Constant: 0.5}, active: true}
	engine.SetSource(source)
	engine.SetMasterGain(0.5)
	if err := engine.PumpOnce(); err != nil {
		return "", err
	}
	want := wheelengine.ConstantForce(wheelengine.TorqueToWheelByte(0.25))
	if !bytes.Equal(transport.last(), want) {
		return "", fmt.Errorf("scheduler golden report: got % X want % X", transport.last(), want)
	}
	first := engine.Snapshot()
	if first.Pumps != 1 || first.SourceFrames != 1 {
		return "", fmt.Errorf("scheduler counters invalid: %+v", first)
	}
	// Remove frames and advance deterministic time beyond the watchdog. PumpOnce
	// must still Tick() and therefore write neutral.
	source.active = false
	now = now.Add(101 * time.Millisecond)
	if err := engine.PumpOnce(); err != nil {
		return "", err
	}
	if !bytes.Equal(transport.last(), wheelengine.ConstantForce(0x80)) {
		return "", fmt.Errorf("watchdog did not center: % X", transport.last())
	}
	if output.Snapshot().WatchdogTrips == 0 {
		return "", errors.New("watchdog trip counter did not advance")
	}
	if err := engine.Stop(); err != nil {
		return "", err
	}
	return fmt.Sprintf("PASS · tick=%s · pumps=%d · frames=%d · writes=%d · watchdogTrips=%d · final=% X", wheelengine.SchedulerTick, engine.Snapshot().Pumps, engine.Snapshot().SourceFrames, output.Snapshot().Writes, output.Snapshot().WatchdogTrips, transport.last()), nil
}

type fusionC3MemoryTransport struct{ writes [][]byte }

func (m *fusionC3MemoryTransport) Write(r []byte) error {
	m.writes = append(m.writes, append([]byte(nil), r...))
	return nil
}
func (m *fusionC3MemoryTransport) last() []byte {
	if len(m.writes) == 0 {
		return nil
	}
	return m.writes[len(m.writes)-1]
}

type fusionC3DrySource struct {
	frame  wheelengine.ForceFrame
	active bool
}

func (s *fusionC3DrySource) Start()                                      { s.active = true }
func (s *fusionC3DrySource) Stop()                                       { s.active = false }
func (s *fusionC3DrySource) TryGetFrame() (wheelengine.ForceFrame, bool) { return s.frame, s.active }

func FusionC3SchedulerSummary() string {
	dry, err := FusionC3DryRun()
	if err != nil {
		dry = "FAIL · " + err.Error()
	}
	st := FusionC3Snapshot()
	last := "—"
	if !st.LastHeartbeat.IsZero() {
		last = st.LastHeartbeat.Format("15:04:05.000")
	}
	live := fmt.Sprintf("active=%v gen=%d wheel=%s model=%s requested=%+d%% applied=%+d%% pumps=%d frames=%d writes=%d slewLimited=%d watchdogTrips=%d stop=%s last=%s error=%s",
		st.Active, st.Generation, st.WheelID, st.Model, st.Requested, st.Applied, st.Pumps, st.SourceFrames, st.Writes, st.SlewLimited, st.WatchdogTrips, st.StoppedReason, last, strings.TrimSpace(st.LastError))
	return "C3 Scheduler Dry Run: " + dry + "\r\nC3 Live Status: " + live + fmt.Sprintf("\r\nCadence: LogiMate Native %s (Target label %d Hz); Live-Test G27-only, max ±5%%, hard stop 900ms.", wheelengine.SchedulerTick, wheelengine.TargetHz)
}
