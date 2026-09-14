package wheelengine

import (
	"encoding/binary"
	"math"
)

const WreckfestPinoSignature uint32 = 1869769584
const WreckfestPinoMainPacketLength = 1218

type WreckfestTelemetry struct {
	RPM, RPMMax, RPMRedline, RPMIdle int
	FFBForce                         float32
	FFBEnabled                       bool
	PlayerStatusFlags                uint16
}

func (w WreckfestTelemetry) InRace() bool          { return w.PlayerStatusFlags&(1<<0) != 0 }
func (w WreckfestTelemetry) PhysicsRunning() bool  { return w.PlayerStatusFlags&(1<<2) != 0 }
func (w WreckfestTelemetry) PlayerInControl() bool { return w.PlayerStatusFlags&(1<<3) != 0 }
func (w WreckfestTelemetry) AIInControl() bool     { return w.PlayerStatusFlags&(1<<4) != 0 }
func ParseWreckfestPinoMain(b []byte) (WreckfestTelemetry, bool) {
	if len(b) < WreckfestPinoMainPacketLength || binary.LittleEndian.Uint32(b[0:4]) != WreckfestPinoSignature || b[4] != 0 {
		return WreckfestTelemetry{}, false
	}
	f := WreckfestTelemetry{
		RPM: int(int32(binary.LittleEndian.Uint32(b[435:439]))), RPMMax: int(int32(binary.LittleEndian.Uint32(b[439:443]))),
		RPMRedline: int(int32(binary.LittleEndian.Uint32(b[443:447]))), RPMIdle: int(int32(binary.LittleEndian.Uint32(b[447:451]))),
		PlayerStatusFlags: binary.LittleEndian.Uint16(b[1090:1092]), FFBEnabled: b[1092]&1 != 0,
		FFBForce: math.Float32frombits(binary.LittleEndian.Uint32(b[1093:1097])),
	}
	if math.IsNaN(float64(f.FFBForce)) || math.IsInf(float64(f.FFBForce), 0) {
		return WreckfestTelemetry{}, false
	}
	return f, true
}
