//go:build windows

package system

import (
	"strings"
	"testing"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

func TestD5StandaloneCoreUsesNativeModels(t *testing.T) {
	if err := NativeEngineCoreGate(); err != nil {
		t.Fatal(err)
	}
	for _, m := range []wheelengine.ModelID{wheelengine.ModelG25, wheelengine.ModelG27, wheelengine.ModelDFGT} {
		if err := wheelengine.ValidateModelAdapter(m); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
	}
}

func TestD5NativeInputDefaultsAreSelfOwned(t *testing.T) {
	cfg := defaultNativeInputDefaults()
	if got := normalizeNativeSteering(0x8000, cfg.Steering, cfg.Deadzone); got != 0 {
		t.Fatalf("center=%v", got)
	}
	if got := normalizeNativePedal(0xff, cfg.Throttle); got != 0 {
		t.Fatalf("pedal rest=%v", got)
	}
	if got := normalizeNativePedal(0x00, cfg.Throttle); got != 1 {
		t.Fatalf("pedal pressed=%v", got)
	}
}

func TestD5LegacyProfileImportIsOfflineCompatibilityOnly(t *testing.T) {
	src := []byte(`[{"Name":"Legacy","ProcessMatch":"legacygame","RotationDeg":540,"GameFfbGain":75}]`)
	parsed, err := parseLegacyOpenG27GameProfiles(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || parsed[0].GameFfbGain != 75 || !strings.EqualFold(parsed[0].ProcessMatch, "legacygame") {
		t.Fatalf("unexpected legacy import: %+v", parsed)
	}
}
