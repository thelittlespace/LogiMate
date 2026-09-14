package wheelengine

import "math"

// ForceFrame is the canonical LogiMate-native FFB frame. The semantics were
// originally validated against the MIT-licensed OpenG27 reference port, but
// production runtime code now consumes this type directly from wheelengine.
type ForceFrame struct {
	Constant  float64
	Spring    float64
	Damper    float64
	Transient float64
}

func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func ResolveForceFrame(f ForceFrame, masterGain float64) float64 {
	return Clamp((f.Constant+f.Transient)*masterGain, -1, 1)
}

func TorqueToWheelByte(torque float64) byte {
	v := int(math.RoundToEven(128 + Clamp(torque, -1, 1)*127))
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return byte(v)
}

func SliderToForceByte(percent int) byte {
	if percent < -100 {
		percent = -100
	}
	if percent > 100 {
		percent = 100
	}
	var scaled float64
	if percent >= 0 {
		scaled = 128 + float64(percent)/100*127
	} else {
		scaled = 128 + float64(percent)/100*128
	}
	v := int(math.RoundToEven(scaled))
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return byte(v)
}

func ConstantForce(force byte) []byte   { return []byte{0x11, 0x08, force, 0x80, 0, 0, 0} }
func StopAllEffects() []byte            { return []byte{0xF3, 0, 0, 0, 0, 0, 0} }
func SetRange(deg int) []byte           { return []byte{0xF8, 0x81, byte(deg), byte(deg >> 8), 0, 0, 0} }
func NativeSwitch(selector byte) []byte { return []byte{0xF8, 0x09, selector, 0x01, 0, 0, 0} }
func SpringSet(slope, clip byte) []byte { return []byte{0xFE, 0x0D, slope, slope, clip, 0, 0} }
func SpringEnable() []byte              { return []byte{0x14, 0, 0, 0, 0, 0, 0} }
func SpringOff() []byte                 { return []byte{0xF5, 0, 0, 0, 0, 0, 0} }
func Damper(coeff, clip byte) []byte {
	if coeff > 15 {
		coeff = 15
	}
	return []byte{0x41, 0x0C, coeff, 0, coeff, 0, clip}
}
func DamperOff() []byte                { return []byte{0x43, 0, 0, 0, 0, 0, 0} }
func Friction(coeff, clip byte) []byte { return []byte{0x81, 0x0E, coeff, coeff, clip, 0, 0} }
func FrictionOff() []byte              { return []byte{0x83, 0, 0, 0, 0, 0, 0} }
func SetLeds(mask byte) []byte         { return []byte{0xF8, 0x12, mask, 0, 0, 0, 0} }
func LedsOff() []byte                  { return SetLeds(0) }

func LedBarForPercent(percent int) byte {
	switch {
	case percent < 8:
		return 0
	case percent < 25:
		return 0x01
	case percent < 50:
		return 0x03
	case percent < 75:
		return 0x07
	case percent < 90:
		return 0x0F
	default:
		return 0x1F
	}
}

func WithReportID(command []byte) []byte {
	report := make([]byte, len(command)+1)
	copy(report[1:], command)
	return report
}
