package wheelengine

import "strings"

// ModelID is the canonical machine-readable identity for a wheel family.
// Human-readable product strings must never be used as a control-flow key.
type ModelID string

const (
	ModelUnknown       ModelID = "unknown"
	ModelCompatibility ModelID = "compatibility"
	ModelG25           ModelID = "g25"
	ModelG27           ModelID = "g27"
	ModelDFGT          ModelID = "dfgt"
)

type OperatingMode string

const (
	ModeUnknown OperatingMode = "unknown"
	ModeLegacy  OperatingMode = "legacy"
	ModeModern  OperatingMode = "modern"
)

type ValidationLevel string

const (
	ValidationUnknown      ValidationLevel = "unknown"
	ValidationCodeOnly     ValidationLevel = "code-only"
	ValidationHardwareGate ValidationLevel = "hardware-gate"
	ValidationValidated    ValidationLevel = "validated"
)

type ControlCapabilities struct {
	Clutch          bool
	HShifter        bool
	SequentialShift bool
	Paddles         bool
	DPad            bool
	RPMLEDs         bool
	ButtonCount     int
}

type OutputCapabilities struct {
	NativeFFB      bool
	Range          bool
	Autocenter     bool
	ConstantForce  bool
	Spring         bool
	Damper         bool
	Friction       bool
	RPMLEDs        bool
	MaxEffectSlots int
	MaxRotationDeg int
	MinRotationDeg int
}

type Descriptor struct {
	Model              ModelID
	DisplayName        string
	VendorID           string
	CompatibilityPID   string
	NativePID          string
	NativeModeSelector byte
	NativeModeLabel    string
	Controls           ControlCapabilities
	Output             OutputCapabilities
	Validation         ValidationLevel
}

var descriptors = map[ModelID]Descriptor{
	ModelG25: {
		Model: ModelG25, DisplayName: "Logitech G25", VendorID: "046D", CompatibilityPID: "C294",
		NativePID: "C299", NativeModeSelector: 0x02, NativeModeLabel: "G25 / C299",
		Controls:   ControlCapabilities{Clutch: true, HShifter: true, SequentialShift: true, Paddles: true, DPad: true, ButtonCount: 18},
		Output:     OutputCapabilities{NativeFFB: true, Range: true, Autocenter: true, ConstantForce: true, Spring: true, Damper: true, Friction: true, MaxEffectSlots: 4, MinRotationDeg: 40, MaxRotationDeg: 900},
		Validation: ValidationHardwareGate,
	},
	ModelG27: {
		Model: ModelG27, DisplayName: "Logitech G27", VendorID: "046D", CompatibilityPID: "C294",
		NativePID: "C29B", NativeModeSelector: 0x04, NativeModeLabel: "G27 / C29B",
		Controls:   ControlCapabilities{Clutch: true, HShifter: true, SequentialShift: false, Paddles: true, DPad: true, RPMLEDs: true, ButtonCount: 22},
		Output:     OutputCapabilities{NativeFFB: true, Range: true, Autocenter: true, ConstantForce: true, Spring: true, Damper: true, Friction: true, RPMLEDs: true, MaxEffectSlots: 4, MinRotationDeg: 40, MaxRotationDeg: 900},
		Validation: ValidationHardwareGate,
	},
	ModelDFGT: {
		Model: ModelDFGT, DisplayName: "Logitech Driving Force GT", VendorID: "046D", CompatibilityPID: "C294",
		NativePID: "C29A", NativeModeSelector: 0x03, NativeModeLabel: "Driving Force GT / C29A",
		Controls:   ControlCapabilities{Clutch: false, HShifter: false, SequentialShift: true, Paddles: false, DPad: true, ButtonCount: 20},
		Output:     OutputCapabilities{NativeFFB: true, Range: true, Autocenter: true, ConstantForce: true, Spring: true, Damper: true, Friction: true, MaxEffectSlots: 4, MinRotationDeg: 40, MaxRotationDeg: 900},
		Validation: ValidationHardwareGate,
	},
	ModelCompatibility: {
		Model: ModelCompatibility, DisplayName: "Logitech C294 compatibility mode", VendorID: "046D", CompatibilityPID: "C294",
		Validation: ValidationCodeOnly,
	},
}

func DescriptorForModel(model ModelID) (Descriptor, bool) {
	d, ok := descriptors[model]
	return d, ok
}

func Descriptors() []Descriptor {
	order := []ModelID{ModelG25, ModelG27, ModelDFGT, ModelCompatibility}
	out := make([]Descriptor, 0, len(order))
	for _, id := range order {
		if d, ok := descriptors[id]; ok {
			out = append(out, d)
		}
	}
	return out
}

func ModelFromPID(pid string) ModelID {
	switch strings.ToUpper(strings.TrimSpace(pid)) {
	case "C299":
		return ModelG25
	case "C29B":
		return ModelG27
	case "C29A":
		return ModelDFGT
	case "C294":
		return ModelCompatibility
	default:
		return ModelUnknown
	}
}

func ModelFromDisplayName(name string) ModelID {
	s := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(s, "logitech g25"):
		return ModelG25
	case strings.HasPrefix(s, "logitech g27"):
		return ModelG27
	case strings.HasPrefix(s, "logitech driving force gt"):
		return ModelDFGT
	case strings.Contains(s, "c294") && (strings.Contains(s, "compat") || strings.Contains(s, "kompat")):
		return ModelCompatibility
	default:
		return ModelUnknown
	}
}

func ParseOperatingMode(mode string) OperatingMode {
	s := strings.ToLower(strings.TrimSpace(mode))
	switch {
	case strings.Contains(s, "generic hid"), strings.Contains(s, "modern"):
		return ModeModern
	case strings.Contains(s, "legacy"), strings.Contains(s, "logitech gaming"):
		return ModeLegacy
	default:
		return ModeUnknown
	}
}
