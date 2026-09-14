//go:build windows

package system

import (
	"bytes"
	"testing"
)

func TestRawPathInstanceIDMatchesSetupAPIForm(t *testing.T) {
	path := `\\?\HID#VID_046D&PID_C29B#7&26ef9b37&2&0000#{4d1e55b2-f16f-11cf-88cb-001111000030}`
	want := `HID\VID_046D&PID_C29B\7&26EF9B37&2&0000`
	if got := rawPathInstanceID(path); got != want {
		t.Fatalf("rawPathInstanceID=%q want %q", got, want)
	}
}

func TestRawPathsForSelectedG27DisambiguatesIdenticalWheels(t *testing.T) {
	w := WheelDevice{InstanceID: `USB\VID_046D&PID_C29B\PORT_A`, InterfaceIDs: []string{`USB\VID_046D&PID_C29B\PORT_A`, `HID\VID_046D&PID_C29B\HID_A`}}
	paths := []string{
		`\\?\HID#VID_046D&PID_C29B#HID_A#{4d1e55b2-f16f-11cf-88cb-001111000030}`,
		`\\?\HID#VID_046D&PID_C29B#HID_B#{4d1e55b2-f16f-11cf-88cb-001111000030}`,
	}
	got := rawPathsForWheel(w, paths, pidG27)
	if len(got) != 1 || got[0] != paths[0] {
		t.Fatalf("selected raw paths=%v want [%s]", got, paths[0])
	}
}

func TestNativeModeReportsUseTwoStepSetReportSequence(t *testing.T) {
	reports := nativeModeReports(0x04)
	if len(reports) != 2 {
		t.Fatalf("reports=%d want 2", len(reports))
	}
	want1 := []byte{0x00, 0xF8, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x00}
	want2 := []byte{0x00, 0xF8, 0x09, 0x04, 0x01, 0x00, 0x00, 0x00}
	if !bytes.Equal(reports[0], want1) || !bytes.Equal(reports[1], want2) {
		t.Fatalf("reports=% x / % x", reports[0], reports[1])
	}
}
