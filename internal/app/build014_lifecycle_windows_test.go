//go:build windows

package app

import "testing"

func TestBuild014PowerBroadcastClassification(t *testing.T) {
	for _, v := range []uintptr{PBT_APMSUSPEND, PBT_APMSTANDBY} {
		if got := classifyPowerBroadcast(v); got != powerEventSuspend {
			t.Fatalf("0x%x classified as %v", v, got)
		}
	}
	for _, v := range []uintptr{PBT_APMRESUMECRITICAL, PBT_APMRESUMESUSPEND, PBT_APMRESUMEAUTOMATIC} {
		if got := classifyPowerBroadcast(v); got != powerEventResume {
			t.Fatalf("0x%x classified as %v", v, got)
		}
	}
	if got := classifyPowerBroadcast(0x9999); got != powerEventNone {
		t.Fatalf("unknown event classified as %v", got)
	}
}
