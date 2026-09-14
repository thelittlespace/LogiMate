//go:build windows

package system

import "testing"

func TestFusionC2ShadowKnownParityAndDiffs(t *testing.T) {
	m := map[string]bool{}
	for _, c := range FusionC2ProtocolComparisons() {
		m[c.Name] = c.Match
	}
	for _, name := range []string{"Rotation 900°", "G27 native switch", "G27 LEDs 0x1F", "Autocenter SpringSet", "Autocenter SpringEnable sequence", "Constant Force neutral/start", "Damper slot-2 layout", "Friction slot-3 layout", "Damper slot-2 stop", "Friction slot-3 stop"} {
		if !m[name] {
			t.Fatalf("expected C2 parity for %q", name)
		}
	}
}
