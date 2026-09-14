package wheelengine

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestD1ModelAdaptersMatchDescriptors(t *testing.T) {
	for _, m := range []ModelID{ModelG25, ModelG27, ModelDFGT} {
		if err := ValidateModelAdapter(m); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
		a, _ := AdapterForModel(m)
		d, _ := DescriptorForModel(m)
		if a.NativePID != d.NativePID || a.ModeSelector != d.NativeModeSelector {
			t.Fatalf("%s identity mismatch", m)
		}
	}
}

func TestD1CertificationCannotPassWithoutEvidence(t *testing.T) {
	for _, m := range []ModelID{ModelG25, ModelG27, ModelDFGT} {
		if (ModelCertification{Model: m}).Complete() {
			t.Fatalf("%s certified without evidence", m)
		}
	}
}

func TestD2NativeProtocolGoldenVectors(t *testing.T) {
	vectors := []struct{ got, want []byte }{
		{ConstantForce(0x80), []byte{0x11, 0x08, 0x80, 0x80, 0, 0, 0}},
		{SetRange(900), []byte{0xF8, 0x81, 0x84, 0x03, 0, 0, 0}},
		{NativeSwitch(0x04), []byte{0xF8, 0x09, 0x04, 0x01, 0, 0, 0}},
		{SetLeds(0x1F), []byte{0xF8, 0x12, 0x1F, 0, 0, 0, 0}},
	}
	for i, v := range vectors {
		if !bytes.Equal(v.got, v.want) {
			t.Fatalf("vector %d got % X want % X", i, v.got, v.want)
		}
	}
}

func TestD2SchedulerWatchdogCenters(t *testing.T) {
	tr := &memoryFFBTransport{}
	now := time.Unix(10, 0)
	out, err := NewWheelOutput(tr, WheelOutputOptions{MaxSlewPerTick: 1, Watchdog: 100 * time.Millisecond}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	src := &testFFBSource{frame: ForceFrame{Constant: .5}, active: true}
	eng, _ := NewFFBEngine(out)
	eng.SetSource(src)
	eng.SetMasterGain(.5)
	if err := eng.PumpOnce(); err != nil {
		t.Fatal(err)
	}
	src.active = false
	now = now.Add(101 * time.Millisecond)
	if err := eng.PumpOnce(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tr.last(), ConstantForce(0x80)) {
		t.Fatalf("not neutral: % X", tr.last())
	}
}

type memoryFFBTransport struct{ writes [][]byte }

func (m *memoryFFBTransport) Write(b []byte) error {
	m.writes = append(m.writes, append([]byte(nil), b...))
	return nil
}
func (m *memoryFFBTransport) last() []byte {
	if len(m.writes) == 0 {
		return nil
	}
	return m.writes[len(m.writes)-1]
}

type testFFBSource struct {
	frame  ForceFrame
	active bool
}

func (s *testFFBSource) Start()                          { s.active = true }
func (s *testFFBSource) Stop()                           { s.active = false }
func (s *testFFBSource) TryGetFrame() (ForceFrame, bool) { return s.frame, s.active }

func TestD2WreckfestPinoParser(t *testing.T) {
	b := make([]byte, WreckfestPinoMainPacketLength)
	binary.LittleEndian.PutUint32(b[0:4], WreckfestPinoSignature)
	binary.LittleEndian.PutUint32(b[435:439], 1109)
	binary.LittleEndian.PutUint32(b[439:443], 5600)
	binary.LittleEndian.PutUint32(b[443:447], 5320)
	binary.LittleEndian.PutUint16(b[1090:1092], 1<<2|1<<3)
	b[1092] = 1
	binary.LittleEndian.PutUint32(b[1093:1097], 0x3f000000)
	f, ok := ParseWreckfestPinoMain(b)
	if !ok || f.RPM != 1109 || !f.PhysicsRunning() || !f.PlayerInControl() || !f.FFBEnabled {
		t.Fatalf("bad parse: %+v ok=%v", f, ok)
	}
}
