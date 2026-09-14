//go:build windows

package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// HIDStressSoftwareReport is deterministic software evidence for D5.9. It
// deliberately does not claim real USB/HID certification; it proves the
// fail-closed policy helpers and high-volume safe report generation remain
// internally coherent before a physical stress run is attempted.
type HIDStressSoftwareReport struct {
	Passed            bool          `json:"passed"`
	Iterations        int           `json:"iterations"`
	SafeReportsBuilt  int           `json:"safeReportsBuilt"`
	PolicyChecks      int           `json:"policyChecks"`
	UnsupportedChecks int           `json:"unsupportedChecks"`
	DeadlineChecks    int           `json:"deadlineChecks"`
	Duration          time.Duration `json:"duration"`
	Detail            string        `json:"detail"`
}

// HIDStressLiveReport is physical/runtime evidence produced by the guided
// D5.9 probe. It never edits docs/HARDWARE_CERTIFICATION.json automatically.
type HIDStressLiveReport struct {
	SchemaVersion      int                       `json:"schemaVersion"`
	AppVersion         string                    `json:"appVersion"`
	WheelID            string                    `json:"wheelId"`
	SessionID          string                    `json:"sessionId"`
	Model              string                    `json:"model"`
	Mode               string                    `json:"mode"`
	PathFingerprint    string                    `json:"pathFingerprint"`
	StartedAt          time.Time                 `json:"startedAt"`
	FinishedAt         time.Time                 `json:"finishedAt"`
	CyclesRequested    int                       `json:"cyclesRequested"`
	CyclesCompleted    int                       `json:"cyclesCompleted"`
	InputSamples       int                       `json:"inputSamples"`
	ValidInputSamples  int                       `json:"validInputSamples"`
	WriteFailures      int                       `json:"writeFailures"`
	DisconnectExpected bool                      `json:"disconnectExpected"`
	DisconnectObserved bool                      `json:"disconnectObserved"`
	LeaseReleased      bool                      `json:"leaseReleased"`
	MaxWriteMillis     int64                     `json:"maxWriteMillis"`
	LastError          string                    `json:"lastError,omitempty"`
	TransportBefore    NativeHIDTransportMetrics `json:"transportBefore"`
	TransportAfter     NativeHIDTransportMetrics `json:"transportAfter"`
}

func (r HIDStressLiveReport) Passed() bool {
	if !r.LeaseReleased {
		return false
	}
	if r.DisconnectExpected {
		return r.DisconnectObserved && r.WriteFailures > 0
	}
	return r.WriteFailures == 0 && r.CyclesCompleted == r.CyclesRequested && r.ValidInputSamples > 0
}

func classifyHIDStressFailure(err error) string {
	if err == nil {
		return "ok"
	}
	s := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(s, "deadline") || strings.Contains(s, "zeitlimit") || strings.Contains(s, "timeout"):
		return "timeout"
	case strings.Contains(s, "unvollständig") || strings.Contains(s, "partial"):
		return "partial"
	case strings.Contains(s, "gesperrt") || strings.Contains(s, "poison"):
		return "poisoned"
	case strings.Contains(s, "device") && strings.Contains(s, "not"):
		return "disconnect"
	case strings.Contains(s, "nicht verbunden") || strings.Contains(s, "abgebrochen") || strings.Contains(s, "invalid handle"):
		return "disconnect"
	default:
		return "io-error"
	}
}

