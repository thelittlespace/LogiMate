package wheelengine

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// FFBTuning describes non-safety signal shaping. It is intentionally bounded
// to normalized -1..+1 input/output. Hardware safety ceilings MUST be applied
// after this pipeline by the caller.
type FFBTuning struct {
	Enabled          bool
	ConstantGain     float64 // 0..1.5
	TransientGain    float64 // 0..1.5
	Deadband         float64 // 0..0.20
	MinimumForce     float64 // 0..0.20, applied after deadband/curve
	ResponseExponent float64 // 0.50..2.00; <1 boosts detail, >1 softens center
	LowPassHz        float64 // 0 disables; otherwise 5..100 Hz
	Smoothing        float64 // 0..0.90, extra exponential smoothing
	OutputLimit      float64 // 0.10..1.00 normalized pre-safety ceiling
}

func DefaultFFBTuning() FFBTuning {
	return FFBTuning{
		Enabled:          true,
		ConstantGain:     1,
		TransientGain:    1,
		Deadband:         0,
		MinimumForce:     0,
		ResponseExponent: 1,
		LowPassHz:        0,
		Smoothing:        0,
		OutputLimit:      1,
	}
}

func NormalizeFFBTuning(t FFBTuning) (FFBTuning, error) {
	if !t.Enabled {
		// Disabled still carries safe/default numeric values so diagnostics are
		// deterministic and a later enable cannot revive NaN/Inf settings.
		d := DefaultFFBTuning()
		d.Enabled = false
		return d, nil
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	for name, v := range map[string]float64{
		"ConstantGain": t.ConstantGain, "TransientGain": t.TransientGain,
		"Deadband": t.Deadband, "MinimumForce": t.MinimumForce,
		"ResponseExponent": t.ResponseExponent, "LowPassHz": t.LowPassHz,
		"Smoothing": t.Smoothing, "OutputLimit": t.OutputLimit,
	} {
		if !finite(v) {
			return t, fmt.Errorf("%s must be finite", name)
		}
	}
	if t.ResponseExponent == 0 {
		t.ResponseExponent = 1
	}
	if t.OutputLimit == 0 {
		t.OutputLimit = 1
	}
	t.ConstantGain = Clamp(t.ConstantGain, 0, 1.5)
	t.TransientGain = Clamp(t.TransientGain, 0, 1.5)
	t.Deadband = Clamp(t.Deadband, 0, .20)
	t.MinimumForce = Clamp(t.MinimumForce, 0, .20)
	t.ResponseExponent = Clamp(t.ResponseExponent, .50, 2.00)
	if t.LowPassHz != 0 {
		t.LowPassHz = Clamp(t.LowPassHz, 5, 100)
	}
	t.Smoothing = Clamp(t.Smoothing, 0, .90)
	t.OutputLimit = Clamp(t.OutputLimit, .10, 1)
	return t, nil
}

type FFBPipelineSnapshot struct {
	Enabled                  bool
	RawForce                 float64
	AfterDeadband            float64
	AfterCurve               float64
	AfterMinimumForce        float64
	AfterLowPass             float64
	Output                   float64
	Frames                   uint64
	InputClipEvents          uint64
	OutputClipEvents         uint64
	DeadbandHits             uint64
	MinimumForceApplications uint64
	LastSampleLatency        time.Duration
	AverageSampleLatency     time.Duration
	MaxSampleLatency         time.Duration
	LastProcessedAt          time.Time
	Config                   FFBTuning
}

type FFBPipeline struct {
	mu  sync.Mutex
	cfg FFBTuning

	haveState bool
	lastAt    time.Time
	lowPass   float64
	output    float64

	snap       FFBPipelineSnapshot
	latencySum time.Duration
}

func NewFFBPipeline(cfg FFBTuning) (*FFBPipeline, error) {
	var err error
	cfg, err = NormalizeFFBTuning(cfg)
	if err != nil {
		return nil, err
	}
	p := &FFBPipeline{cfg: cfg}
	p.snap.Enabled = cfg.Enabled
	p.snap.Config = cfg
	return p, nil
}

func (p *FFBPipeline) SetTuning(cfg FFBTuning) error {
	cfg, err := NormalizeFFBTuning(cfg)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = cfg
	p.haveState = false
	p.lastAt = time.Time{}
	p.lowPass = 0
	p.output = 0
	p.snap = FFBPipelineSnapshot{Enabled: cfg.Enabled, Config: cfg}
	p.latencySum = 0
	return nil
}

func (p *FFBPipeline) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	cfg := p.cfg
	p.haveState = false
	p.lastAt = time.Time{}
	p.lowPass = 0
	p.output = 0
	p.snap = FFBPipelineSnapshot{Enabled: cfg.Enabled, Config: cfg}
	p.latencySum = 0
}

