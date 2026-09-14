//go:build windows

package system

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

type FusionC6Status struct {
	Active                  bool
	Generation              uint64
	WheelID                 string
	Profile                 string
	EngineProfile           string
	Process                 string
	Adapter                 string
	FFBEnabled              bool
	LEDsEnabled             bool
	GameFFBGainPercent      int
	GameMasterGainPercent   int
	GameConstantGainPercent int
	GameFFBInvert           bool
	RequestedForce          float64
	ShapedForce             float64
	AppliedPercent          int
	FFBPreset               string
	TuningRevision          uint64
	TransportBackend        string
	PipelineClips           uint64
	SafetyClips             uint64
	PipelineDeadband        uint64
	PipelineMinForce        uint64
	SampleLatency           time.Duration
	MaxSampleLatency        time.Duration
	LEDMask                 byte
	Pumps                   uint64
	Writes                  uint64
	TelemetryFrames         uint64
	LastTelemetry           time.Time
	LastHeartbeat           time.Time
	StoppedReason           string
	LastError               string
}

var fusionC6 = struct {
	sync.Mutex
	status         FusionC6Status
	source         *fusionC6TelemetrySource
	tuningRevision uint64
}{}

func FusionC6Snapshot() FusionC6Status {
	fusionC6.Lock()
	defer fusionC6.Unlock()
	return fusionC6.status
}
func updateFusionC6(g uint64, fn func(*FusionC6Status)) bool {
	fusionC6.Lock()
	defer fusionC6.Unlock()
	if fusionC6.status.Generation != g {
		return false
	}
	fn(&fusionC6.status)
	return true
}

type NativeGameTuningApplyResult struct {
	Active        bool
	EngineProfile string
	FFBPreset     string
	Revision      uint64
}

// RefreshNativeGameOutputTuning applies the currently persisted per-wheel
// profile and D4 tuning to an already running telemetry/game FFB session.
// D6.0 only updated the files, so sliders could look successful while the
// active runtime continued using the values captured when it started.
func RefreshNativeGameOutputTuning(s State) (NativeGameTuningApplyResult, error) {
	w, ok := SelectedWheel(s)
	if !ok {
		return NativeGameTuningApplyResult{}, errors.New("kein physisches Wheel ausgewählt")
	}

	fusionC6.Lock()
	st := fusionC6.status
	src := fusionC6.source
	fusionC6.Unlock()
	if !st.Active || src == nil || !st.FFBEnabled {
		return NativeGameTuningApplyResult{Active: false}, nil
	}
	if !strings.EqualFold(st.WheelID, w.ID) {
		return NativeGameTuningApplyResult{}, errors.New("laufendes Game-FFB gehört zu einem anderen Wheel")
	}

	ep := ReadActiveNativeEngineProfile(s.DataDir, w.ID)
	advanced := ReadAdvancedFFBConfig(s.DataDir, w.ID, wheelModelKind(w))
	tuning := AdvancedFFBTuning(advanced, wheelModelKind(w))

	src.mu.Lock()
	src.engine = ep
	pipeline := src.pipeline
	src.mu.Unlock()
	if pipeline == nil {
		return NativeGameTuningApplyResult{}, errors.New("laufendes Game-FFB besitzt keine aktive Signalformungs-Pipeline")
	}
	if err := pipeline.SetTuning(tuning); err != nil {
		return NativeGameTuningApplyResult{}, fmt.Errorf("Live-Signalformung konnte nicht übernommen werden: %w", err)
	}

	fusionC6.Lock()
	if fusionC6.status.Generation != st.Generation || fusionC6.source != src {
		fusionC6.Unlock()
		return NativeGameTuningApplyResult{}, errors.New("Game-FFB wurde während der Tuning-Aktualisierung neu gestartet")
	}
	fusionC6.tuningRevision++
	revision := fusionC6.tuningRevision
	fusionC6.status.EngineProfile = ep.Name
	fusionC6.status.FFBPreset = advanced.Preset
	fusionC6.status.TuningRevision = revision
	fusionC6.Unlock()

	updateNativeFFBStatusGeneration(st.Generation, func(x *NativeFFBStatus) {
		x.ProfileName = ep.Name
		x.LastHeartbeat = time.Now()
	})
	return NativeGameTuningApplyResult{Active: true, EngineProfile: ep.Name, FFBPreset: advanced.Preset, Revision: revision}, nil
}

type fusionC6Transport struct {
	mu     sync.Mutex
	tr     *nativeHIDTransport
	writes uint64
}