func HIDStressSoftwareSelfTest(iterations int) (HIDStressSoftwareReport, error) {
	if iterations < 256 {
		iterations = 256
	}
	if iterations > 100000 {
		iterations = 100000
	}
	start := time.Now()
	rep := HIDStressSoftwareReport{Iterations: iterations}

	// Exercise the exact safe report builders used by the live probe. Vary the
	// supported range so malformed values cannot accidentally enter the test.
	ranges := []int{270, 360, 540, 720, 900}
	for i := 0; i < iterations; i++ {
		r, err := BuildClassicRangeReport(ranges[i%len(ranges)])
		if err != nil || len(r) != 8 {
			return rep, fmt.Errorf("safe range report iteration %d: len=%d err=%v", i, len(r), err)
		}
		rep.SafeReportsBuilt++
	}

	// Regression-check the only synchronous errors that are allowed to switch
	// from OVERLAPPED WriteFile to HidD_SetOutputReport. Everything else must
	// stay on the fail-closed path rather than being silently retried.
	supported := []syscall.Errno{1, 50, 87}
	for _, e := range supported {
		rep.UnsupportedChecks++
		if !unsupportedOverlappedWrite(e) {
			return rep, fmt.Errorf("errno %d no longer permits the known-safe compatibility fallback", e)
		}
	}
	for _, e := range []syscall.Errno{2, 5, 6, 31, 995, 996, 1167} {
		rep.UnsupportedChecks++
		if unsupportedOverlappedWrite(e) {
			return rep, fmt.Errorf("errno %d would incorrectly retry through HidD compatibility", e)
		}
	}

	// Deadline conversion must be bounded and nonzero for a positive budget.
	for _, d := range []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 250 * time.Millisecond, defaultNativeWriteTimeout, openG27SwitchWriteTimeout} {
		rep.DeadlineChecks++
		if got := contextWaitMilliseconds(nilContextDeadline{deadline: time.Now().Add(d)}, d); got == 0 {
			return rep, fmt.Errorf("positive deadline %s collapsed to zero wait", d)
		}
	}

	// Error classification is diagnostic-only, but keep it deterministic so
	// exported D5.9 reports remain comparable across releases.
	cases := []struct {
		err  error
		want string
	}{
		{nil, "ok"},
		{errors.New("HID-Write überschritt die Deadline; Transport wurde fail-closed gesperrt"), "timeout"},
		{errors.New("unvollständiger HID-Write: 4/8 Bytes"), "partial"},
		{errors.New("HID-Transport ist nach einem unbestätigten I/O-Abbruch gesperrt"), "poisoned"},
		{errors.New("device not connected"), "disconnect"},
		{errors.New("random I/O failure"), "io-error"},
	}
	for _, tc := range cases {
		rep.PolicyChecks++
		if got := classifyHIDStressFailure(tc.err); got != tc.want {
			return rep, fmt.Errorf("failure classification %q: got %s want %s", tc.err, got, tc.want)
		}
	}

	rep.Passed = true
	rep.Duration = time.Since(start)
	rep.Detail = fmt.Sprintf("%d safe report builds, %d fallback-policy checks, %d deadline checks", rep.SafeReportsBuilt, rep.UnsupportedChecks, rep.DeadlineChecks)
	return rep, nil
}

// deadlineContext is the tiny subset contextWaitMilliseconds needs. We keep a
// real context adapter below to avoid changing the production helper signature.
type nilContextDeadline struct{ deadline time.Time }

func (n nilContextDeadline) Deadline() (time.Time, bool) { return n.deadline, true }
func (n nilContextDeadline) Done() <-chan struct{}       { return nil }
func (n nilContextDeadline) Err() error                  { return nil }
func (n nilContextDeadline) Value(key any) any           { return nil }

func safeHIDStressReport(s State, w WheelDevice) ([]byte, string, error) {
	p := ReadActiveNativeEngineProfile(s.DataDir, w.ID)
	degrees := p.RotationDegrees
	if degrees == 0 {
		degrees = 900
	}
	r, err := BuildClassicRangeReport(degrees)
	return r, fmt.Sprintf("range=%d° (idempotent, non-motor)", degrees), err
}

