//go:build windows

package system

import (
	"math"
)

type nativeAxisCalibration struct {
	Left, Center, Right int
}

type nativePedalCalibration struct {
	Rest, Pressed int
	Sensitivity   float64
	Deadzone      float64
}

type nativePedalRoles struct {
	Throttle, Brake, Clutch string
}

type nativeInputDefaults struct {
	Steering   nativeAxisCalibration
	Throttle   nativePedalCalibration
	Brake      nativePedalCalibration
	Clutch     nativePedalCalibration
	PedalRoles nativePedalRoles
	Deadzone   float64
}

func defaultNativeInputDefaults() nativeInputDefaults {
	pedal := nativePedalCalibration{Rest: 0xFF, Pressed: 0x00, Sensitivity: 1}
	return nativeInputDefaults{
		Steering: nativeAxisCalibration{Left: 0, Center: 0x8000, Right: 0xFFFF},
		Throttle: pedal, Brake: pedal, Clutch: pedal,
		PedalRoles: nativePedalRoles{Throttle: "Idx6", Brake: "Idx7", Clutch: "Idx8"},
		Deadzone:   0.02,
	}
}

func normalizeNativeSteering(raw uint16, c nativeAxisCalibration, deadzone float64) float64 {
	diff := float64(int(raw) - c.Center)
	right := float64(c.Right - c.Center)
	left := float64(c.Left - c.Center)
	t := 0.0
	switch {
	case right != 0 && math.Signbit(diff) == math.Signbit(right):
		t = diff / right
	case left != 0 && math.Signbit(diff) == math.Signbit(left):
		t = -diff / left
	}
	if t < -1 {
		t = -1
	}
	if t > 1 {
		t = 1
	}
	if deadzone < 0 {
		deadzone = 0
	}
	if deadzone > 1 {
		deadzone = 1
	}
	if math.Abs(t) <= deadzone {
		t = 0
	}
	return t
}

func normalizeNativePedal(raw byte, c nativePedalCalibration) float64 {
	if c.Rest == c.Pressed {
		return 0
	}
	t := float64(int(raw)-c.Rest) / float64(c.Pressed-c.Rest)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	dz := c.Deadzone
	if dz < 0 {
		dz = 0
	}
	if dz >= 1 {
		dz = 0.999
	}
	if dz > 0 {
		if t <= dz {
			t = 0
		} else {
			t = (t - dz) / (1 - dz)
		}
	}
	sensitivity := c.Sensitivity
	if sensitivity <= 0 {
		sensitivity = 1
	}
	if sensitivity != 1 {
		t = math.Pow(t, sensitivity)
	}
	return t
}

func norm01ToAxis(v float64) uint32 {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return uint32(math.Round(v * 65535))
}

func normSignedToAxis(v float64) uint32 {
	if v < -1 {
		v = -1
	}
	if v > 1 {
		v = 1
	}
	return uint32(math.Round((v + 1) * 0.5 * 65535))
}
