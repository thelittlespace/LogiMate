package openg27port

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

type memoryTransport struct {
	mu     sync.Mutex
	writes [][]byte
}

func (m *memoryTransport) Write(r []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes = append(m.writes, append([]byte(nil), r...))
	return nil
}
func (m *memoryTransport) last() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.writes) == 0 {
		return nil
	}
	return append([]byte(nil), m.writes[len(m.writes)-1]...)
}

type staticSource struct {
	frame    ForceFrame
	hasFrame bool
	started  bool
	stopped  bool
}

func (s *staticSource) Start()                          { s.started = true }
func (s *staticSource) Stop()                           { s.stopped = true }
func (s *staticSource) TryGetFrame() (ForceFrame, bool) { return s.frame, s.hasFrame }

func TestWheelOutputSetForceMatchesOpenG27(t *testing.T) {
	now := time.Unix(0, 0)
	transport := &memoryTransport{}
	out, err := NewWheelOutput(transport, WheelOutputOptions{MaxSlewPerTick: 1, Watchdog: time.Second}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err := out.SetForce(0.5); err != nil {
		t.Fatal(err)
	}
	want := ConstantForce(TorqueToWheelByte(0.5))
	if got := transport.last(); !bytes.Equal(got, want) {
		t.Fatalf("got % X want % X", got, want)
	}
}

func TestWheelOutputSlewAndWatchdog(t *testing.T) {
	now := time.Unix(0, 0)
	transport := &memoryTransport{}
	out, err := NewWheelOutput(transport, WheelOutputOptions{MaxSlewPerTick: 0.1, Watchdog: 100 * time.Millisecond}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	_ = out.SetForce(1)
	if got := out.Snapshot().CurrentForce; got < 0.099 || got > 0.101 {
		t.Fatalf("slew got %f", got)
	}
	now = now.Add(101 * time.Millisecond)
	_ = out.Tick()
	if got := out.Snapshot().CurrentForce; got != 0 {
		t.Fatalf("watchdog force=%f", got)
	}
	if out.Snapshot().WatchdogTrips != 1 {
		t.Fatalf("watchdogTrips=%d", out.Snapshot().WatchdogTrips)
	}
}

func TestEnginePumpOnceAndMasterGain(t *testing.T) {
	now := time.Unix(0, 0)
	transport := &memoryTransport{}
	out, _ := NewWheelOutput(transport, WheelOutputOptions{MaxSlewPerTick: 1, Watchdog: time.Second}, func() time.Time { return now })
	engine, _ := NewFFBEngine(out)
	source := &staticSource{frame: ForceFrame{Constant: 0.5}, hasFrame: true}
	engine.SetSource(source)
	engine.SetMasterGain(0.5)
	if err := engine.PumpOnce(); err != nil {
		t.Fatal(err)
	}
	want := ConstantForce(TorqueToWheelByte(0.25))
	if got := transport.last(); !bytes.Equal(got, want) {
		t.Fatalf("got % X want % X", got, want)
	}
	st := engine.Snapshot()
	if st.Pumps != 1 || st.SourceFrames != 1 {
		t.Fatalf("snapshot=%+v", st)
	}
}

func TestEnginePumpWithoutSourceStillTicks(t *testing.T) {
	transport := &memoryTransport{}
	out, _ := NewWheelOutput(transport, WheelOutputOptions{MaxSlewPerTick: 1, Watchdog: time.Second}, nil)
	engine, _ := NewFFBEngine(out)
	if err := engine.PumpOnce(); err != nil {
		t.Fatal(err)
	}
	if got := transport.last(); !bytes.Equal(got, ConstantForce(0x80)) {
		t.Fatalf("got % X", got)
	}
}

func TestEngineStopAlwaysCentersAndStopsSource(t *testing.T) {
	transport := &memoryTransport{}
	out, _ := NewWheelOutput(transport, WheelOutputOptions{MaxSlewPerTick: 0.05, Watchdog: time.Second}, nil)
	engine, _ := NewFFBEngine(out)
	source := &staticSource{frame: ForceFrame{Constant: 1}, hasFrame: true}
	engine.SetSource(source)
	for i := 0; i < 25; i++ {
		_ = engine.PumpOnce()
	}
	if err := engine.Stop(); err != nil {
		t.Fatal(err)
	}
	if !source.stopped {
		t.Fatal("source Stop not called")
	}
	if got := transport.last(); !bytes.Equal(got, ConstantForce(0x80)) {
		t.Fatalf("last=% X", got)
	}
	if out.Snapshot().CurrentForce != 0 {
		t.Fatalf("force=%f", out.Snapshot().CurrentForce)
	}
}

func TestEngineStartIsSingleOwnerAndStops(t *testing.T) {
	transport := &memoryTransport{}
	out, _ := NewWheelOutput(transport, WheelOutputOptions{MaxSlewPerTick: 1, Watchdog: time.Second}, nil)
	engine, _ := NewFFBEngine(out)
	source := &staticSource{frame: ForceFrame{Constant: 0.2}, hasFrame: true}
	engine.SetSource(source)
	if !engine.Start() {
		t.Fatal("first Start false")
	}
	if engine.Start() {
		t.Fatal("second Start true")
	}
	time.Sleep(25 * time.Millisecond)
	if err := engine.Stop(); err != nil {
		t.Fatal(err)
	}
	if !source.started || !source.stopped {
		t.Fatalf("lifecycle started=%v stopped=%v", source.started, source.stopped)
	}
	if engine.Snapshot().Running {
		t.Fatal("still running")
	}
	if engine.Snapshot().Pumps == 0 {
		t.Fatal("no pumps")
	}
}