// RunHIDStressLiveProbe repeatedly opens, writes and closes the selected HID
// output path while concurrently sampling input. It uses only an idempotent
// range command, never starts a motor effect, and owns the normal output lease
// for the whole run so game/FFB writers cannot race it.
func RunHIDStressLiveProbe(s State, cycles int, expectDisconnect bool) (HIDStressLiveReport, error) {
	if cycles < 1 {
		cycles = 1
	}
	if cycles > 1000 {
		cycles = 1000
	}
	w, _, err := validateNativeOutputTarget(s)
	if err != nil {
		return HIDStressLiveReport{}, err
	}
	if NativeFFBSnapshot().Active || NativeOutputSnapshot().Active {
		return HIDStressLiveReport{}, errors.New("HID-Stresstest startet nicht, solange Motor-/FFB-Output aktiv ist")
	}
	lease, err := acquireNativeOutputLease(s, "d5.9-hid-stress", false)
	if err != nil {
		return HIDStressLiveReport{}, err
	}
	rep := HIDStressLiveReport{
		SchemaVersion: 1, AppVersion: s.AppVersion, WheelID: w.ID, SessionID: w.SessionID,
		Model: canonicalWheelModel(w.Model), Mode: s.ActiveMode, PathFingerprint: shortPathFingerprint(lease.Path),
		StartedAt: time.Now().UTC(), CyclesRequested: cycles, DisconnectExpected: expectDisconnect,
		TransportBefore: NativeHIDTransportMetricsSnapshot(),
	}
	report, reportDesc, buildErr := safeHIDStressReport(s, w)
	if buildErr != nil {
		_ = releaseNativeOutputLease(lease)
		return rep, buildErr
	}
	_ = reportDesc // kept for future report variants and explicit audit readability.

	delay := 12 * time.Millisecond
	if expectDisconnect {
		delay = 40 * time.Millisecond
	}
	for i := 0; i < cycles; i++ {
		started := time.Now()
		writeErr := writeNativeOutputPath(lease.Path, report)
		elapsed := time.Since(started).Milliseconds()
		if elapsed > rep.MaxWriteMillis {
			rep.MaxWriteMillis = elapsed
		}
		if writeErr != nil {
			rep.WriteFailures++
			rep.LastError = classifyHIDStressFailure(writeErr) + ": " + writeErr.Error()
			if expectDisconnect {
				rep.DisconnectObserved = true
			}
			break
		}
		rep.CyclesCompleted++
		// Sampling every fifth cycle keeps the input reader involved without
		// making a stale UI snapshot the authority for output safety.
		if i%5 == 0 {
			rep.InputSamples++
			j := ReadPreferredWheelInput(s)
			if j.Found && j.SampleValid {
				rep.ValidInputSamples++
			}
		}
		time.Sleep(delay)
	}
	rep.FinishedAt = time.Now().UTC()
	rep.TransportAfter = NativeHIDTransportMetricsSnapshot()
	rep.LeaseReleased = releaseNativeOutputLease(lease)

	if expectDisconnect && !rep.DisconnectObserved {
		return rep, errors.New("während des adverse-I/O-Fensters wurde kein USB/HID-Abbruch beobachtet")
	}
	if !expectDisconnect && rep.WriteFailures > 0 {
		return rep, errors.New(rep.LastError)
	}
	if !rep.LeaseReleased {
		return rep, errors.New("HID-Stress-Lease konnte nach dem Test nicht bestätigt freigegeben werden")
	}
	return rep, nil
}

func shortPathFingerprint(path string) string {
	path = strings.TrimSpace(strings.ToLower(path))
	if path == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(path))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

func ExportHIDStressReport(s State, rep HIDStressLiveReport) (string, error) {
	dir := filepath.Join(s.DataDir, "Certification")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	base := filepath.Join(dir, "hid-stress-"+stamp)
	publicRep := rep
	publicRep.WheelID = redactIdentifier(rep.WheelID)
	publicRep.SessionID = redactIdentifier(rep.SessionID)
	jb, err := json.MarshalIndent(publicRep, "", "  ")
	if err != nil {
		return "", err
	}
	if err := AtomicWriteFile(base+".json", append(jb, '\n'), 0600); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# LogiMate HID Stress Evidence\n\n")
	fmt.Fprintf(&b, "- App: %s\n- Wheel: %s (%s)\n- Session: %s\n- Mode: %s\n- Started: %s\n- Finished: %s\n", rep.AppVersion, rep.Model, redactIdentifier(rep.WheelID), redactIdentifier(rep.SessionID), rep.Mode, rep.StartedAt.Format(time.RFC3339), rep.FinishedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- Cycles: %d/%d\n- Input samples valid: %d/%d\n- Write failures: %d\n- Max write: %d ms\n- Disconnect expected/observed: %v/%v\n- Lease released: %v\n- Result: %v\n", rep.CyclesCompleted, rep.CyclesRequested, rep.ValidInputSamples, rep.InputSamples, rep.WriteFailures, rep.MaxWriteMillis, rep.DisconnectExpected, rep.DisconnectObserved, rep.LeaseReleased, rep.Passed())
	if rep.LastError != "" {
		fmt.Fprintf(&b, "- Last error: %s\n", rep.LastError)
	}
	fmt.Fprintf(&b, "\n## Transport counters before\n\n%s\n\n## Transport counters after\n\n%s\n", FormatNativeHIDTransportMetrics(rep.TransportBefore), FormatNativeHIDTransportMetrics(rep.TransportAfter))
	fmt.Fprintf(&b, "\n> This file is physical/runtime evidence only. It does not automatically set `hidStressValidated=true` in the repository Stable certification manifest.\n")
	if err := AtomicWriteFile(base+".md", []byte(b.String()), 0600); err != nil {
		return "", err
	}
	return base + ".md", nil
}
