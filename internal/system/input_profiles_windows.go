//go:build windows

package system

import (
	"encoding/hex"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

// Audit 31-40 input learning/calibration state. It is deliberately keyed by
// LogiMate's stable physical wheel ID so USB/HID re-enumeration does not lose
// learned controls.
type AxisCalibration struct {
	Min      uint32  `json:"min"`
	Center   uint32  `json:"center,omitempty"`
	Max      uint32  `json:"max"`
	Inverted bool    `json:"inverted,omitempty"`
	Deadzone float64 `json:"deadzone,omitempty"`
	Curve    string  `json:"curve,omitempty"`
}

type SteeringCalibration struct {
	Left         uint32 `json:"left"`
	Center       uint32 `json:"center"`
	Right        uint32 `json:"right"`
	RangeDegrees int    `json:"rangeDegrees"`
}

type ControlProfile struct {
	Buttons  map[string]uint32   `json:"buttons,omitempty"` // logical name -> bit index
	Gears    map[string]int      `json:"gears,omitempty"`   // stable raw signature -> gear (-1,0,1..6)
	Steering SteeringCalibration `json:"steering,omitempty"`
}

const inputProfileSchemaVersion = 1

type inputProfileFile struct {
	Version  int                       `json:"version"`
	Profiles map[string]ControlProfile `json:"profiles"`
}

func inputProfilePath(dataDir string) string { return filepath.Join(dataDir, "input-profiles.json") }

func inputProfileKey(wheelID string) string {
	return sanitizePedalWheelID(wheelID)
}

func defaultInputProfileFile() inputProfileFile {
	return inputProfileFile{Version: inputProfileSchemaVersion, Profiles: map[string]ControlProfile{}}
}

func readInputProfileFileStrict(dataDir string) (inputProfileFile, error) {
	f := defaultInputProfileFile()
	exists, err := ReadJSONConfigStrict(inputProfilePath(dataDir), &f)
	if err != nil {
		return defaultInputProfileFile(), err
	}
	if exists && f.Version > inputProfileSchemaVersion {
		return defaultInputProfileFile(), fmt.Errorf("input-profiles.json verwendet Schema %d; unterstützt wird maximal %d", f.Version, inputProfileSchemaVersion)
	}
	if f.Profiles == nil {
		f.Profiles = map[string]ControlProfile{}
	}
	if f.Version == 0 {
		f.Version = inputProfileSchemaVersion
	}
	return f, nil
}

func readInputProfileFile(dataDir string) inputProfileFile {
	f, err := readInputProfileFileStrict(dataDir)
	if err != nil {
		return defaultInputProfileFile()
	}
	return f
}

func writeInputProfileFile(dataDir string, f inputProfileFile) error {
	if f.Profiles == nil {
		f.Profiles = map[string]ControlProfile{}
	}
	f.Version = inputProfileSchemaVersion
	return WriteJSONConfigStrict(inputProfilePath(dataDir), f, 0644)
}

func ReadControlProfile(dataDir, wheelID string) ControlProfile {
	f := readInputProfileFile(dataDir)
	return f.Profiles[inputProfileKey(wheelID)]
}

func SaveButtonMapping(dataDir, wheelID, logical string, bit uint32) error {
	if strings.TrimSpace(wheelID) == "" {
		return fmt.Errorf("keine stabile Wheel-ID")
	}
	logical = strings.TrimSpace(logical)
	if logical == "" || bit >= 32 {
		return fmt.Errorf("ungültiges Button-Mapping")
	}
	f, err := readInputProfileFileStrict(dataDir)
	if err != nil {
		return err
	}
	key := inputProfileKey(wheelID)
	p := f.Profiles[key]
	if p.Buttons == nil {
		p.Buttons = map[string]uint32{}
	}
	p.Buttons[logical] = bit
	f.Profiles[key] = p
	return writeInputProfileFile(dataDir, f)
}

func SaveGearMapping(dataDir, wheelID string, signatures map[string]int) error {
	if strings.TrimSpace(wheelID) == "" {
		return fmt.Errorf("keine stabile Wheel-ID")
	}
	f, err := readInputProfileFileStrict(dataDir)
	if err != nil {
		return err
	}
	key := inputProfileKey(wheelID)
	p := f.Profiles[key]
	p.Gears = map[string]int{}
	for sig, gear := range signatures {
		sig = strings.TrimSpace(sig)
		if sig != "" {
			p.Gears[sig] = gear
		}
	}
	f.Profiles[key] = p
	return writeInputProfileFile(dataDir, f)
}

func SaveSteeringCalibration(dataDir, wheelID string, c SteeringCalibration) error {
	if strings.TrimSpace(wheelID) == "" {
		return fmt.Errorf("keine stabile Wheel-ID")
	}
	if c.RangeDegrees < 40 || c.RangeDegrees > 1080 {
		return fmt.Errorf("ungültiger Lenkwinkel: %d°", c.RangeDegrees)
	}
	if c.Left == c.Center || c.Right == c.Center || c.Left == c.Right {
		return fmt.Errorf("Lenkwinkel-Kalibrierung enthält identische Positionen")
	}
	lo, hi := c.Left, c.Right
	if lo > hi {
		lo, hi = hi, lo
	}
	if c.Center <= lo || c.Center >= hi {
		return fmt.Errorf("Lenkwinkel-Mitte muss strikt zwischen linkem und rechtem Anschlag liegen")
	}
	absDelta := func(a, b uint32) uint32 {
		if a > b {
			return a - b
		}
		return b - a
	}
	if absDelta(c.Left, c.Center) < 512 || absDelta(c.Right, c.Center) < 512 {
		return fmt.Errorf("Lenkwinkel-Kalibrierung hat zu wenig gemessenen Weg")
	}
	f, err := readInputProfileFileStrict(dataDir)
	if err != nil {
		return err
	}
	key := inputProfileKey(wheelID)
	p := f.Profiles[key]
	p.Steering = c
	f.Profiles[key] = p
	return writeInputProfileFile(dataDir, f)
}

func firstPressedBit(before, after uint32) (uint32, bool) {
	newly := after &^ before
	if newly == 0 || newly&(newly-1) != 0 {
		return 0, false
	}
	for i := uint32(0); i < 32; i++ {
		if newly&(1<<i) != 0 {
			return i, true
		}
	}
	return 0, false
}

func FirstPressedButton(before, after JoyState) (uint32, bool) {
	return firstPressedBit(before.Buttons, after.Buttons)
}

// GearSignature intentionally ignores steering/pedals so a learned gear remains
// stable while the user moves the wheel or pedals. Native G27/G25 reports use
// their shifter-specific bytes; WinMM falls back to button/POV state.
func GearSignature(j JoyState) string {
	raw := j.RawReport
	if len(raw) > 0 {
		base := 0
		if len(raw) >= 12 && raw[0] == 0 {
			base = 1
		}
		if strings.Contains(strings.ToLower(j.Name), "g27") && len(raw) > base+10 {
			return fmt.Sprintf("g27:%02x:%02x", raw[base+2]&0x3f, raw[base+10]&0x01)
		}
		if strings.Contains(strings.ToLower(j.Name), "g25") && len(raw) > base+9 {
			// G25 shifter is analog; coarse 16-value buckets tolerate small ADC jitter.
			return fmt.Sprintf("g25:%02x:%02x", raw[base+8]>>4, raw[base+9]>>4)
		}
	}
	return fmt.Sprintf("generic:%08x:%08x", j.Buttons, j.POV)
}

func RawReportHex(j JoyState) string {
	if len(j.RawReport) == 0 {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(j.RawReport))
}

func RawReportPretty(j JoyState) string {
	if len(j.RawReport) == 0 {
		return "Kein Direct-HID-Rohreport verfügbar (WinMM liefert nur normalisierte Controllerwerte)."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Quelle: %s\r\nBytes: %d\r\n", j.Selection, len(j.RawReport))
	for i, v := range j.RawReport {
		fmt.Fprintf(&b, "[%02d]  0x%02X  %3d  %08b\r\n", i, v, v, v)
	}
	return b.String()
}

func applyLearnedButtons(p ControlProfile, j *JoyState) {
	if len(p.Buttons) == 0 {
		return
	}
	override := func(name string, current *bool) bool {
		bit, ok := p.Buttons[name]
		if !ok || bit >= 32 {
			return false
		}
		*current = j.Buttons&(1<<bit) != 0
		return true
	}
	mapped := override("Paddle L", &j.PaddleLeft)
	mapped = override("Paddle R", &j.PaddleRight) || mapped
	for i := 0; i < 6; i++ {
		mapped = override(fmt.Sprintf("Wheel %d", i+1), &j.WheelButtons[i]) || mapped
	}
	for i := 0; i < 8; i++ {
		mapped = override(fmt.Sprintf("Shifter %d", i+1), &j.ShifterButtons[i]) || mapped
	}
	if mapped {
		j.NativeControls = true
	}
}

func steeringDegrees(c SteeringCalibration, x uint32) (float64, bool) {
	if c.RangeDegrees <= 0 || c.Left == c.Center || c.Right == c.Center || c.Left == c.Right {
		return 0, false
	}
	half := float64(c.RangeDegrees) / 2
	dx := int64(x) - int64(c.Center)
	dl := int64(c.Left) - int64(c.Center)
	dr := int64(c.Right) - int64(c.Center)
	leftSide := (dl < 0 && dx <= 0) || (dl > 0 && dx >= 0)
	if leftSide {
		t := float64(dx) / float64(dl)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		return -t * half, true
	}
	t := float64(dx) / float64(dr)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * half, true
}

func ApplyControlProfile(dataDir, wheelID string, j *JoyState) {
	if j == nil || strings.TrimSpace(wheelID) == "" {
		return
	}
	p := ReadControlProfile(dataDir, wheelID)
	applyLearnedButtons(p, j)
	if sig := GearSignature(*j); sig != "" && len(p.Gears) > 0 {
		if gear, ok := p.Gears[sig]; ok {
			j.Gear = gear
			j.NativeControls = true
		}
	}
	if deg, ok := steeringDegrees(p.Steering, j.X); ok {
		j.SteeringDegrees = deg
		j.SteeringCalibrated = true
		j.SteeringRangeDegrees = p.Steering.RangeDegrees
	}
}

func SortedButtonMappings(p ControlProfile) []string {
	keys := make([]string, 0, len(p.Buttons))
	for k := range p.Buttons {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%s=B%d", k, p.Buttons[k]+1))
	}
	return out
}

func curveValue(v float64, curve string) float64 {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	switch strings.ToLower(strings.TrimSpace(curve)) {
	case "progressiv", "progressive":
		return math.Pow(v, 1.65)
	case "weich", "soft":
		return math.Pow(v, 0.65)
	default:
		return v
	}
}

func calibratedAxisPercent(v uint32, c AxisCalibration) (float64, bool) {
	if c.Min == c.Max {
		return 0, false
	}
	min, max := c.Min, c.Max
	if min > max {
		min, max = max, min
	}
	t := (float64(v) - float64(min)) / (float64(max) - float64(min))
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	if c.Inverted {
		t = 1 - t
	}
	dz := c.Deadzone
	if dz < 0 {
		dz = 0
	}
	if dz > .45 {
		dz = .45
	}
	if t <= dz {
		t = 0
	} else if dz > 0 {
		t = (t - dz) / (1 - dz)
	}
	return curveValue(t, c.Curve), true
}

func axisRawByName(j JoyState, name string) (uint32, bool) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "X":
		return j.X, true
	case "Y":
		return j.Y, true
	case "Z":
		return j.Z, true
	case "R":
		return j.R, true
	case "U":
		return j.U, true
	case "V":
		return j.V, true
	default:
		return 0, false
	}
}

func PedalPercent(j JoyState, mapping PedalMapping, role string) (float64, bool) {
	if mapping.InputSource != "" && !strings.EqualFold(mapping.InputSource, j.InputSource) {
		return 0, false
	}
	if mapping.LayoutID != "" && !strings.EqualFold(mapping.LayoutID, j.LayoutID) {
		return 0, false
	}
	var axis string
	var c AxisCalibration
	switch strings.ToLower(role) {
	case "gas":
		axis, c = mapping.Gas, mapping.GasCalibration
	case "brake", "bremse":
		axis, c = mapping.Brake, mapping.BrakeCalibration
	case "clutch", "kupplung":
		axis, c = mapping.Clutch, mapping.ClutchCalibration
	default:
		return 0, false
	}
	if axis == "" {
		return 0, false
	}
	raw, ok := axisRawByName(j, axis)
	if !ok {
		return 0, false
	}
	return calibratedAxisPercent(raw, c)
}

func AxisRawValue(j JoyState, axis string) (uint32, bool) { return axisRawByName(j, axis) }
