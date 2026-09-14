//go:build windows

package system

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// FusionC7Check is one parity/cutover gate in the final Path-C release.
// PASS means LogiMate has a native implementation, WARN means the feature is
// available but still benefits from real-hardware validation, and BLOCK means
// the selected state is not safe for LogiMate-native Modern operation.
type FusionC7Check struct {
	Name   string
	Status string
	Detail string
}

type FusionC7Report struct {
	GeneratedAt time.Time
	Ready       bool
	Pass        int
	Warn        int
	Block       int
	Checks      []FusionC7Check
}

func (r *FusionC7Report) add(name, status, detail string) {
	status = strings.ToUpper(strings.TrimSpace(status))
	switch status {
	case "PASS":
		r.Pass++
	case "BLOCK":
		r.Block++
	default:
		status = "WARN"
		r.Warn++
	}
	r.Checks = append(r.Checks, FusionC7Check{Name: name, Status: status, Detail: detail})
}

// FusionC7CoreGate validates the historical compatibility core without touching HID.
// It is deliberately safe enough to run before an elevated driver migration.
func FusionC7CoreGate() error {
	// D5 keeps the historical function name for compatibility, but the gate is
	// now entirely LogiMate-owned and has no runtime/reference-package dependency.
	if wheelengine.SliderToForceByte(-100) != 0x00 || wheelengine.SliderToForceByte(0) != 0x80 || wheelengine.SliderToForceByte(100) != 0xFF {
		return fmt.Errorf("native force scaling gate failed")
	}
	vectors := []struct{ got, want []byte }{
		{wheelengine.ConstantForce(0x80), []byte{0x11, 0x08, 0x80, 0x80, 0, 0, 0}},
		{wheelengine.SetRange(900), []byte{0xF8, 0x81, 0x84, 0x03, 0, 0, 0}},
		{wheelengine.NativeSwitch(0x04), []byte{0xF8, 0x09, 0x04, 0x01, 0, 0, 0}},
		{wheelengine.SetLeds(0x1F), []byte{0xF8, 0x12, 0x1F, 0, 0, 0, 0}},
	}
	for _, v := range vectors {
		if !bytes.Equal(v.got, v.want) {
			return fmt.Errorf("native protocol gate failed: got % X want % X", v.got, v.want)
		}
	}
	if _, err := FusionC3DryRun(); err != nil {
		return fmt.Errorf("native scheduler: %w", err)
	}
	raw := []byte{0x00, 0x08, 0x00, 0x00, 0x00, 0x80, 0xFF, 0xFF, 0xFF, 0x67, 0x78, 0x98}
	st, err := wheelengine.ParseNativeInputReport(wheelengine.ModelG27, raw)
	if err != nil || st.Steering != 0x8000 || st.ThrottleRaw != 0xFF || st.BrakeRaw != 0xFF || st.ClutchRaw != 0xFF {
		return fmt.Errorf("native parser gate failed: %+v %v", st, err)
	}
	if _, err := ParseLogiMateJSONTelemetry([]byte(`{"rpm":1109,"rpmMax":5600,"rpmRedline":5320,"force":-0.0765498,"physics":true,"playerControl":true}`)); err != nil {
		return fmt.Errorf("native telemetry gate failed: %w", err)
	}
	return nil
}

// FusionC4ParitySnapshot exposes only counters required by the C7 gate.
func FusionC4ParitySnapshot() (reports, matches, mismatches, parseErrors uint64, last time.Time) {
	fusionC4Parity.RLock()
	defer fusionC4Parity.RUnlock()
	return fusionC4Parity.Reports, fusionC4Parity.Matches, fusionC4Parity.Mismatches, fusionC4Parity.ParseErrors, fusionC4Parity.LastAt
}

