package openg27port

import (
	"encoding/binary"
	"math"
)

const (
	PinoSignature        uint32 = 1869769584
	PinoMainPacketLength        = 1218
)

type Wf2Telemetry struct {
	RPM               int
	RPMMax            int
	RPMRedline        int
	RPMIdle           int
	FFBForce          float32
	FFBEnabled        bool
	PlayerStatusFlags uint16
}

func (w Wf2Telemetry) InRace() bool          { return w.PlayerStatusFlags&(1<<0) != 0 }
func (w Wf2Telemetry) PhysicsRunning() bool  { return w.PlayerStatusFlags&(1<<2) != 0 }
func (w Wf2Telemetry) PlayerInControl() bool { return w.PlayerStatusFlags&(1<<3) != 0 }
func (w Wf2Telemetry) AIInControl() bool     { return w.PlayerStatusFlags&(1<<4) != 0 }

func ParsePinoMain(b []byte) (Wf2Telemetry, bool) {
	if len(b) < PinoMainPacketLength {
		return Wf2Telemetry{}, false
	}
	if binary.LittleEndian.Uint32(b[0:4]) != PinoSignature || b[4] != 0 {
		return Wf2Telemetry{}, false
	}
	f := Wf2Telemetry{
		RPM:               int(int32(binary.LittleEndian.Uint32(b[435:439]))),
		RPMMax:            int(int32(binary.LittleEndian.Uint32(b[439:443]))),
		RPMRedline:        int(int32(binary.LittleEndian.Uint32(b[443:447]))),
		RPMIdle:           int(int32(binary.LittleEndian.Uint32(b[447:451]))),
		PlayerStatusFlags: binary.LittleEndian.Uint16(b[1090:1092]),
		FFBEnabled:        b[1092]&1 != 0,
		FFBForce:          math.Float32frombits(binary.LittleEndian.Uint32(b[1093:1097])),
	}
	return f, true
}
