//go:build windows

package system

import (
	"fmt"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// NativeEngineCoreGate is the hardware-free standalone gate for every integrated
// classic Logitech model. D5 deliberately depends only on LogiMate-owned native
// code; historical OpenG27 parity lives exclusively in tests/import provenance.
func NativeEngineCoreGate() error {
	want := map[wheelengine.ModelID]struct {
		pid      string
		selector byte
	}{
		wheelengine.ModelG25:  {pidG25, 0x02},
		wheelengine.ModelDFGT: {pidDFGT, 0x03},
		wheelengine.ModelG27:  {pidG27, 0x04},
	}
	for model, exp := range want {
		d, ok := wheelengine.DescriptorForModel(model)
		if !ok || d.NativePID != exp.pid || d.NativeModeSelector != exp.selector || !d.Output.NativeFFB {
			return fmt.Errorf("Descriptor-Gate für %s fehlgeschlagen", model)
		}
		if err := wheelengine.ValidateModelAdapter(model); err != nil {
			return fmt.Errorf("D1 Modelladapter %s: %w", model, err)
		}
	}
	if wheelengine.SchedulerTick <= 0 || wheelengine.TargetHz != 150 {
		return fmt.Errorf("D2 Native-Scheduler-Konstanten ungültig")
	}
	if got := wheelengine.NativeSwitch(0x04); len(got) != 7 || got[2] != 0x04 {
		return fmt.Errorf("D2 Native-Protokoll-Gate fehlgeschlagen")
	}
	return nil
}
