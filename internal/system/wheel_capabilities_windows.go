//go:build windows

package system

import "github.com/thelittlespace/LogiMate/internal/wheelengine"

// Compatibility aliases keep the C8 API stable while D0 moves canonical model
// truth into the platform-independent wheelengine package.
type WheelModelKind = wheelengine.ModelID

type OperatingModeKind = wheelengine.OperatingMode

type WheelCapabilities struct {
	Model              WheelModelKind
	NativePID          string
	NativeModeSelector byte
	NativeModeLabel    string
	HasClutch          bool
	HasHShifter        bool
	HasRPMLEDs         bool
	HasNativeFFB       bool
	MaxRotationDegrees int
}

const (
	WheelModelUnknown       = wheelengine.ModelUnknown
	WheelModelCompatibility = wheelengine.ModelCompatibility
	WheelModelG25           = wheelengine.ModelG25
	WheelModelG27           = wheelengine.ModelG27
	WheelModelDFGT          = wheelengine.ModelDFGT

	OperatingModeUnknown = wheelengine.ModeUnknown
	OperatingModeLegacy  = wheelengine.ModeLegacy
	OperatingModeModern  = wheelengine.ModeModern
)

func ClassifyWheelModel(model string) WheelModelKind { return wheelengine.ModelFromDisplayName(model) }

func ClassifyOperatingMode(mode string) OperatingModeKind {
	return wheelengine.ParseOperatingMode(mode)
}

func CapabilitiesForWheelModel(model string) (WheelCapabilities, bool) {
	d, ok := wheelengine.DescriptorForModel(ClassifyWheelModel(model))
	if !ok || d.Model == wheelengine.ModelCompatibility || d.Model == wheelengine.ModelUnknown {
		return WheelCapabilities{Model: ClassifyWheelModel(model)}, false
	}
	return WheelCapabilities{
		Model: d.Model, NativePID: d.NativePID, NativeModeSelector: d.NativeModeSelector, NativeModeLabel: d.NativeModeLabel,
		HasClutch: d.Controls.Clutch, HasHShifter: d.Controls.HShifter, HasRPMLEDs: d.Controls.RPMLEDs,
		HasNativeFFB: d.Output.NativeFFB, MaxRotationDegrees: d.Output.MaxRotationDeg,
	}, true
}

func SelectedWheelHasRPMLEDs(s State) bool {
	w, ok := SelectedWheel(s)
	if !ok {
		return false
	}
	c, ok := WheelCapabilitiesForDevice(w)
	return ok && c.HasRPMLEDs
}
