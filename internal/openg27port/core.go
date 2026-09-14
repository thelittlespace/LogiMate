// Package openg27port contains hardware-independent logic ported from the
// MIT-licensed Jabelius/OpenG27 project as part of LogiMate's C -> D migration.
//
// Provenance is documented in docs/OPENG27_PROVENANCE.md. This package is kept
// UI- and Windows-independent on purpose so its behavior can be regression
// tested on every build host before any translated code is allowed near HID.
package openg27port

import "math"

// ForceFrame mirrors OpenG27.Core.ForceFrame. Spring and Damper are retained in
// the data model even though OpenG27's current Resolve implementation combines
// Constant + Transient for the single torque output path.
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

// ResolveForceFrame is a direct behavioral port of OpenG27 Core/ForceMixer.cs:
// sum constant + transient, apply master gain and clamp to [-1, +1].
func ResolveForceFrame(f ForceFrame, masterGain float64) float64 {
	return Clamp((f.Constant+f.Transient)*masterGain, -1.0, 1.0)
}

// TorqueToWheelByte mirrors OpenG27 ForceMixer.ToWheelByte. Math.Round in C#
// uses midpoint-to-even by default, so math.RoundToEven is required for byte
// parity instead of Go's ordinary math.Round.
func TorqueToWheelByte(torque float64) byte {
	v := int(math.RoundToEven(128.0 + Clamp(torque, -1.0, 1.0)*127.0))
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return byte(v)
}

// SliderToForceByte ports OpenG27 App/Logic/ForceScaling.cs exactly, including
// the intentional asymmetric scale: -100 -> 0x00, 0 -> 0x80, +100 -> 0xFF.
func SliderToForceByte(percent int) byte {
	if percent < -100 {
		percent = -100
	}
	if percent > 100 {
		percent = 100
	}
	var scaled float64
	if percent >= 0 {
		scaled = 128.0 + float64(percent)/100.0*127.0
	} else {
		scaled = 128.0 + float64(percent)/100.0*128.0
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

// SliderToForceByteWithGain scales the deviation from 0x80 by masterGain,
// matching OpenG27's overload of ForceScaling.SliderToForceByte.
func SliderToForceByteWithGain(percent int, masterGain float64) byte {
	base := int(SliderToForceByte(percent))
	deviation := base - 128
	v := 128 + int(math.RoundToEven(float64(deviation)*masterGain))
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return byte(v)
}

// CenteredAxisCalibration is the hardware-independent OpenG27 steering
// calibration model. Endpoints may be asymmetric or numerically inverted.
type CenteredAxisCalibration struct {
	Left   int
	Center int
	Right  int
}

func DefaultCenteredAxisCalibration() CenteredAxisCalibration {
	return CenteredAxisCalibration{Left: 0, Center: 0x8000, Right: 0xFFFF}
}

// Normalize maps raw to [-1,+1] using independent center->left and
// center->right linear segments, then applies an optional center deadzone.
func (c CenteredAxisCalibration) Normalize(raw uint16, deadzone float64) float64 {
	diff := float64(int(raw) - c.Center)
	towardRight := float64(c.Right - c.Center)
	towardLeft := float64(c.Left - c.Center)

	sameSign := func(a, b float64) bool {
		if a == 0 || b == 0 {
			return false
		}
		return (a > 0) == (b > 0)
	}

	var t float64
	if towardRight != 0 && sameSign(diff, towardRight) {
		t = diff / towardRight
	} else if towardLeft != 0 && sameSign(diff, towardLeft) {
		t = -diff / towardLeft
	}
	t = Clamp(t, -1.0, 1.0)
	if deadzone < 0 {
		deadzone = 0
	}
	if math.Abs(t) <= deadzone {
		return 0
	}
	return t
}

// PedalCalibration ports OpenG27.Core.PedalCalibration. It supports both
// inverted and non-inverted electrical scales plus independent deadzone and
// gamma/sensitivity shaping.
type PedalCalibration struct {
	Rest        int
	Pressed     int
	Sensitivity float64
	Deadzone    float64
}

func DefaultPedalCalibration() PedalCalibration {
	return PedalCalibration{Rest: 0xFF, Pressed: 0x00, Sensitivity: 1.0, Deadzone: 0}
}

func (c PedalCalibration) Normalize(raw byte) float64 {
	if c.Rest == c.Pressed {
		return 0
	}
	t := float64(int(raw)-c.Rest) / float64(c.Pressed-c.Rest)
	t = Clamp(t, 0, 1)
	dz := Clamp(c.Deadzone, 0, 0.999999)
	if dz > 0 {
		if t <= dz {
			t = 0
		} else {
			t = Clamp((t-dz)/(1-dz), 0, 1)
		}
	}
	sensitivity := c.Sensitivity
	if sensitivity <= 0 || math.IsNaN(sensitivity) || math.IsInf(sensitivity, 0) {
		sensitivity = 1
	}
	if sensitivity != 1 {
		t = math.Pow(t, sensitivity)
	}
	return t
}

// RangeTracker is a direct port of OpenG27.Core.RangeTracker.
type RangeTracker struct {
	HasData bool
	Min     int
	Max     int
}

func (r *RangeTracker) Add(sample int) {
	if !r.HasData {
		r.Min, r.Max, r.HasData = sample, sample, true
		return
	}
	if sample < r.Min {
		r.Min = sample
	}
	if sample > r.Max {
		r.Max = sample
	}
}

func (r *RangeTracker) Reset() {
	r.HasData = false
	r.Min = 0
	r.Max = 0
}
