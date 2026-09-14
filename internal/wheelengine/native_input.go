package wheelengine

import (
	"encoding/binary"
	"fmt"
)

// NativeInputReport is the model-adapter output before user calibration. It is
// platform independent so D1 fixtures can validate report decoding on every CI
// host, while the Windows layer remains responsible for device I/O and stored
// calibration.
type NativeInputReport struct {
	Model          ModelID
	LayoutID       string
	Steering       uint16
	ThrottleRaw    byte
	BrakeRaw       byte
	ClutchRaw      byte
	HasClutch      bool
	Buttons        uint32
	DPad           int // 0 neutral, 1=N ... 8=NW
	Gear           int // -1 reverse, 0 neutral, 1..6
	PaddleLeft     bool
	PaddleRight    bool
	WheelButtons   [6]bool
	ShifterButtons [8]bool
}

func nativeReportBase(report []byte) int {
	if len(report) >= 12 && report[0] == 0 {
		return 1
	}
	return 0
}

func bit(v, mask byte) bool { return v&mask != 0 }
func setButton(mask *uint32, index int, active bool) {
	if active && index >= 0 && index < 32 {
		*mask |= 1 << uint(index)
	}
}

func ParseNativeInputReport(model ModelID, report []byte) (NativeInputReport, error) {
	a, ok := AdapterForModel(model)
	if !ok {
		return NativeInputReport{}, fmt.Errorf("no native input adapter for %s", model)
	}
	base := nativeReportBase(report)
	if len(report) < base+a.MinNativeReportBytes {
		return NativeInputReport{}, fmt.Errorf("%s report too short: %d", model, len(report))
	}
	out := NativeInputReport{Model: model, LayoutID: a.InputLayoutID}
	out.Steering = binary.LittleEndian.Uint16(report[base+3 : base+5])
	out.ThrottleRaw = report[base+5]
	out.BrakeRaw = report[base+6]
	dpadRaw := int(report[base] & 0x0f)
	if dpadRaw >= 0 && dpadRaw <= 7 {
		out.DPad = dpadRaw + 1
	}

	switch model {
	case ModelG27:
		out.HasClutch = true
		out.ClutchRaw = report[base+7]
		b0, b1, b2, b3 := report[base], report[base+1], report[base+2], report[base+3]
		rev := report[base+10]
		out.PaddleRight = bit(b1, 0x01)
		out.PaddleLeft = bit(b1, 0x02)
		out.WheelButtons[0] = bit(b1, 0x08)
		out.WheelButtons[3] = bit(b1, 0x04)
		out.WheelButtons[4] = bit(b2, 0x40)
		out.WheelButtons[5] = bit(b2, 0x80)
		out.WheelButtons[1] = bit(b3, 0x01)
		out.WheelButtons[2] = bit(b3, 0x02)
		out.ShifterButtons[0] = bit(b0, 0x80)
		out.ShifterButtons[1] = bit(b0, 0x40)
		out.ShifterButtons[2] = bit(b0, 0x10)
		out.ShifterButtons[3] = bit(b0, 0x20)
		out.ShifterButtons[4] = bit(b1, 0x80)
		out.ShifterButtons[5] = bit(b1, 0x10)
		out.ShifterButtons[6] = bit(b1, 0x20)
		out.ShifterButtons[7] = bit(b1, 0x40)
		if rev&0x01 != 0 {
			out.Gear = -1
		} else {
			switch b2 & 0x3f {
			case 0x01:
				out.Gear = 1
			case 0x02:
				out.Gear = 2
			case 0x04:
				out.Gear = 3
			case 0x08:
				out.Gear = 4
			case 0x10:
				out.Gear = 5
			case 0x20:
				out.Gear = 6
			}
		}
		for i, v := range out.WheelButtons {
			setButton(&out.Buttons, i, v)
		}
		setButton(&out.Buttons, 6, out.PaddleLeft)
		setButton(&out.Buttons, 7, out.PaddleRight)
		for i, v := range out.ShifterButtons {
			setButton(&out.Buttons, 8+i, v)
		}
	case ModelG25:
		out.HasClutch = true
		out.ClutchRaw = report[base+11]
		out.Buttons = uint32(report[base]&0xf0) | uint32(report[base+1])<<8 | uint32(report[base+2])<<16
		x, y := report[base+8], report[base+9]
		switch {
		case y > 80 && y < 180:
			out.Gear = 0
		case x < 110 && y > 180:
			out.Gear = 1
		case x < 110:
			out.Gear = 2
		case x <= 170 && y > 180:
			out.Gear = 3
		case x <= 170:
			out.Gear = 4
		case x <= 195 && y > 180:
			out.Gear = 5
		case x <= 195:
			out.Gear = 6
		case x > 195 && y < 80:
			out.Gear = -1
		}
		b1, b2 := report[base+1], report[base+2]
		out.PaddleRight = bit(b1, 0x01)
		out.PaddleLeft = bit(b1, 0x02)
		out.WheelButtons[0] = bit(b1, 0x08)
		out.WheelButtons[1] = bit(b1, 0x04)
		out.ShifterButtons[0] = bit(b2, 0x08)
		out.ShifterButtons[1] = bit(b2, 0x10)
		out.ShifterButtons[2] = bit(b2, 0x20)
		out.ShifterButtons[3] = bit(b2, 0x40)
	case ModelDFGT:
		out.Buttons = uint32(report[base]&0xf0) | uint32(report[base+1])<<8 | uint32(report[base+2])<<16
	default:
		return NativeInputReport{}, fmt.Errorf("unsupported native input model %s", model)
	}
	return out, nil
}
