package wheelengine

import "fmt"

// ModelAdapter contains model differences that are allowed to affect native
// runtime behavior. D1 keeps these differences out of the UI/system layer.
type ModelAdapter struct {
	Model                ModelID
	InputLayoutID        string
	MinNativeReportBytes int
	NativePID            string
	ModeSelector         byte
	SteeringBits         int
	PedalBits            int
	HasAnalogShifter     bool
	Certification        ValidationLevel
}

var adapters = map[ModelID]ModelAdapter{
	ModelG25:  {Model: ModelG25, InputLayoutID: "g25-c299-v1", MinNativeReportBytes: 12, NativePID: "C299", ModeSelector: 0x02, SteeringBits: 16, PedalBits: 8, HasAnalogShifter: true, Certification: ValidationHardwareGate},
	ModelG27:  {Model: ModelG27, InputLayoutID: "g27-c29b-v1", MinNativeReportBytes: 11, NativePID: "C29B", ModeSelector: 0x04, SteeringBits: 16, PedalBits: 8, HasAnalogShifter: false, Certification: ValidationHardwareGate},
	ModelDFGT: {Model: ModelDFGT, InputLayoutID: "dfgt-c29a-v1", MinNativeReportBytes: 7, NativePID: "C29A", ModeSelector: 0x03, SteeringBits: 16, PedalBits: 8, HasAnalogShifter: false, Certification: ValidationHardwareGate},
}

func AdapterForModel(model ModelID) (ModelAdapter, bool) { a, ok := adapters[model]; return a, ok }
func ValidateModelAdapter(model ModelID) error {
	d, ok := DescriptorForModel(model)
	if !ok {
		return fmt.Errorf("descriptor missing for %s", model)
	}
	a, ok := AdapterForModel(model)
	if !ok {
		return fmt.Errorf("adapter missing for %s", model)
	}
	if a.NativePID != d.NativePID || a.ModeSelector != d.NativeModeSelector {
		return fmt.Errorf("adapter/descriptor native identity mismatch for %s", model)
	}
	if a.InputLayoutID == "" || a.MinNativeReportBytes < 7 {
		return fmt.Errorf("invalid input adapter for %s", model)
	}
	return nil
}
