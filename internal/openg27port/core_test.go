package openg27port

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestForceScalingGoldenVectors(t *testing.T) {
	vectors := []struct {
		p    int
		want byte
	}{{-100, 0x00}, {-50, 0x40}, {0, 0x80}, {50, 0xC0}, {100, 0xFF}, {-999, 0x00}, {999, 0xFF}}
	for _, v := range vectors {
		if got := SliderToForceByte(v.p); got != v.want {
			t.Fatalf("SliderToForceByte(%d)=0x%02X want 0x%02X", v.p, got, v.want)
		}
	}
	if got := SliderToForceByteWithGain(100, 0.5); got != 0xC0 {
		t.Fatalf("gain parity: got 0x%02X want 0xC0", got)
	}
}

func TestForceMixerGoldenVectors(t *testing.T) {
	if got := ResolveForceFrame(ForceFrame{Constant: .6, Transient: .2}, .5); !near(got, .4) {
		t.Fatalf("resolve=%v want .4", got)
	}
	if got := ResolveForceFrame(ForceFrame{Constant: .9, Transient: .9}, 1); !near(got, 1) {
		t.Fatalf("upper clamp=%v", got)
	}
	if got := ResolveForceFrame(ForceFrame{Constant: -.9, Transient: -.9}, 1); !near(got, -1) {
		t.Fatalf("lower clamp=%v", got)
	}
	if TorqueToWheelByte(-1) != 0x01 || TorqueToWheelByte(0) != 0x80 || TorqueToWheelByte(1) != 0xFF {
		t.Fatalf("unexpected torque byte endpoints: %02X %02X %02X", TorqueToWheelByte(-1), TorqueToWheelByte(0), TorqueToWheelByte(1))
	}
}

func TestCenteredAxisCalibrationNormalAndInverted(t *testing.T) {
	c := CenteredAxisCalibration{Left: 1000, Center: 3000, Right: 7000}
	if !near(c.Normalize(1000, 0), -1) || !near(c.Normalize(3000, 0), 0) || !near(c.Normalize(7000, 0), 1) {
		t.Fatalf("normal calibration endpoints failed")
	}
	if !near(c.Normalize(3200, .1), 0) {
		t.Fatalf("deadzone failed")
	}
	inv := CenteredAxisCalibration{Left: 60000, Center: 30000, Right: 1000}
	if !near(inv.Normalize(60000, 0), -1) || !near(inv.Normalize(1000, 0), 1) {
		t.Fatalf("inverted calibration endpoints failed")
	}
}

func TestPedalCalibrationInvertedDeadzoneSensitivity(t *testing.T) {
	c := DefaultPedalCalibration()
	if !near(c.Normalize(0xFF), 0) || !near(c.Normalize(0x00), 1) {
		t.Fatalf("inverted pedal endpoints failed")
	}
	c.Deadzone = .1
	if c.Normalize(245) != 0 {
		t.Fatalf("deadzone should keep near-rest at zero: %v", c.Normalize(245))
	}
	c.Deadzone = 0
	c.Sensitivity = 2
	got := c.Normalize(128)
	linear := float64(255-128) / 255
	if !near(got, linear*linear) {
		t.Fatalf("sensitivity=%v want %v", got, linear*linear)
	}
}

func TestRangeTracker(t *testing.T) {
	var r RangeTracker
	for _, v := range []int{10, 4, 25, 8} {
		r.Add(v)
	}
	if !r.HasData || r.Min != 4 || r.Max != 25 {
		t.Fatalf("range=%+v", r)
	}
	r.Reset()
	if r.HasData || r.Min != 0 || r.Max != 0 {
		t.Fatalf("reset=%+v", r)
	}
}
