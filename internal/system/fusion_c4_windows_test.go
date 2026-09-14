//go:build windows

package system

import (
	"strings"
	"testing"
)

func TestFusionC4RawFieldsMatchOpenG27Indices(t *testing.T) {
	report := []byte{0x00, 0x08, 0x00, 0x00, 0x34, 0x12, 0x11, 0x22, 0x33, 0x01, 0x02, 0x03}
	steer, throttle, brake, clutch, ok := g27RawAxisFields(report)
	if !ok {
		t.Fatal("report rejected")
	}
	if steer != 0x1234 || throttle != 0x11 || brake != 0x22 || clutch != 0x33 {
		t.Fatalf("got %04X/%02X/%02X/%02X", steer, throttle, brake, clutch)
	}
}

func TestFusionC4ObserverRecordsMatch(t *testing.T) {
	fusionC4Parity.Lock()
	fusionC4Parity.Reports = 0
	fusionC4Parity.Matches = 0
	fusionC4Parity.Mismatches = 0
	fusionC4Parity.ParseErrors = 0
	fusionC4Parity.LastDetail = ""
	fusionC4Parity.Unlock()
	report := []byte{0x00, 0x08, 0x00, 0x00, 0x00, 0x80, 0xFF, 0xFF, 0xFF, 0x67, 0x78, 0x98}
	steer, th, br, cl, ok := g27RawAxisFields(report)
	if !ok {
		t.Fatal("report rejected")
	}
	observeFusionC4RawFields(report, steer, th, br, cl)
	summary := FusionC4ParserSummary()
	if !strings.Contains(summary, "MATCH: 1") || strings.Contains(summary, "DIFF: 1") {
		t.Fatalf("unexpected summary: %s", summary)
	}
}
