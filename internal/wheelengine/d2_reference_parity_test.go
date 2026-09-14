package wheelengine_test

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/thelittlespace/LogiMate/internal/openg27port"
	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

func TestD2ProtocolMatchesReferenceOracle(t *testing.T) {
	pairs := []struct {
		name              string
		native, reference []byte
	}{
		{"constant", wheelengine.ConstantForce(0x91), openg27port.ConstantForce(0x91)},
		{"range", wheelengine.SetRange(900), openg27port.SetRange(900)},
		{"native-g27", wheelengine.NativeSwitch(0x04), openg27port.NativeSwitch()},
		{"spring-set", wheelengine.SpringSet(7, 128), openg27port.SpringSet(7, 128)},
		{"spring-enable", wheelengine.SpringEnable(), openg27port.SpringEnable()},
		{"spring-off", wheelengine.SpringOff(), openg27port.SpringOff()},
		{"leds", wheelengine.SetLeds(0x1f), openg27port.SetLeds(0x1f)},
	}
	for _, p := range pairs {
		if !bytes.Equal(p.native, p.reference) {
			t.Fatalf("%s native=% X reference=% X", p.name, p.native, p.reference)
		}
	}
	for _, v := range []int{-100, -50, 0, 50, 100} {
		if wheelengine.SliderToForceByte(v) != openg27port.SliderToForceByte(v) {
			t.Fatalf("slider %d differs", v)
		}
	}
	for _, v := range []float64{-1, -.5, 0, .5, 1} {
		if wheelengine.TorqueToWheelByte(v) != openg27port.TorqueToWheelByte(v) {
			t.Fatalf("torque %.2f differs", v)
		}
	}
	for _, v := range []int{0, 7, 8, 24, 25, 49, 50, 74, 75, 89, 90, 100} {
		if wheelengine.LedBarForPercent(v) != openg27port.LedBarForPercent(v) {
			t.Fatalf("led %d differs", v)
		}
	}
}

func TestHardwareConfirmedConditionSlots(t *testing.T) {
	cases := []struct {
		name      string
		got, want []byte
	}{
		{"damper-slot-2", wheelengine.Damper(15, 128), []byte{0x41, 0x0C, 0x0F, 0x00, 0x0F, 0x00, 0x80}},
		{"damper-slot-2-stop", wheelengine.DamperOff(), []byte{0x43, 0, 0, 0, 0, 0, 0}},
		{"friction-slot-3", wheelengine.Friction(64, 255), []byte{0x81, 0x0E, 0x40, 0x40, 0xFF, 0, 0}},
		{"friction-slot-3-stop", wheelengine.FrictionOff(), []byte{0x83, 0, 0, 0, 0, 0, 0}},
	}
	for _, tc := range cases {
		if !bytes.Equal(tc.got, tc.want) {
			t.Fatalf("%s got=% X want=% X", tc.name, tc.got, tc.want)
		}
	}
}

func TestD2PinoMatchesReferenceOracle(t *testing.T) {
	b := make([]byte, wheelengine.WreckfestPinoMainPacketLength)
	binary.LittleEndian.PutUint32(b[0:4], wheelengine.WreckfestPinoSignature)
	binary.LittleEndian.PutUint32(b[435:439], 4321)
	binary.LittleEndian.PutUint32(b[439:443], 6000)
	binary.LittleEndian.PutUint32(b[443:447], 5700)
	binary.LittleEndian.PutUint16(b[1090:1092], (1<<2)|(1<<3))
	b[1092] = 1
	binary.LittleEndian.PutUint32(b[1093:1097], 0xbe800000)
	n, nok := wheelengine.ParseWreckfestPinoMain(b)
	r, rok := openg27port.ParsePinoMain(b)
	if !nok || !rok || n.RPM != r.RPM || n.RPMMax != r.RPMMax || n.RPMRedline != r.RPMRedline || n.FFBEnabled != r.FFBEnabled || n.FFBForce != r.FFBForce || n.PlayerStatusFlags != r.PlayerStatusFlags {
		t.Fatalf("native=%+v/%v reference=%+v/%v", n, nok, r, rok)
	}
}

func TestD2SchedulerCoreMatchesReferenceFirstPump(t *testing.T) {
	ntr := &memTransport{}
	rtr := &refMemTransport{}
	now := time.Unix(100, 0)
	clock := func() time.Time { return now }
	no, _ := wheelengine.NewWheelOutput(ntr, wheelengine.WheelOutputOptions{MaxSlewPerTick: 1, Watchdog: 100 * time.Millisecond}, clock)
	ro, _ := openg27port.NewWheelOutput(rtr, openg27port.WheelOutputOptions{MaxSlewPerTick: 1, Watchdog: 100 * time.Millisecond}, clock)
	ns := &nativeSource{active: true, frame: wheelengine.ForceFrame{Constant: .4}}
	rs := &referenceSource{active: true, frame: openg27port.ForceFrame{Constant: .4}}
	ne, _ := wheelengine.NewFFBEngine(no)
	ne.SetSource(ns)
	ne.SetMasterGain(.5)
	re, _ := openg27port.NewFFBEngine(ro)
	re.SetSource(rs)
	re.SetMasterGain(.5)
	if err := ne.PumpOnce(); err != nil {
		t.Fatal(err)
	}
	if err := re.PumpOnce(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ntr.last(), rtr.last()) {
		t.Fatalf("native=% X reference=% X", ntr.last(), rtr.last())
	}
}

type memTransport struct{ x [][]byte }

func (m *memTransport) Write(b []byte) error {
	m.x = append(m.x, append([]byte(nil), b...))
	return nil
}
func (m *memTransport) last() []byte { return m.x[len(m.x)-1] }

type refMemTransport struct{ x [][]byte }

func (m *refMemTransport) Write(b []byte) error {
	m.x = append(m.x, append([]byte(nil), b...))
	return nil
}
func (m *refMemTransport) last() []byte { return m.x[len(m.x)-1] }

type nativeSource struct {
	active bool
	frame  wheelengine.ForceFrame
}

func (s *nativeSource) Start()                                      { s.active = true }
func (s *nativeSource) Stop()                                       { s.active = false }
func (s *nativeSource) TryGetFrame() (wheelengine.ForceFrame, bool) { return s.frame, s.active }

type referenceSource struct {
	active bool
	frame  openg27port.ForceFrame
}

func (s *referenceSource) Start()                                      { s.active = true }
func (s *referenceSource) Stop()                                       { s.active = false }
func (s *referenceSource) TryGetFrame() (openg27port.ForceFrame, bool) { return s.frame, s.active }