func (t *fusionC6Transport) Write(report7 []byte) error {
	if len(report7) != 7 {
		return fmt.Errorf("C6 erwartet 7 lg4ff Bytes, erhalten: %d", len(report7))
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tr == nil {
		return errors.New("C6 HID-Transport ist nicht geöffnet")
	}
	if err := t.tr.WriteReport(wheelengine.WithReportID(report7)); err != nil {
		return err
	}
	t.writes++
	return nil
}
func (t *fusionC6Transport) Writes() uint64 { t.mu.Lock(); defer t.mu.Unlock(); return t.writes }
func (t *fusionC6Transport) Backend() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tr == nil {
		return ""
	}
	return t.tr.Backend()
}

type fusionC6TelemetrySource struct {
	mu          sync.Mutex
	active      bool
	game        GameProfile
	engine      NativeEngineProfile
	cfg         NativeFFBConfig
	adapterID   string
	requested   float64
	shaped      float64
	applied     int
	safetyClips uint64
	pipeline    *wheelengine.FFBPipeline
}

func (s *fusionC6TelemetrySource) Start() { s.mu.Lock(); s.active = true; s.mu.Unlock() }
func (s *fusionC6TelemetrySource) Stop()  { s.mu.Lock(); s.active = false; s.mu.Unlock() }
func (s *fusionC6TelemetrySource) TryGetFrame() (wheelengine.ForceFrame, bool) {
	s.mu.Lock()
	active := s.active
	gp := s.game
	ep := s.engine
	cfg := s.cfg
	s.mu.Unlock()
	if !active || !gp.GameFFBEnabled {
		return wheelengine.ForceFrame{}, false
	}
	t := TelemetrySnapshot()
	if !t.Running || !sameTelemetryAdapter(t.Adapter, s.adapterID) || t.Stale || t.LastFrame.ReceivedAt.IsZero() {
		return wheelengine.ForceFrame{}, false
	}
	force := t.LastFrame.Force
	// Pino's own FFBEnabled bit is authoritative. A game/telemetry frame that
	// says FFB is disabled may still carry a stale/non-zero force field.
	if !t.LastFrame.Physics || !t.LastFrame.PlayerControl || (telemetryAdapterRequiresFFBAuthorization(s.adapterID) && !t.LastFrame.FFBEnabled) {
		force = 0
	}
	if gp.GameFFBInvert {
		force = -force
	}
	gameGain := float64(gp.GameFFBGainPercent) / 100.0
	gameGain *= float64(gp.MasterGainPercent) / 100.0
	gameGain *= float64(gp.ConstantGainPercent) / 100.0
	shaped := force * gameGain
	if s.pipeline != nil {
		shaped = s.pipeline.ProcessFrame(wheelengine.ForceFrame{Constant: force}, gameGain, t.LastFrame.ReceivedAt, time.Now())
	} else {
		shaped = wheelengine.Clamp(shaped, -1, 1)
	}
	nominal := int(math.Round(shaped * 100.0))
	mix := MixNativeEffects(NativeEffectRequest{Constant: nominal}, ep, cfg)
	s.mu.Lock()
	s.requested = force
	s.shaped = shaped
	s.applied = mix.Constant
	s.safetyClips += uint64(mix.ClipEvents)
	s.mu.Unlock()
	return wheelengine.ForceFrame{Constant: float64(mix.Constant) / 100.0}, true
}
func (s *fusionC6TelemetrySource) Snapshot() (float64, float64, int, uint64, wheelengine.FFBPipelineSnapshot) {
	s.mu.Lock()
	requested, shaped, applied, safetyClips, pipeline := s.requested, s.shaped, s.applied, s.safetyClips, s.pipeline
	s.mu.Unlock()
	var ps wheelengine.FFBPipelineSnapshot
	if pipeline != nil {
		ps = pipeline.Snapshot()
	}
	return requested, shaped, applied, safetyClips, ps
}

