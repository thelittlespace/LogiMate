package wheelengine

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// SchedulerTick intentionally retains the proven 6 ms Path-C cadence for D2.
// D4 may replace this with an adaptive/measured cadence after hardware data.
const SchedulerTick = 6 * time.Millisecond
const TargetHz = 150

type FFBTransport interface{ Write(report7 []byte) error }
type FFBSource interface {
	Start()
	Stop()
	TryGetFrame() (ForceFrame, bool)
}

type WheelOutputOptions struct {
	MaxSlewPerTick  float64
	Watchdog        time.Duration
	DefaultRangeDeg int
}

func (o WheelOutputOptions) normalized() (WheelOutputOptions, error) {
	if o.MaxSlewPerTick <= 0 || math.IsNaN(o.MaxSlewPerTick) || math.IsInf(o.MaxSlewPerTick, 0) {
		return o, fmt.Errorf("MaxSlewPerTick must be strictly positive")
	}
	if o.Watchdog <= 0 {
		return o, fmt.Errorf("Watchdog must be strictly positive")
	}
	if o.DefaultRangeDeg == 0 {
		o.DefaultRangeDeg = 900
	}
	return o, nil
}

type WheelOutputSnapshot struct {
	CurrentForce  float64
	Writes        uint64
	SlewLimited   uint64
	WatchdogTrips uint64
	LastUpdate    time.Time
	LastWrite     time.Time
	LastError     string
}

type WheelOutput struct {
	mu                                 sync.Mutex
	transport                          FFBTransport
	options                            WheelOutputOptions
	clock                              func() time.Time
	currentForce                       float64
	lastUpdate, lastWrite              time.Time
	writes, slewLimited, watchdogTrips uint64
	lastError                          string
}

func NewWheelOutput(transport FFBTransport, options WheelOutputOptions, clock func() time.Time) (*WheelOutput, error) {
	if transport == nil {
		return nil, errors.New("wheel transport is required")
	}
	var err error
	options, err = options.normalized()
	if err != nil {
		return nil, err
	}
	if clock == nil {
		clock = time.Now
	}
	now := clock()
	return &WheelOutput{transport: transport, options: options, clock: clock, lastUpdate: now}, nil
}
func (w *WheelOutput) SetRange(deg int) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writeLocked(SetRange(deg))
}
func (w *WheelOutput) stepTowardLocked(target float64) float64 {
	target = Clamp(target, -1, 1)
	delta := Clamp(target-w.currentForce, -w.options.MaxSlewPerTick, w.options.MaxSlewPerTick)
	if math.Abs(target-w.currentForce) > w.options.MaxSlewPerTick+1e-12 {
		w.slewLimited++
	}
	next := w.currentForce + delta
	if math.Abs(next-target) < 1e-9 {
		return target
	}
	return next
}
func (w *WheelOutput) SetForce(torque float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.currentForce = w.stepTowardLocked(torque)
	w.lastUpdate = w.clock()
	return w.writeCurrentForceLocked()
}
func (w *WheelOutput) Tick() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.clock().Sub(w.lastUpdate) > w.options.Watchdog {
		if w.currentForce != 0 {
			w.watchdogTrips++
		}
		w.currentForce = 0
	}
	return w.writeCurrentForceLocked()
}
func (w *WheelOutput) Panic() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var errs []error
	if err := w.writeLocked(StopAllEffects()); err != nil {
		errs = append(errs, err)
	}
	for i := 0; i < 100000 && w.currentForce != 0; i++ {
		w.currentForce = w.stepTowardLocked(0)
		if err := w.writeCurrentForceLocked(); err != nil {
			errs = append(errs, err)
			break
		}
	}
	w.currentForce = 0
	if err := w.writeCurrentForceLocked(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
func (w *WheelOutput) Snapshot() WheelOutputSnapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	return WheelOutputSnapshot{w.currentForce, w.writes, w.slewLimited, w.watchdogTrips, w.lastUpdate, w.lastWrite, w.lastError}
}
func (w *WheelOutput) writeCurrentForceLocked() error {
	return w.writeLocked(ConstantForce(TorqueToWheelByte(w.currentForce)))
}
func (w *WheelOutput) writeLocked(report []byte) error {
	err := w.transport.Write(report)
	w.lastWrite = w.clock()
	w.writes++
	if err != nil {
		w.lastError = err.Error()
	} else {
		w.lastError = ""
	}
	return err
}

type FFBEngineSnapshot struct {
	Running             bool
	Pumps, SourceFrames uint64
	LastPump            time.Time
	LastError           string
	MasterGain          float64
}
type FFBEngine struct {
	mu                  sync.Mutex
	output              *WheelOutput
	source              FFBSource
	masterGain          float64
	running             bool
	cancel, done        chan struct{}
	pumps, sourceFrames uint64
	lastPump            time.Time
	lastError           string
}

func NewFFBEngine(output *WheelOutput) (*FFBEngine, error) {
	if output == nil {
		return nil, errors.New("wheel output is required")
	}
	return &FFBEngine{output: output, masterGain: 1}, nil
}
func (e *FFBEngine) SetSource(source FFBSource) { e.mu.Lock(); e.source = source; e.mu.Unlock() }
func (e *FFBEngine) SetMasterGain(gain float64) {
	if math.IsNaN(gain) || math.IsInf(gain, 0) {
		gain = 1
	}
	e.mu.Lock()
	e.masterGain = gain
	e.mu.Unlock()
}
func (e *FFBEngine) PumpOnce() error {
	e.mu.Lock()
	source, gain := e.source, e.masterGain
	e.mu.Unlock()
	var err error
	got := false
	if source != nil {
		if frame, ok := source.TryGetFrame(); ok {
			got = true
			if x := e.output.SetForce(ResolveForceFrame(frame, gain)); x != nil {
				err = x
			}
		}
	}
	if x := e.output.Tick(); x != nil && err == nil {
		err = x
	}
	e.mu.Lock()
	e.pumps++
	if got {
		e.sourceFrames++
	}
	e.lastPump = time.Now()
	if err != nil {
		e.lastError = err.Error()
	} else {
		e.lastError = ""
	}
	e.mu.Unlock()
	return err
}
func (e *FFBEngine) Start() bool {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return false
	}
	e.running = true
	e.cancel = make(chan struct{})
	e.done = make(chan struct{})
	cancel, done, source := e.cancel, e.done, e.source
	e.mu.Unlock()
	if source != nil {
		source.Start()
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(SchedulerTick)
		defer ticker.Stop()
		for {
			_ = e.PumpOnce()
			select {
			case <-cancel:
				return
			case <-ticker.C:
			}
		}
	}()
	return true
}
func (e *FFBEngine) Stop() error {
	e.mu.Lock()
	was := e.running
	cancel, done, source := e.cancel, e.done, e.source
	if e.running {
		e.running = false
		e.cancel = nil
		e.done = nil
		close(cancel)
	}
	e.mu.Unlock()
	if was && done != nil {
		<-done
	}
	if source != nil {
		source.Stop()
	}
	return e.output.Panic()
}
func (e *FFBEngine) Snapshot() FFBEngineSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return FFBEngineSnapshot{e.running, e.pumps, e.sourceFrames, e.lastPump, e.lastError, e.masterGain}
}