// BuildFusionC7ParityReport decides whether the currently selected G27 can use
// LogiMate Native as the normal Modern engine. Hardware-validation gaps are
// WARN, not silently promoted to PASS. External wheel software is never a Modern-mode requirement.
func BuildFusionC7ParityReport(s State) FusionC7Report {
	r := FusionC7Report{GeneratedAt: time.Now()}
	if err := FusionC7CoreGate(); err != nil {
		r.add("C1–C6 Core", "BLOCK", err.Error())
	} else {
		r.add("C1–C6 Core", "PASS", "LogiMate-eigene Skalierung, Protokollbuilder, Scheduler, Parser und Telemetrie bestehen den hardwarefreien Gate-Test.")
	}

	w, selected := SelectedWheel(s)
	if !selected || !HasActionableSelectedWheel(s) {
		r.add("G27 Zielgerät", "BLOCK", "Kein eindeutig ausgewähltes, modellbestätigtes Logitech-Wheel.")
	} else if !IsG27Model(w.Model) {
		r.add("G27 Zielgerät", "BLOCK", "C7-Cutover ist Path-C zunächst G27-spezifisch; ausgewählt ist "+w.Model+".")
	} else if !w.PnPVerified {
		r.add("G27 Zielgerät", "BLOCK", "Das G27 ist nicht durch eine native PnP-PID verifiziert.")
	} else {
		r.add("G27 Zielgerät", "PASS", "StableWheelID="+w.ID+" · Modell/PnP bestätigt.")
	}

	if strings.Contains(strings.ToLower(s.ActiveMode), "generic") {
		r.add("Windows Modern Mode", "PASS", s.ActiveMode)
	} else {
		r.add("Windows Modern Mode", "BLOCK", "Aktuell "+s.ActiveMode+"; LogiMate Native ist erst im Generic-HID-/Modern-Modus der aktive Standardpfad.")
	}

	legacyDiffs := 0
	legacyTotal := 0
	for _, c := range FusionC2ProtocolComparisons() {
		legacyTotal++
		if !c.Match {
			legacyDiffs++
		}
	}
	if legacyDiffs == 0 {
		r.add("C2 Legacy Builder Shadow", "PASS", fmt.Sprintf("%d/%d alte LogiMate-Builder stimmen mit der archivierten Protokollreferenz überein.", legacyTotal, legacyTotal))
	} else {
		r.add("C2 Legacy Builder Shadow", "WARN", fmt.Sprintf("%d/%d alte Test-/Kompatibilitäts-Builder weichen noch ab. Der produktive Native-Pfad verwendet bereits LogiMates eigene Protokoll-Builder; die Altpfade bleiben nur als historische Diagnose erhalten.", legacyDiffs, legacyTotal))
	}

	lease := NativeOutputLeaseSnapshot()
	if lease.Active {
		r.add("Output Ownership", "PASS", "Zentraler LogiMate-Output-Lease aktiv für das ausgewählte Wheel.")
	} else {
		r.add("Output Ownership", "PASS", "Kein aktiver Motor-Writer; LogiMate benötigt keine externe Wheel-Runtime.")
	}

	reports, matches, diffs, parseErrs, last := FusionC4ParitySnapshot()
	switch {
	case diffs > 0 || parseErrs > 0:
		r.add("G27 Input Parser", "BLOCK", fmt.Sprintf("C4 live: reports=%d match=%d diff=%d errors=%d", reports, matches, diffs, parseErrs))
	case reports == 0:
		r.add("G27 Input Parser", "WARN", "Noch kein echter C29B-HID-Report in dieser Sitzung beobachtet; Golden-Vector-Test ist PASS.")
	default:
		r.add("G27 Input Parser", "PASS", fmt.Sprintf("C4 live: %d/%d MATCH · letzter Report %s", matches, reports, last.Format("15:04:05.000")))
	}

	if G27DirectHIDConnected() {
		r.add("Direct HID", "PASS", "Shared C29B Direct-HID Reader verbunden.")
	} else if selected && IsG27Model(w.Model) && strings.Contains(strings.ToLower(s.ActiveMode), "generic") {
		r.add("Direct HID", "WARN", "C29B ist ausgewählt, Direct-HID Reader meldet aktuell noch keine aktive Verbindung.")
	} else {
		r.add("Direct HID", "WARN", "Wird nach dem Modern-/C29B-Wechsel erneut geprüft.")
	}

	if _, err := FusionC3DryRun(); err != nil {
		r.add("Native FFB Scheduler", "BLOCK", err.Error())
	} else {
		r.add("Native FFB Scheduler", "PASS", "6-ms Scheduler, Slew, Watchdog und Panic/Center Dry-Run PASS.")
	}

	r.add("Eigenständige Runtime", "PASS", "LogiMate sucht, installiert und startet keine externe Wheel-Runtime. Legacy-OpenG27-Daten bleiben nur als Offline-Importformat erhalten.")

	// We cannot manufacture physical-matrix evidence in software. Keep that
	// visible until the real wheel has been exercised by the user.
	r.add("Physische G27-Matrix", "WARN", "Live-FFB, LEDs und Reconnect müssen weiterhin am echten G27 validiert werden; die Softwareprüfung behauptet dafür keinen künstlichen PASS.")

	r.Ready = r.Block == 0
	return r
}

func FusionC7ParitySummary(s State) string {
	r := BuildFusionC7ParityReport(s)
	var b strings.Builder
	state := "READY"
	if !r.Ready {
		state = "BLOCKED"
	}
	fmt.Fprintf(&b, "C7 Native Cutover: %s · PASS=%d WARN=%d BLOCK=%d · %s\r\n", state, r.Pass, r.Warn, r.Block, r.GeneratedAt.Format("15:04:05.000"))
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "- [%s] %s: %s\r\n", c.Status, c.Name, c.Detail)
	}
	return strings.TrimRight(b.String(), "\r\n")
}

func FusionC7ModernNativeReady(s State) bool { return BuildFusionC7ParityReport(s).Ready }
