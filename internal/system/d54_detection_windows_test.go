//go:build windows

package system

import (
	"testing"
	"time"
)

func TestD54StaleLegacyPreferenceDoesNotBlockGenericHIDG27(t *testing.T) {
	s := d53ActionableCompatG27("legacy", "Generic HID / Modern")
	if !shouldAutoPrepareNativeWheel(s) {
		t.Fatal("stale legacy preference must not strand an actually Generic-HID G27 in C294")
	}
}

func TestD54LiveLegacyBindingStillBlocksAutoNative(t *testing.T) {
	s := d53ActionableCompatG27("modern", "Logitech Legacy")
	if shouldAutoPrepareNativeWheel(s) {
		t.Fatal("actual live Legacy binding must suppress automatic C294 native switch")
	}
}

func TestD54OpenG27SwitchDeadline(t *testing.T) {
	if openG27SwitchWriteTimeout < 2500*time.Millisecond {
		t.Fatalf("G27 switch timeout=%s; OpenG27-style handshake needs a multi-second write window", openG27SwitchWriteTimeout)
	}
}
