//go:build windows

package system

import (
	"strings"
	"testing"
)

func testCertificationState(t *testing.T) State {
	t.Helper()
	d := t.TempDir()
	w := WheelDevice{
		ID: "wheel-1", SessionID: "session-1", Name: "G27 Racing Wheel", Model: ModelG27,
		Mode: "Modern / Generic HID", ModelKind: WheelModelG27, ModeKind: OperatingModeModern,
		Evidence: "test", InstanceID: `HID\VID_046D&PID_C29B\A`, Supported: true,
		ModelConfirmed: true, PnPVerified: true,
	}
	return State{DataDir: d, AppVersion: "0.5.7-alpha", Wheels: []WheelDevice{w}, SelectedWheelID: w.ID, WheelModel: w.Model, ActiveMode: w.Mode}
}

func TestD57CertificationStartsPendingAndPersistsEvidence(t *testing.T) {
	s := testCertificationState(t)
	p := HardwareCertificationProgress(s)
	if p.Required == 0 || p.Passed != 0 || p.Pending != p.Required {
		t.Fatalf("unexpected initial progress: %+v", p)
	}
	if err := RecordHardwareCertificationResult(s, "buttons", true, "physical button transition observed"); err != nil {
		t.Fatal(err)
	}
	p = HardwareCertificationProgress(s)
	if p.Passed != 1 || p.Pending != p.Required-1 {
		t.Fatalf("unexpected persisted progress: %+v", p)
	}
}

func TestD57AutoEvidenceAcceptsNativeG27PIDOnly(t *testing.T) {
	s := testCertificationState(t)
	check, pass, evidence := AutoCertificationEvidence(s)
	if check != "native-mode-switch" || !pass || !strings.Contains(evidence, "C29B") {
		t.Fatalf("unexpected auto evidence: %q %v %q", check, pass, evidence)
	}
	s.Wheels[0].InstanceID = `HID\VID_046D&PID_C294\A`
	_, pass, _ = AutoCertificationEvidence(s)
	if pass {
		t.Fatal("compatibility PID must not auto-pass native-mode certification")
	}
}

func TestD57ReportRequiresNonEmptyEvidence(t *testing.T) {
	s := testCertificationState(t)
	if err := RecordHardwareCertificationResult(s, "steering", true, ""); err == nil {
		t.Fatal("empty evidence must be rejected")
	}
}

func TestD57G27IncludesShifterRequirement(t *testing.T) {
	s := testCertificationState(t)
	p := HardwareCertificationProgress(s)
	found := false
	for _, it := range p.Items {
		if it.Check == "shifter" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("G27 must require shifter certification")
	}
}
