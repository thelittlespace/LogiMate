//go:build windows

package system

import "testing"

func TestD1G27DirectHIDCarriesCanonicalSourceAndLayout(t *testing.T) {
	report := []byte{0, 0x08, 0x00, 0x00, 0x00, 0x80, 0xFF, 0xFF, 0xFF, 0, 0, 0}
	j, ok := parseG27NativeReport(report)
	if !ok {
		t.Fatal("valid G27 report rejected")
	}
	if j.InputSource != "direct-hid" || j.LayoutID != "g27-c29b-v1" || !j.SampleValid || j.SampleAt.IsZero() {
		t.Fatalf("canonical direct-HID metadata missing: %+v", j)
	}
}

func TestD1ClassicLayoutsComeFromModelAdapter(t *testing.T) {
	g25 := make([]byte, 13)
	g25[0] = 0
	g25[1] = 0x08
	g25[4] = 0
	g25[5] = 0x80
	g25[6] = 0xFF
	g25[7] = 0xFF
	g25[12] = 0xFF
	j, ok := parseClassicNativeReport(modelG25, g25)
	if !ok || j.LayoutID != "g25-c299-v1" {
		t.Fatalf("G25 adapter layout: %+v ok=%v", j, ok)
	}
	dfgt := make([]byte, 12)
	dfgt[0] = 0x08
	dfgt[3] = 0
	dfgt[4] = 0x80
	dfgt[5] = 0xFF
	dfgt[6] = 0xFF
	j, ok = parseClassicNativeReport(modelDFGT, dfgt)
	if !ok || j.LayoutID != "dfgt-c29a-v1" {
		t.Fatalf("DFGT adapter layout: %+v ok=%v", j, ok)
	}
}
