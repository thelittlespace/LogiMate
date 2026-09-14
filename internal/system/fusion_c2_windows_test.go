//go:build windows

package system

import "testing"

func TestFusionC2ShadowKnownParityAndDiffs(t *testing.T) {
	m := map[string]bool{}
	for _, c := range FusionC2ProtocolComparisons() {
		m[c.Name] = c.Match
	}
	for _, name := range []string{"Rotation 900°", "G27 native switch", "Autocenter SpringSet", "Autocenter SpringEnable sequence", "Constant Force neutral/start", "Damper slot-1 layout", "Friction slot-1 layout", "Condition slot-1 stop"} {
		if !m[name] {
			t.Fatalf("expected C2 parity for %q", name)
		}
	}
	for _, name := range []string{"G27 LEDs 0x1F"} {
		if m[name] {
			t.Fatalf("expected C2 shadow difference for %q; routing must not be silently changed", name)
		}
	}
}
