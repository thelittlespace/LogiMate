package openg27port

import "fmt"

// G27RawState is the direct Go port of OpenG27.Core.G27RawState from the
// pinned 1.0.4 source. It intentionally carries only raw wire values.
type G27RawState struct {
	Steering uint16
	Pedal0   byte
	Pedal1   byte
	Pedal2   byte
	Buttons  uint64
}

func (s G27RawState) Throttle() byte { return s.Pedal0 }
func (s G27RawState) Brake() byte    { return s.Pedal1 }
func (s G27RawState) Clutch() byte   { return s.Pedal2 }

const (
	G27ReportMinLength   = 9
	G27ButtonsStartIndex = 9
)

// ParseG27Report ports OpenG27's G27ReportParser.Parse byte-for-byte:
// idx4/5 steering little-endian, idx6/7/8 pedals, idx9+ packed button bits.
func ParseG27Report(report []byte) (G27RawState, error) {
	if report == nil {
		return G27RawState{}, fmt.Errorf("report is nil")
	}
	if len(report) < G27ReportMinLength {
		return G27RawState{}, fmt.Errorf("report too short: expected at least %d bytes, got %d", G27ReportMinLength, len(report))
	}
	steering := uint16(report[4]) | uint16(report[5])<<8
	return G27RawState{
		Steering: steering,
		Pedal0:   report[6],
		Pedal1:   report[7],
		Pedal2:   report[8],
		Buttons:  ExtractG27ButtonBits(report, G27ButtonsStartIndex),
	}, nil
}

// ExtractG27ButtonBits mirrors OpenG27's little-endian packing of idx9+ into
// a 64-bit diagnostic bit mask.
func ExtractG27ButtonBits(report []byte, startIndex int) uint64 {
	if report == nil {
		return 0
	}
	var bits uint64
	bitPos := 0
	for i := startIndex; i >= 0 && i < len(report) && bitPos < 64; i++ {
		bits |= uint64(report[i]) << bitPos
		bitPos += 8
	}
	return bits
}
