//go:build windows

package system

import (
	"strings"
	"testing"
)

func TestD52HIDRankingPrefersLargestOutputReport(t *testing.T) {
	a := hidInterfaceMetadata{Path: `\\?\hid#a`, InputReportLength: 12, OutputReportLength: 8, UsagePage: 1, Usage: 4}
	b := hidInterfaceMetadata{Path: `\\?\hid#b`, InputReportLength: 64, OutputReportLength: 32, UsagePage: 1, Usage: 4}
	if !hidPathPreferenceLess(b, a) {
		t.Fatal("larger output report must rank ahead of smaller interface")
	}
	if hidPathPreferenceLess(a, b) {
		t.Fatal("smaller output report must not outrank larger interface")
	}
}

func TestD52DiscoverySettlesMixedCompatNativeSameStableWheel(t *testing.T) {
	devs := []Device{
		{InstanceID: `USB\VID_046D&PID_C294\ABC`, StableID: "usbloc:test"},
		{InstanceID: `HID\VID_046D&PID_C29B&COL01\ABC`, StableID: "usbloc:test"},
	}
	if !discoveryNeedsSettle(devs, nil) {
		t.Fatal("C294/native overlap on the same stable wheel must be treated as re-enumeration")
	}
}

func TestD52HIDProductConsensusConfirmsCurrentC294G27Session(t *testing.T) {
	wheels := []WheelDevice{{
		ID: "usbloc:test", SessionID: "container:test", InstanceID: `HID\VID_046D&PID_C294\ABC`,
		Model: modelCompat, ModelKind: WheelModelCompatibility, Supported: true, PnPVerified: true,
	}}
	devs := []Device{{
		InstanceID: `HID\VID_046D&PID_C294\ABC`, PhysicalID: "container:test", ContainerID: "test",
		StableID: "usbloc:test", Model: modelG27, HIDProduct: "Logitech G27 Racing Wheel",
	}}
	got := applySessionHIDProductConsensus(wheels, devs)
	if len(got) != 1 || !got[0].ModelConfirmed || !IsG27Model(got[0].Model) {
		t.Fatalf("expected session-confirmed G27 from explicit HID product, got %+v", got)
	}
	if !strings.Contains(strings.ToLower(got[0].Evidence), "hid") {
		t.Fatalf("expected HID evidence, got %q", got[0].Evidence)
	}
}

func TestD52HIDProductConsensusDoesNotGuessGenericC294(t *testing.T) {
	wheels := []WheelDevice{{
		ID: "usbloc:test", SessionID: "container:test", InstanceID: `HID\VID_046D&PID_C294\ABC`,
		Model: modelCompat, ModelKind: WheelModelCompatibility, Supported: true, PnPVerified: true,
	}}
	devs := []Device{{
		InstanceID: `HID\VID_046D&PID_C294\ABC`, PhysicalID: "container:test", ContainerID: "test",
		StableID: "usbloc:test", Model: modelCompat, HIDProduct: "Logitech Driving Force USB",
	}}
	got := applySessionHIDProductConsensus(wheels, devs)
	if got[0].ModelConfirmed {
		t.Fatal("generic C294 product text must not authorize model-specific commands")
	}
}
