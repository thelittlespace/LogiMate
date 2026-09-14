package openg27port

import (
	"encoding/binary"
	"math"
	"testing"
)

func pinoPacket() []byte {
	b := make([]byte, PinoMainPacketLength)
	binary.LittleEndian.PutUint32(b[0:4], PinoSignature)
	b[4] = 0
	binary.LittleEndian.PutUint32(b[435:439], uint32(1109))
	binary.LittleEndian.PutUint32(b[439:443], uint32(5600))
	binary.LittleEndian.PutUint32(b[443:447], uint32(5320))
	binary.LittleEndian.PutUint32(b[447:451], uint32(800))
	binary.LittleEndian.PutUint16(b[1090:1092], 11)
	binary.LittleEndian.PutUint32(b[1093:1097], math.Float32bits(-0.07654980))
	return b
}
func TestPinoParserParity(t *testing.T) {
	d, ok := ParsePinoMain(pinoPacket())
	if !ok {
		t.Fatal("expected Main packet")
	}
	if d.RPM != 1109 || d.RPMMax != 5600 || d.RPMRedline != 5320 || d.RPMIdle != 800 || d.PlayerStatusFlags != 11 {
		t.Fatalf("bad decode: %+v", d)
	}
	if math.Abs(float64(d.FFBForce-(-0.07654980))) > 0.00001 {
		t.Fatalf("bad FFB: %f", d.FFBForce)
	}
	if !d.PlayerInControl() || d.AIInControl() || d.PhysicsRunning() {
		t.Fatalf("flags parity: %+v", d)
	}
	b := pinoPacket()
	b[1092] = 1
	d, ok = ParsePinoMain(b)
	if !ok || !d.FFBEnabled {
		t.Fatal("FFB flag")
	}
	b = pinoPacket()
	b[4] = 1
	if _, ok = ParsePinoMain(b); ok {
		t.Fatal("non-Main accepted")
	}
	if _, ok = ParsePinoMain(make([]byte, PinoMainPacketLength-1)); ok {
		t.Fatal("short accepted")
	}
}
