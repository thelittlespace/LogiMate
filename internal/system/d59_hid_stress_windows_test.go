//go:build windows

package system

import (
	"errors"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestD59HIDStressSoftwareSelfTest(t *testing.T) {
	rep, err := HIDStressSoftwareSelfTest(2048)
	if err != nil {
		t.Fatalf("D5.9 software stress self-test failed: %v", err)
	}
	if !rep.Passed || rep.SafeReportsBuilt != 2048 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	if rep.UnsupportedChecks < 10 || rep.DeadlineChecks < 5 || rep.PolicyChecks < 6 {
		t.Fatalf("stress policy coverage too small: %+v", rep)
	}
}

func TestD59FailureClassification(t *testing.T) {
	cases := map[string]string{
		"HID-Write überschritt die Deadline; Transport gesperrt": "timeout",
		"unvollständiger HID-Write: 2/8 Bytes":                   "partial",
		"HID-Transport ist gesperrt":                             "poisoned",
		"device not connected":                                   "disconnect",
		"random failure":                                         "io-error",
	}
	for in, want := range cases {
		if got := classifyHIDStressFailure(errors.New(in)); got != want {
			t.Fatalf("classify %q: got %q want %q", in, got, want)
		}
	}
}

func TestD59SafeProbeReportIsNonMotorRangeCommand(t *testing.T) {
	s := State{DataDir: t.TempDir()}
	w := WheelDevice{ID: "wheel", Model: modelG27}
	report, desc, err := safeHIDStressReport(s, w)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 8 || report[1] != 0xF8 || report[2] != 0x81 {
		t.Fatalf("D5.9 probe must use the non-motor range report, got % X", report)
	}
	if !strings.Contains(desc, "non-motor") {
		t.Fatalf("probe description must document non-motor behavior: %q", desc)
	}
}

func TestD59TransportMetricsClassifyFailures(t *testing.T) {
	ResetNativeHIDTransportMetrics()
	noteNativeHIDWrite("writefile-overlapped", 14*time.Millisecond, nil)
	noteNativeHIDWrite("writefile-overlapped", 350*time.Millisecond, errors.New("HID-Write überschritt die Deadline; Transport gesperrt"))
	noteNativeHIDWrite("writefile-overlapped", 2*time.Millisecond, errors.New("unvollständiger HID-Write: 4/8 Bytes"))
	noteNativeHIDCancel()
	noteNativeHIDPoison()
	got := NativeHIDTransportMetricsSnapshot()
	if got.Writes != 3 || got.SuccessfulWrites != 1 || got.FailedWrites != 2 || got.Timeouts != 1 || got.PartialWrites != 1 {
		t.Fatalf("unexpected metrics: %+v", got)
	}
	if got.Cancels != 1 || got.PoisonEvents != 1 || got.MaxWriteLatency != 350*time.Millisecond {
		t.Fatalf("unexpected safety metrics: %+v", got)
	}
}

func TestD59LiveReportPassRules(t *testing.T) {
	normal := HIDStressLiveReport{CyclesRequested: 50, CyclesCompleted: 50, ValidInputSamples: 10, LeaseReleased: true}
	if !normal.Passed() {
		t.Fatalf("normal clean run should pass: %+v", normal)
	}
	normal.WriteFailures = 1
	if normal.Passed() {
		t.Fatal("normal run with write failure must fail")
	}
	yank := HIDStressLiveReport{DisconnectExpected: true, DisconnectObserved: true, WriteFailures: 1, LeaseReleased: true}
	if !yank.Passed() {
		t.Fatalf("adverse run should pass only after observed disconnect + lease release: %+v", yank)
	}
	yank.LeaseReleased = false
	if yank.Passed() {
		t.Fatal("adverse run without confirmed lease release must fail")
	}
}

func TestD59AmbiguousWritePoisonsTransport(t *testing.T) {
	ResetNativeHIDTransportMetrics()
	tr := &nativeHIDTransport{handle: syscall.InvalidHandle, backend: "writefile-overlapped"}
	err := tr.failClosedAmbiguousLocked(errors.New("partial completion"))
	if err == nil || !tr.poisoned || !tr.closed {
		t.Fatalf("ambiguous write must poison and close transport: err=%v state=%+v", err, tr)
	}
	if NativeHIDTransportMetricsSnapshot().PoisonEvents != 1 {
		t.Fatalf("poison event was not recorded: %+v", NativeHIDTransportMetricsSnapshot())
	}
}
