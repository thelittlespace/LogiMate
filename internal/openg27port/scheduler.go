package openg27port

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// SchedulerTick reproduces OpenG27 FfbEngine's current integer-millisecond
// cadence: 1000 / 150 == 6 ms. This is deliberately parity-first; a more exact
// 150 Hz scheduler belongs to LogiMate's later D-phase optimization work.
const SchedulerTick = 6 * time.Millisecond

const TargetHz = 150

// WheelTransport is the hardware-agnostic sink used by the ported OpenG27
// wheel-output layer. Reports are the seven lg4ff command bytes, without the
// Windows HID report-ID byte, matching OpenG27's IWheelTransport contract.
type WheelTransport interface {
	Write(report7 []byte) error
}

// FFBSource is the Go equivalent of OpenG27.Core.IFfbSource.
type FFBSource interface {
	Start()
	Stop()
	TryGetFrame() (ForceFrame, bool)
}

// WheelOutputOptions mirrors OpenG27.Core.WheelOutputOptions.
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

// WheelOutput is a behavioral Go port of OpenG27.Core.WheelOutput. LogiMate's
// system layer adds the stricter per-wheel safety policy and ownership gates.
type WheelOutput struct {
	mu        sync.Mutex
	transport WheelTransport
	options   WheelOutputOptions
	clock     func() time.Time

	currentForce  float64
	lastUpdate    time.Time
	lastWrite     time.Time
	writes        uint64
	slewLimited   uint64
	watchdogTrips uint64
	lastError     string
}

func NewWheelOutput(transport WheelTransport, options WheelOutputOptions, clock func() time.Time) (*WheelOutput, error) {
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

// Init preserves OpenG27's initialization sequence. C3 does not call this on
// live LogiMate hardware because LogiMate already owns native-mode/range policy.
func (w *WheelOutput) Init() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.writeLocked(NativeSwitch()); err != nil {
		return err
	}
	if err := w.writeLocked(SetRange(w.options.DefaultRangeDeg)); err != nil {
		return err
	}
	return w.writeLocked(SpringOff())
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

// SetForce clamps, slew-limits, records the source heartbeat and immediately
// emits one constant-force report, matching OpenG27 WheelOutput.SetForce.
func (w *WheelOutput) SetForce(torque float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.currentForce = w.stepTowardLocked(torque)
	w.lastUpdate = w.clock()
	return w.writeCurrentForceLocked()
}

// Tick re-sends the current force. If the source stopped updating beyond the
// watchdog, force is snapped to center before the write, matching OpenG27.
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

// Panic sends the global stop command and then fully converges the constant
// force back to neutral. The final neutral write is unconditional.
func (w *WheelOutput) Panic() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var errs []error
	if err := w.writeLocked(Stop()); err != nil {
		errs = append(errs, err)
	}
	const maxSteps = 100000
	for i := 0; i < maxSteps && w.currentForce != 0; i++ {
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
	return WheelOutputSnapshot{
		CurrentForce:  w.currentForce,
		Writes:        w.writes,
		SlewLimited:   w.slewLimited,
		WatchdogTrips: w.watchdogTrips,
		LastUpdate:    w.lastUpdate,
		LastWrite:     w.lastWrite,
		LastError:     w.lastError,
	}
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
	Running      bool
	Pumps        uint64
	SourceFrames uint64
	LastPump     time.Time
	LastError    string
	MasterGain   float64
}

// FFBEngine ports OpenG27.Core.FfbEngine while retaining deterministic
// PumpOnce for tests/shadow mode. Later LogiMate D-phase work may replace the
// 6 ms parity cadence with a measured/adaptive scheduler.
type FFBEngine struct {
	mu sync.Mutex

	output     *WheelOutput
	source     FFBSource
	masterGain float64

	running      bool
	cancel       chan struct{}
	done         chan struct{}
	pumps        uint64
	sourceFrames uint64
	lastPump     time.Time
	lastError    string
}

func NewFFBEngine(output *WheelOutput) (*FFBEngine, error) {
	if output == nil {
		return nil, errors.New("wheel output is required")
	}
	return &FFBEngine{output: output, masterGain: 1}, nil
}

func (e *FFBEngine) SetSource(source FFBSource) {
	e.mu.Lock()
	e.source = source
	e.mu.Unlock()
}

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
	source := e.source
	gain := e.masterGain
	e.mu.Unlock()

	var err error
	gotFrame := false
	if source != nil {
		if frame, ok := source.TryGetFrame(); ok {
			gotFrame = true
			torque := ResolveForceFrame(frame, gain)
			if setErr := e.output.SetForce(torque); setErr != nil {
				err = setErr
			}
		}
	}
	if tickErr := e.output.Tick(); tickErr != nil && err == nil {
		err = tickErr
	}

	e.mu.Lock()
	e.pumps++
	if gotFrame {
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

// Start starts at most one scheduler goroutine. It returns false when already
// running, matching OpenG27's no-op-on-second-Start behavior.
func (e *FFBEngine) Start() bool {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return false
	}
	e.running = true
	e.cancel = make(chan struct{})
	e.done = make(chan struct{})
	cancel := e.cancel
	done := e.done
	source := e.source
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
	wasRunning := e.running
	cancel := e.cancel
	done := e.done
	source := e.source
	if e.running {
		e.running = false
		e.cancel = nil
		e.done = nil
		close(cancel)
	}
	e.mu.Unlock()

	if wasRunning && done != nil {
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
	return FFBEngineSnapshot{
		Running:      e.running,
		Pumps:        e.pumps,
		SourceFrames: e.sourceFrames,
		LastPump:     e.lastPump,
		LastError:    e.lastError,
		MasterGain:   e.masterGain,
	}
}
