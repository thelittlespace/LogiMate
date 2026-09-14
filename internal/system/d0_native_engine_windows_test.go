//go:build windows

package system

import (
	"testing"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

func TestD0NativeEngineCoreGate(t *testing.T) {
	if err := NativeEngineCoreGate(); err != nil {
		t.Fatalf("D0 core gate: %v", err)
	}
}

func TestD0SupportedModelCapabilities(t *testing.T) {
	cases := []struct {
		model     wheelengine.ModelID
		pid       string
		selector  byte
		clutch    bool
		hShifter  bool
		rpmLEDs   bool
		nativeFFB bool
	}{
		{wheelengine.ModelG25, "C299", 0x02, true, true, false, true},
		{wheelengine.ModelG27, "C29B", 0x04, true, true, true, true},
		{wheelengine.ModelDFGT, "C29A", 0x03, false, false, false, true},
	}
	for _, tc := range cases {
		d, ok := wheelengine.DescriptorForModel(tc.model)
		if !ok {
			t.Fatalf("missing descriptor %s", tc.model)
		}
		if d.NativePID != tc.pid || d.NativeModeSelector != tc.selector || d.Controls.Clutch != tc.clutch || d.Controls.HShifter != tc.hShifter || d.Controls.RPMLEDs != tc.rpmLEDs || d.Output.NativeFFB != tc.nativeFFB {
			t.Fatalf("descriptor mismatch %s: %+v", tc.model, d)
		}
	}
}

func TestD0TelemetryAdapterLegacyAliasNormalizes(t *testing.T) {
	info, ok := TelemetryAdapterByID("openg27-pino")
	if !ok {
		t.Fatal("legacy Pino adapter alias must remain readable")
	}
	if info.ID != TelemetryAdapterWreckfestPino {
		t.Fatalf("legacy alias resolved to %q, want %q", info.ID, TelemetryAdapterWreckfestPino)
	}
	p := normalizeGameProfile(GameProfile{Name: "legacy", TelemetryAdapter: "openg27-pino"})
	if p.TelemetryAdapter != TelemetryAdapterWreckfestPino {
		t.Fatalf("profile adapter not canonicalized: %q", p.TelemetryAdapter)
	}
}

func TestD0TelemetryRegistryExposesLogiMateNativeID(t *testing.T) {
	for _, a := range RegisteredTelemetryAdapters() {
		if a.ID == "openg27-pino" {
			t.Fatal("legacy OpenG27 adapter ID must not be advertised as the canonical runtime adapter")
		}
	}
	if info, ok := TelemetryAdapterByID(TelemetryAdapterWreckfestPino); !ok || info.Name != "Wreckfest 2 / Pino" {
		t.Fatalf("canonical Pino adapter missing: %+v ok=%v", info, ok)
	}
}
