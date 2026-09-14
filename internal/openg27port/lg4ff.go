package openg27port

// This file is a Go translation of Jabelius/OpenG27
// src/Core/Lg4ffReports.cs at commit 12c9421d2c9e3de05c3ff543ed7748f8d6162994.
//
// OpenG27 is MIT licensed. See docs/OPENG27_PROVENANCE.md and
// THIRD_PARTY_NOTICES.md. Builders return Logitech's seven command bytes.
// Windows' HID report-ID byte (0x00) is deliberately not included here,
// matching OpenG27's Core contract.

// ConstantForce returns the OpenG27/lg4ff constant-force download+play report.
// force 0x80 is neutral, 0x00 and 0xFF are the protocol extremes.
func ConstantForce(force byte) []byte {
	return []byte{0x11, 0x08, force, 0x80, 0x00, 0x00, 0x00}
}

// Stop returns the lg4ff global stop command.
func Stop() []byte {
	return []byte{0xF3, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

// SetRange returns the G25/G27 rotation range command. It intentionally keeps
// OpenG27's pure-builder semantics: callers own range validation/policy.
func SetRange(deg int) []byte {
	return []byte{0xF8, 0x81, byte(deg & 0xFF), byte((deg >> 8) & 0xFF), 0x00, 0x00, 0x00}
}

// NativeSwitch is OpenG27's G27 C294 -> C29B switch report.
func NativeSwitch() []byte {
	return []byte{0xF8, 0x09, 0x04, 0x01, 0x00, 0x00, 0x00}
}

func SpringSet(slope, clip byte) []byte {
	return []byte{0xFE, 0x0D, slope, slope, clip, 0x00, 0x00}
}

func SpringEnable() []byte {
	return []byte{0x14, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

func SpringOff() []byte {
	return []byte{0xF5, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

// Damper mirrors OpenG27 exactly: coefficient is a 4-bit nibble and clamps at
// 15; clip remains a full byte.
func Damper(coeff, clip byte) []byte {
	if coeff > 15 {
		coeff = 15
	}
	return []byte{0x21, 0x0C, coeff, 0x00, coeff, 0x00, clip}
}

func DamperOff() []byte {
	return []byte{0x23, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

// Friction differs from Damper: OpenG27/lg4ff uses the full coefficient byte.
func Friction(coeff, clip byte) []byte {
	return []byte{0x21, 0x0E, coeff, coeff, clip, 0x00, 0x00}
}

func FrictionOff() []byte {
	return []byte{0x23, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

func SetLeds(leds byte) []byte {
	return []byte{0xF8, 0x12, leds, 0x00, 0x00, 0x00, 0x00}
}

func LedsOff() []byte { return SetLeds(0) }

// LedBarForPercent reproduces OpenG27's cumulative lg4ff thresholds.
func LedBarForPercent(percent int) byte {
	if percent < 8 { // OpenG27 compares integer percent against 7.5.
		return 0b00000
	}
	if percent < 25 {
		return 0b00001
	}
	if percent < 50 {
		return 0b00011
	}
	if percent < 75 {
		return 0b00111
	}
	if percent < 90 {
		return 0b01111
	}
	return 0b11111
}

// WithReportID converts one seven-byte OpenG27 command into the eight-byte
// Windows HID buffer expected by HidD_SetOutputReport. It allocates a fresh
// slice so callers cannot mutate the source command through aliasing.
func WithReportID(command []byte) []byte {
	report := make([]byte, len(command)+1)
	copy(report[1:], command)
	return report
}