func signedMagnitude(signSource, mag float64) float64 {
	if signSource < 0 {
		return -mag
	}
	return mag
}

// ProcessFrame shapes a semantic FFB frame but does not apply any hardware
// safety ceiling. Callers must still run their final output through the hard
// per-wheel safety mixer before converting to device packets.
func (p *FFBPipeline) ProcessFrame(frame ForceFrame, masterGain float64, sampleAt, now time.Time) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	cfg := p.cfg
	if now.IsZero() {
		now = time.Now()
	}
	if math.IsNaN(masterGain) || math.IsInf(masterGain, 0) {
		masterGain = 1
	}
	rawUnclamped := (frame.Constant*cfg.ConstantGain + frame.Transient*cfg.TransientGain) * masterGain
	raw := Clamp(rawUnclamped, -1, 1)
	if raw != rawUnclamped {
		p.snap.InputClipEvents++
	}
	if !cfg.Enabled {
		p.recordLocked(raw, raw, raw, raw, raw, Clamp(raw, -cfg.OutputLimit, cfg.OutputLimit), sampleAt, now)
		return p.snap.Output
	}

	// Symmetric deadband with range re-scaling preserves full-scale output.
	absRaw := math.Abs(raw)
	dead := 0.0
	if absRaw <= cfg.Deadband {
		if absRaw > 0 {
			p.snap.DeadbandHits++
		}
	} else if cfg.Deadband < 1 {
		dead = signedMagnitude(raw, (absRaw-cfg.Deadband)/(1-cfg.Deadband))
	}

	curved := 0.0
	if dead != 0 {
		curved = signedMagnitude(dead, math.Pow(math.Abs(dead), cfg.ResponseExponent))
	}

	minForced := curved
	if curved != 0 && cfg.MinimumForce > 0 {
		mag := cfg.MinimumForce + math.Abs(curved)*(1-cfg.MinimumForce)
		minForced = signedMagnitude(curved, mag)
		p.snap.MinimumForceApplications++
	}

	low := minForced
	if cfg.LowPassHz > 0 && p.haveState {
		dt := now.Sub(p.lastAt).Seconds()
		if dt <= 0 || dt > .5 {
			dt = 1.0 / float64(TargetHz)
		}
		rc := 1.0 / (2 * math.Pi * cfg.LowPassHz)
		alpha := dt / (rc + dt)
		low = p.lowPass + alpha*(minForced-p.lowPass)
	}
	p.lowPass = low

	out := low
	if cfg.Smoothing > 0 && p.haveState {
		out = p.output*cfg.Smoothing + low*(1-cfg.Smoothing)
	}
	limited := Clamp(out, -cfg.OutputLimit, cfg.OutputLimit)
	if limited != out {
		p.snap.OutputClipEvents++
	}
	p.output = limited
	p.haveState = true
	p.lastAt = now
	p.recordLocked(raw, dead, curved, minForced, low, limited, sampleAt, now)
	return limited
}

func (p *FFBPipeline) recordLocked(raw, dead, curved, minForced, low, out float64, sampleAt, now time.Time) {
	p.snap.Enabled = p.cfg.Enabled
	p.snap.Config = p.cfg
	p.snap.RawForce = raw
	p.snap.AfterDeadband = dead
	p.snap.AfterCurve = curved
	p.snap.AfterMinimumForce = minForced
	p.snap.AfterLowPass = low
	p.snap.Output = out
	p.snap.Frames++
	p.snap.LastProcessedAt = now
	if !sampleAt.IsZero() && !now.Before(sampleAt) {
		lat := now.Sub(sampleAt)
		p.snap.LastSampleLatency = lat
		p.latencySum += lat
		p.snap.AverageSampleLatency = time.Duration(int64(p.latencySum) / int64(p.snap.Frames))
		if lat > p.snap.MaxSampleLatency {
			p.snap.MaxSampleLatency = lat
		}
	}
}

func (p *FFBPipeline) Snapshot() FFBPipelineSnapshot {
	if p == nil {
		return FFBPipelineSnapshot{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.snap
}