func StartFusionC6GameOutput(s State, p GameProfile, process string) error {
	p = normalizeGameProfile(p)
	adapterID := canonicalTelemetryAdapterID(p.TelemetryAdapter)
	adapterInfo, adapterOK := TelemetryAdapterByID(adapterID)
	if !adapterOK || !adapterInfo.Implemented {
		return fmt.Errorf("aktives Game-Profil verwendet keinen verfügbaren Telemetrie-Adapter: %q", adapterID)
	}
	if p.GameFFBEnabled && !adapterInfo.ProvidesForce {
		return fmt.Errorf("Telemetry-Adapter %q liefert keinen Force-Kanal", adapterID)
	}
	if !p.GameFFBEnabled && p.LEDPolicy != "telemetry" {
		return errors.New("Profil aktiviert weder Game-FFB noch Telemetrie-LEDs")
	}
	if err := stopNativeFFBSession(false); err != nil {
		return err
	}
	lease, err := acquireNativeOutputLease(s, "native-game-adapter:"+adapterID, p.GameFFBEnabled)
	if err != nil {
		return err
	}
	w, _ := SelectedWheel(s)
	caps, capOK := WheelCapabilitiesForDevice(w)
	if !capOK || !caps.HasNativeFFB {
		releaseNativeOutputLease(lease)
		return errors.New("dieses Wheel besitzt keinen LogiMate-Native-FFB-Pfad")
	}
	ledsEnabled := p.LEDPolicy == "telemetry" && caps.HasRPMLEDs && adapterInfo.ProvidesRPM
	if !p.GameFFBEnabled && !ledsEnabled {
		releaseNativeOutputLease(lease)
		return errors.New("Profil fordert nur Telemetrie-LEDs, dieses Wheel besitzt jedoch keine unterstützten Rev-LEDs")
	}
	if err := EnsureGameProfileTelemetry(p); err != nil {
		releaseNativeOutputLease(lease)
		return fmt.Errorf("Telemetry-Adapter %s: %w", adapterID, err)
	}
	cfg := ReadNativeFFBConfig(s.DataDir, w.ID)
	advanced := ReadAdvancedFFBConfig(s.DataDir, w.ID, wheelModelKind(w))
	pipeline, err := wheelengine.NewFFBPipeline(AdvancedFFBTuning(advanced, wheelModelKind(w)))
	if err != nil {
		releaseNativeOutputLease(lease)
		return fmt.Errorf("FFB-Signalformungs-Pipeline konnte nicht initialisiert werden: %w", err)
	}
	ep, ok := ReadNativeEngineProfile(s.DataDir, w.ID, p.EngineProfile)
	if !ok {
		ep = ReadActiveNativeEngineProfile(s.DataDir, w.ID)
	}
	hidTransport, err := openNativeHIDTransport(lease.Path)
	if err != nil {
		releaseNativeOutputLease(lease)
		return err
	}
	tr := &fusionC6Transport{tr: hidTransport}
	// LED-off is used only on wheels that actually expose the G27-style RPM
	// LEDs. Motor-capable wheels without LEDs are proven by the same exclusive
	// transport and, when Game-FFB is enabled, the first guarded FFB pump below.
	if ledsEnabled {
		if err := tr.Write(wheelengine.LedsOff()); err != nil {
			_ = hidTransport.Close()
			releaseNativeOutputLease(lease)
			return fmt.Errorf("initialer LED-HID-Write fehlgeschlagen: %w", err)
		}
	}

	var output *wheelengine.WheelOutput
	var engine *wheelengine.FFBEngine
	var src *fusionC6TelemetrySource
	if p.GameFFBEnabled {
		if err := MarkRuntimeOutputActiveTarget(s.DataDir, w, "native-game-adapter:"+adapterID); err != nil {
			_ = hidTransport.Close()
			releaseNativeOutputLease(lease)
			return fmt.Errorf("Recovery-Marker konnte nicht geschrieben werden; Game-FFB verweigert: %w", err)
		}
		output, err = wheelengine.NewWheelOutput(tr, wheelengine.WheelOutputOptions{MaxSlewPerTick: fusionC3SlewPerTick(cfg), Watchdog: time.Duration(cfg.WatchdogMS) * time.Millisecond, DefaultRangeDeg: ep.RotationDegrees}, nil)
		if err != nil {
			_ = hidTransport.Close()
			return abortMotorStartAfterPossibleWrite(lease, err)
		}
		engine, err = wheelengine.NewFFBEngine(output)
		if err != nil {
			_ = hidTransport.Close()
			return abortMotorStartAfterPossibleWrite(lease, err)
		}
		src = &fusionC6TelemetrySource{adapterID: adapterID, game: p, engine: ep, cfg: cfg, active: true, pipeline: pipeline}
		engine.SetSource(src)
		engine.SetMasterGain(1)
		if err := engine.PumpOnce(); err != nil {
			_ = hidTransport.Close()
			return abortMotorStartAfterPossibleWrite(lease, fmt.Errorf("initialer C6 FFB-Pump fehlgeschlagen: %w", err))
		}
		fusionC3BeginTimerResolution()
		if !engine.Start() {
			fusionC3EndTimerResolution()
			_ = hidTransport.Close()
			return abortMotorStartAfterPossibleWrite(lease, errors.New("C6 FFB-Scheduler konnte nicht gestartet werden"))
		}
	}

	cancel := make(chan struct{})
	done := make(chan struct{})
	nativeFFB.Lock()
	prev := nativeFFB.status
	nativeFFB.generation++
	generation := nativeFFB.generation
	nativeFFB.cancel, nativeFFB.done = cancel, done
	leaseCopy := lease
	nativeFFB.lease = &leaseCopy
	nativeFFB.dataDir = s.DataDir
	nativeFFB.status = NativeFFBStatus{Active: p.GameFFBEnabled, WheelID: w.ID, Model: w.Model, Effect: "native-game-adapter:" + adapterID, Config: cfg, ProfileName: ep.Name, Generation: generation, StartedAt: time.Now(), LastHeartbeat: time.Now(), ClipEvents: prev.ClipEvents, EffectTransitions: prev.EffectTransitions + 1, WatchdogStops: prev.WatchdogStops, EmergencyStops: prev.EmergencyStops}
	nativeFFB.Unlock()
	fusionC6.Lock()
	fusionC6.tuningRevision++
	fusionC6.source = src
	fusionC6.status = FusionC6Status{Active: true, Generation: generation, WheelID: w.ID, Profile: p.Name, EngineProfile: ep.Name, Process: process, Adapter: adapterID, FFBEnabled: p.GameFFBEnabled, LEDsEnabled: ledsEnabled, GameFFBGainPercent: p.GameFFBGainPercent, GameMasterGainPercent: p.MasterGainPercent, GameConstantGainPercent: p.ConstantGainPercent, GameFFBInvert: p.GameFFBInvert, FFBPreset: advanced.Preset, TuningRevision: fusionC6.tuningRevision, TransportBackend: tr.Backend(), LastHeartbeat: time.Now()}
	fusionC6.Unlock()

	go func() {
		defer close(done)
		defer hidTransport.Close()
		defer func() {
			nativeFFB.Lock()
			if nativeFFB.lease != nil && nativeFFB.lease.Generation == lease.Generation {
				if cur, ok := currentNativeOutputLease(); !ok || cur.Generation != lease.Generation {
					nativeFFB.lease = nil
				}
			}
			if nativeFFB.cancel == cancel {
				nativeFFB.cancel = nil
			}
			if nativeFFB.done == done {
				nativeFFB.done = nil
			}
			nativeFFB.Unlock()
		}()
		reason := "stopped"
		ticker := time.NewTicker(40 * time.Millisecond)
		defer ticker.Stop()
		var lastLED byte = 0xff
		for {
			select {
			case <-cancel:
				reason = "cancelled"
				goto stop
			case <-ticker.C:
				ts := TelemetrySnapshot()
				mask := byte(0)
				if ledsEnabled && ts.Running && !ts.Stale && sameTelemetryAdapter(ts.Adapter, adapterID) {
					mask = NativeRPMLEDMask(ts.LastFrame.RPM, ts.LastFrame.RPMRedline, ts.LastFrame.RPMMax)
				}
				if mask != lastLED {
					if err := tr.Write(wheelengine.SetLeds(mask)); err != nil {
						updateFusionC6(generation, func(st *FusionC6Status) { st.LastError = err.Error() })
						reason = "led-write-error"
						goto stop
					}
					lastLED = mask
				}
				var req float64
				var shaped float64
				var applied int
				var safetyClips uint64
				var pipeSnap wheelengine.FFBPipelineSnapshot
				var pumps uint64
				var heartbeat = time.Now()
				var outErr string
				var slew uint64
				if p.GameFFBEnabled && engine != nil && output != nil && src != nil {
					es := engine.Snapshot()
					os := output.Snapshot()
					req, shaped, applied, safetyClips, pipeSnap = src.Snapshot()
					pumps, heartbeat, slew = es.Pumps, es.LastPump, os.SlewLimited
					if es.LastError != "" {
						outErr = es.LastError
					} else {
						outErr = os.LastError
					}
				}
				updateFusionC6(generation, func(st *FusionC6Status) {
					st.RequestedForce = req
					st.ShapedForce = shaped
					st.AppliedPercent = applied
					st.PipelineClips = pipeSnap.InputClipEvents + pipeSnap.OutputClipEvents
					st.SafetyClips = safetyClips
					st.PipelineDeadband = pipeSnap.DeadbandHits
					st.PipelineMinForce = pipeSnap.MinimumForceApplications
					st.SampleLatency = pipeSnap.AverageSampleLatency
					st.MaxSampleLatency = pipeSnap.MaxSampleLatency
					st.LEDMask = mask
					st.Pumps = pumps
					st.Writes = tr.Writes()
					st.TransportBackend = tr.Backend()
					st.TelemetryFrames = ts.Frames
					st.LastTelemetry = ts.LastFrame.ReceivedAt
					st.LastHeartbeat = heartbeat
					if outErr != "" {
						st.LastError = outErr
					}
				})
				if p.GameFFBEnabled {
					updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) {
						st.Applied = applied
						st.Frames = pumps
						st.ClipEvents = prev.ClipEvents + safetyClips
						st.SlewLimited = slew
						st.LastHeartbeat = heartbeat
						if outErr != "" {
							st.LastError = outErr
						}
					})
				}
			}
		}
	stop:
		var stopErr error
		if engine != nil {
			stopErr = engine.Stop()
			fusionC3EndTimerResolution()
		}
		if ledsEnabled {
			ledErr := tr.Write(wheelengine.LedsOff())
			if stopErr == nil && ledErr != nil {
				stopErr = ledErr
			}
		}
		if p.GameFFBEnabled {
			stopErr = completeNativeMotorStop(lease, s.DataDir, stopErr)
		} else if stopErr == nil && !releaseNativeOutputLease(lease) {
			stopErr = errors.New("C6 Output-Lease konnte nach bestätigtem Stop nicht freigegeben werden")
		}
		updateFusionC6(generation, func(st *FusionC6Status) {
			st.Active = stopErr != nil
			st.AppliedPercent = 0
			st.LEDMask = 0
			st.Writes = tr.Writes()
			st.LastHeartbeat = time.Now()
			st.StoppedReason = reason
			if stopErr != nil {
				st.LastError = stopErr.Error()
			}
		})
		fusionC6.Lock()
		if fusionC6.status.Generation == generation && stopErr == nil {
			fusionC6.source = nil
		}
		fusionC6.Unlock()
		updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) {
			st.Stopping = false
			st.Faulted = stopErr != nil && p.GameFFBEnabled
			st.Active = stopErr != nil && p.GameFFBEnabled
			if stopErr == nil {
				st.Applied = 0
				st.LastError = ""
			} else {
				st.LastError = stopErr.Error()
			}
			st.LastHeartbeat = time.Now()
		})
	}()
	return nil
}

func StopFusionC6GameOutput(reason string) {
	if strings.TrimSpace(reason) == "" {
		reason = "user"
	}
	st := FusionC6Snapshot()
	if st.Active {
		err := stopNativeFFBSession(false)
		updateFusionC6(st.Generation, func(x *FusionC6Status) {
			x.Active = err != nil
			x.StoppedReason = reason
			if err == nil {
				x.AppliedPercent = 0
				x.LEDMask = 0
				x.LastError = ""
			} else {
				x.LastError = err.Error()
			}
		})
	}
}

func FusionC6Summary() string {
	st := FusionC6Snapshot()
	ts := TelemetrySnapshot()
	last := "—"
	if !st.LastHeartbeat.IsZero() {
		last = st.LastHeartbeat.Format("15:04:05.000")
	}
	return fmt.Sprintf("FFB Adapter: listener=%v adapter=%s frames=%d stale=%v | output=%v profile=%s engineProfile=%s process=%s outputAdapter=%s ffb=%v leds=%v raw=%+.3f shaped=%+.3f applied=%+d%% preset=%s tuningRev=%d transport=%s pipeClips=%d safetyClips=%d latency=%s/%s led=0x%02X pumps=%d writes=%d stop=%s last=%s error=%s", ts.Running, ts.Adapter, ts.Frames, ts.Stale, st.Active, st.Profile, st.EngineProfile, st.Process, st.Adapter, st.FFBEnabled, st.LEDsEnabled, st.RequestedForce, st.ShapedForce, st.AppliedPercent, st.FFBPreset, st.TuningRevision, st.TransportBackend, st.PipelineClips, st.SafetyClips, st.SampleLatency, st.MaxSampleLatency, st.LEDMask, st.Pumps, st.Writes, st.StoppedReason, last, st.LastError)
}
