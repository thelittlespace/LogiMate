//go:build windows

package system

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// NativeOutputStatus is intentionally small and diagnostic-friendly. Native
// output remains experimental until the physical hardware matrix is complete.
type NativeOutputStatus struct {
	Active         bool
	WheelID        string
	Model          string
	LastCommand    string
	LastError      string
	CommandCount   int
	EmergencyStops int
	WatchdogStops  int
	LastWrite      time.Time
}

var nativeOutput = struct {
	sync.Mutex
	status NativeOutputStatus
	timer  *time.Timer
	lease  *nativeOutputLeaseToken
}{}

func outputReport(payload ...byte) []byte {
	r := make([]byte, 8)
	copy(r[1:], payload)
	return r
}

func BuildClassicRangeReport(degrees int) ([]byte, error) {
	if degrees < 40 || degrees > 900 {
		return nil, fmt.Errorf("Lenkwinkel %d° außerhalb des sicheren Bereichs 40–900°", degrees)
	}
	return wheelengine.WithReportID(wheelengine.SetRange(degrees)), nil
}

func BuildG27LEDReport(mask byte) ([]byte, error) {
	if mask > 0x1F {
		return nil, fmt.Errorf("LED-Maske 0x%02X außerhalb 0x00–0x1F", mask)
	}
	return wheelengine.WithReportID(wheelengine.SetLeds(mask)), nil
}

func BuildClassicAutocenterReport(percent, ramp int) ([]byte, error) {
	if percent < 0 || percent > 30 {
		return nil, fmt.Errorf("experimentelles Autocenter ist auf 0–30 %% begrenzt")
	}
	if ramp < 0 || ramp > 7 {
		return nil, fmt.Errorf("Autocenter-Ramp muss 0–7 sein")
	}
	// FE/0D byte 4 is the clip/saturation value, not a ramp value. Earlier
	// builds wrote the UI ramp (normally 2) into that field, effectively making
	// Autocenter almost imperceptible. Use the same proven 0x80 clip as the
	// OpenG27 spring transaction; keep `ramp` only as a validated API-compat
	// parameter until we expose a real software-side ramp.
	_ = ramp
	strength := percentToSpringSlope(percent)
	return outputReport(0xFE, 0x0D, strength, strength, 0x80, 0x00, 0x00), nil
}

func BuildClassicAutocenterEnableReport() []byte {
	return wheelengine.WithReportID(wheelengine.SpringEnable())
}

func BuildClassicAutocenterOffReport() []byte {
	return wheelengine.WithReportID(wheelengine.SpringOff())
}

func nativeOutputPID(model string) (string, bool) {
	c, ok := CapabilitiesForWheelModel(model)
	if !ok || !c.HasNativeFFB || c.NativePID == "" {
		return "", false
	}
	return c.NativePID, true
}

func validateNativeOutputTarget(s State) (WheelDevice, string, error) {
	if len(s.Wheels) != 1 {
		return WheelDevice{}, "", errors.New("experimentelle native Ausgabe erfordert genau ein angeschlossenes unterstütztes Lenkrad")
	}
	if !HasActionableSelectedWheel(s) {
		return WheelDevice{}, "", errors.New("Lenkradmodell ist nicht sicher bestätigt")
	}
	w, ok := SelectedWheel(s)
	if !ok || !w.ModelConfirmed || !w.PnPVerified {
		return WheelDevice{}, "", errors.New("ausgewähltes Wheel ist nicht PnP-verifiziert und modellbestätigt")
	}
	if wheelModeKind(w) != OperatingModeModern && ClassifyOperatingMode(s.ActiveMode) != OperatingModeModern {
		return WheelDevice{}, "", fmt.Errorf("native Ausgabe ist nur im Modern/Generic-HID-Modus erlaubt; aktuell: %s", s.ActiveMode)
	}
	caps, ok := WheelCapabilitiesForDevice(w)
	if !ok || !caps.HasNativeFFB || caps.NativePID == "" {
		return WheelDevice{}, "", errors.New("für dieses Modell existiert kein freigegebener nativer Output-Pfad")
	}
	pid := caps.NativePID
	paths := rawPathsForWheel(w, s.RawInputDevices, pid)
	if len(paths) == 0 && len(s.Wheels) == 1 {
		paths = rawPathsForPID(s.RawInputDevices, pid)
	}
	if len(paths) != 1 {
		return WheelDevice{}, "", fmt.Errorf("nativer HID-Output-Pfad ist nicht eindeutig (%d Treffer)", len(paths))
	}
	return w, paths[0], nil
}

func writeNativeOutputPath(path string, reports ...[]byte) error {
	return writeNativeReports(path, reports...)
}

// NativeOutputProbeResult describes a non-motor HID access probe. It opens the
// exact selected native interface through the same OutputLease and transport
// factory used by real output, but deliberately sends no report. D6.1 uses it
// to distinguish "writer can be opened" from "an effect write succeeded".
type NativeOutputProbeResult struct {
	Backend   string
	ShareMode string
}

func ProbeNativeOutputTransport(s State) (NativeOutputProbeResult, error) {
	lease, err := acquireNativeOutputLease(s, "hid-probe", false)
	if err != nil {
		return NativeOutputProbeResult{}, err
	}
	released := false
	defer func() {
		if !released {
			releaseNativeOutputLease(lease)
		}
	}()

	tr, err := openNativeHIDTransport(lease.Path)
	if err != nil {
		releaseNativeOutputLease(lease)
		released = true
		return NativeOutputProbeResult{}, err
	}
	result := NativeOutputProbeResult{Backend: tr.Backend(), ShareMode: tr.ShareMode()}
	closeErr := tr.Close()
	if !releaseNativeOutputLease(lease) {
		released = true
		if closeErr != nil {
			return result, errors.Join(closeErr, errors.New("HID-Probe-Lease konnte nicht eindeutig freigegeben werden"))
		}
		return result, errors.New("HID-Probe-Lease konnte nicht eindeutig freigegeben werden")
	}
	released = true
	if closeErr != nil {
		return result, closeErr
	}
	return result, nil
}

func recordNativeOutput(w WheelDevice, command string, active bool, err error) {
	nativeOutput.Lock()
	defer nativeOutput.Unlock()
	nativeOutput.status.Active = err == nil && active
	nativeOutput.status.WheelID = w.ID
	nativeOutput.status.Model = w.Model
	nativeOutput.status.LastCommand = command
	nativeOutput.status.LastWrite = time.Now()
	nativeOutput.status.CommandCount++
	if err != nil {
		nativeOutput.status.LastError = err.Error()
	} else {
		nativeOutput.status.LastError = ""
	}
}

func stopWatchdogLocked() {
	if nativeOutput.timer != nil {
		nativeOutput.timer.Stop()
		nativeOutput.timer = nil
	}
}

func NativeOutputApplyRange(s State, degrees int) error {
	lease, err := acquireNativeOutputLease(s, "range", false)
	if err != nil {
		recordNativeOutput(WheelDevice{}, "range", false, err)
		return err
	}
	w, _ := SelectedWheel(s)
	report, err := BuildClassicRangeReport(degrees)
	if err == nil {
		err = writeNativeOutputPath(lease.Path, report)
	}
	if !releaseNativeOutputLease(lease) {
		err = errors.Join(err, errors.New("Range-Output-Lease konnte nicht eindeutig freigegeben werden"))
	}
	recordNativeOutput(w, fmt.Sprintf("range=%d°", degrees), false, err)
	return err
}

func NativeOutputSetLEDs(s State, mask byte) error {
	lease, err := acquireNativeOutputLease(s, "leds", false)
	if err != nil {
		recordNativeOutput(WheelDevice{}, "leds", false, err)
		return err
	}
	w, _ := SelectedWheel(s)
	caps, capOK := WheelCapabilitiesForDevice(w)
	if !capOK || !caps.HasRPMLEDs {
		err = errors.New("dieses Wheel meldet keine unterstützte Rev-LED-Ausgabe")
	} else {
		var report []byte
		report, err = BuildG27LEDReport(mask)
		if err == nil {
			err = writeNativeOutputPath(lease.Path, report)
		}
	}
	if !releaseNativeOutputLease(lease) {
		err = errors.Join(err, errors.New("LED-Output-Lease konnte nicht eindeutig freigegeben werden"))
	}
	recordNativeOutput(w, fmt.Sprintf("leds=0x%02X", mask), false, err)
	return err
}

// NativeOutputPulseLEDs is a non-motor physical transport proof for G27.
// Unlike ProbeNativeOutputTransport it writes a harmless, visible report so a
// successful Windows handle open cannot be confused with hardware delivery.
func NativeOutputPulseLEDs(s State, duration time.Duration) error {
	if duration <= 0 || duration > time.Second {
		duration = 220 * time.Millisecond
	}
	lease, err := acquireNativeOutputLease(s, "led-ping", false)
	if err != nil {
		return err
	}
	w, _ := SelectedWheel(s)
	caps, capOK := WheelCapabilitiesForDevice(w)
	if !capOK || !caps.HasRPMLEDs {
		releaseNativeOutputLease(lease)
		return errors.New("dieses Wheel unterstützt keinen sichtbaren RPM-LED-Output-Ping")
	}
	on, err := BuildG27LEDReport(0x1F)
	if err != nil {
		releaseNativeOutputLease(lease)
		return err
	}
	off, err := BuildG27LEDReport(0)
	if err != nil {
		releaseNativeOutputLease(lease)
		return err
	}
	tr, err := openNativeHIDTransport(lease.Path)
	if err != nil {
		releaseNativeOutputLease(lease)
		return err
	}
	writeErr := tr.WriteReport(on)
	if writeErr == nil {
		time.Sleep(duration)
		writeErr = tr.WriteReport(off)
	}
	closeErr := tr.Close()
	if !releaseNativeOutputLease(lease) {
		writeErr = errors.Join(writeErr, errors.New("LED-Ping Output-Lease konnte nicht eindeutig freigegeben werden"))
	}
	writeErr = errors.Join(writeErr, closeErr)
	recordNativeOutput(w, "led-ping", false, writeErr)
	return writeErr
}

func NativeOutputTestAutocenter(s State, percent, ramp int) error {
	if percent == 0 {
		return NativeOutputEmergencyStop(s)
	}
	// Build 009: Autocenter is a replace-current-test action just like Constant,
	// Spring, Damper and Friction. Stop both nativeFFB workers and any previous
	// momentary nativeOutput lease before acquiring the new motor lease.
	if err := NativeOutputEmergencyStop(s); err != nil {
		return fmt.Errorf("vorherige Motor-Ausgabe konnte nicht sicher neutralisiert werden: %w", err)
	}
	lease, err := acquireNativeOutputLease(s, "autocenter-test", true)
	if err != nil {
		return err
	}
	w, _ := SelectedWheel(s)
	report, err := BuildClassicAutocenterReport(percent, ramp)
	if err != nil {
		releaseNativeOutputLease(lease)
		return err
	}
	enableReport := BuildClassicAutocenterEnableReport()
	if err := MarkRuntimeOutputActiveTarget(s.DataDir, w, "autocenter-test"); err != nil {
		releaseNativeOutputLease(lease)
		return fmt.Errorf("Recovery-Marker konnte nicht geschrieben werden; Motor-Ausgabe verweigert: %w", err)
	}
	// Logitech/lg4ff/OpenG27 autocenter is a two-report transaction: first set
	// the spring parameters, then explicitly enable the spring. D6.1 sent only
	// the FE/0D set packet, which could make the Autocenter button appear to do
	// nothing on real G25/G27/DFGT hardware.
	if err := writeNativeOutputPath(lease.Path, report, enableReport); err != nil {
		failErr := abortMotorStartAfterPossibleWrite(lease, err)
		recordNativeOutput(w, "autocenter", failErr != nil && NativeOutputLeaseSnapshot().Active, failErr)
		return failErr
	}
	recordNativeOutput(w, fmt.Sprintf("autocenter=%d%% ramp=%d", percent, ramp), true, nil)
	nativeOutput.Lock()
	stopWatchdogLocked()
	copyLease := lease
	nativeOutput.lease = &copyLease
	nativeOutput.timer = time.AfterFunc(nativeManualFFBTestDuration, func() {
		_ = nativeOutputSafeStopLease(copyLease, true)
	})
	nativeOutput.Unlock()
	return nil
}

func neutralReportsForModel(model string) [][]byte {
	reports := make([][]byte, 0, 7)
	// Canonical lg4ff global stop covers the productive constant-force scheduler
	// in addition to the historical per-slot stop packets below.
	reports = append(reports, wheelengine.WithReportID(wheelengine.StopAllEffects()))
	for slot := 0; slot < 4; slot++ {
		r, _ := BuildClassicEffectStopReport(slot)
		reports = append(reports, r)
	}
	// Once SpringEnable has been sent, a zero SpringSet is not the clearest
	// neutralization primitive. Use the canonical lg4ff/OpenG27 SpringOff packet.
	reports = append(reports, BuildClassicAutocenterOffReport())
	if IsG27Model(model) {
		led, _ := BuildG27LEDReport(0)
		reports = append(reports, led)
	}
	return reports
}

func emergencyOutputTarget(s State) (nativeOutputLeaseToken, error) {
	if lease, ok := currentNativeOutputLease(); ok {
		return lease, nil
	}
	w, ok := SelectedWheel(s)
	if !ok || !w.Supported || !w.ModelConfirmed || !w.PnPVerified || !IsKnownWheelModel(w.Model) {
		return nativeOutputLeaseToken{}, errors.New("kein aktuell PnP-verifiziertes, modellbestätigtes Wheel für Emergency Stop")
	}
	pid, ok := nativeOutputPID(w.Model)
	if !ok {
		return nativeOutputLeaseToken{}, errors.New("kein Emergency-Stop-Protokoll für dieses Modell")
	}
	paths := rawPathsForWheel(w, s.RawInputDevices, pid)
	if len(paths) != 1 {
		return nativeOutputLeaseToken{}, fmt.Errorf("Emergency-Stop-HID-Pfad ist nicht eindeutig (%d Treffer)", len(paths))
	}
	return nativeOutputLeaseToken{WheelID: w.ID, SessionID: w.SessionID, Model: w.Model, Path: paths[0], Purpose: "emergency", Motor: true, DataDir: s.DataDir}, nil
}

func nativeOutputSafeStopLease(lease nativeOutputLeaseToken, watchdog bool) error {
	writeErr := writeNativeOutputPath(lease.Path, neutralReportsForModel(lease.Model)...)
	stopErr := writeErr
	if stopErr == nil {
		if lease.Motor {
			stopErr = completeNativeMotorStop(lease, lease.DataDir, nil)
		} else if lease.Generation != 0 && !releaseNativeOutputLease(lease) {
			stopErr = errors.New("neutraler Output wurde geschrieben, aber der Output-Lease konnte nicht bestätigt freigegeben werden")
		}
	}
	nativeOutput.Lock()
	stopWatchdogLocked()
	nativeOutput.status.Active = stopErr != nil && lease.Motor // fail closed: unproven motor state stays active
	nativeOutput.status.WheelID = lease.WheelID
	nativeOutput.status.Model = lease.Model
	nativeOutput.status.LastCommand = "EMERGENCY STOP"
	nativeOutput.status.LastWrite = time.Now()
	nativeOutput.status.CommandCount++
	if watchdog {
		nativeOutput.status.WatchdogStops++
	} else {
		nativeOutput.status.EmergencyStops++
	}
	if stopErr != nil {
		nativeOutput.status.LastError = stopErr.Error()
	} else {
		nativeOutput.status.LastError = ""
		if nativeOutput.lease != nil && (lease.Generation == 0 || nativeOutput.lease.Generation == lease.Generation) {
			nativeOutput.lease = nil
		}
	}
	nativeOutput.Unlock()
	return stopErr
}

// nativeOutputBestEffortNeutralizeLease may be used after a worker-stop timeout.
// It deliberately never clears the recovery marker or releases the lease: the
// old writer is still unproven and could resume after this write.
func abortMotorStartAfterPossibleWrite(lease nativeOutputLeaseToken, cause error) error {
	stopErr := nativeOutputSafeStopLease(lease, false)
	if stopErr != nil {
		return errors.Join(cause, fmt.Errorf("Startfehler konnte nicht mit bestätigter Neutralisierung abgeschlossen werden; Lease/Recovery bleiben fail-closed: %w", stopErr))
	}
	return cause
}

func nativeOutputBestEffortNeutralizeLease(lease nativeOutputLeaseToken) error {
	err := writeNativeOutputPath(lease.Path, neutralReportsForModel(lease.Model)...)
	nativeOutput.Lock()
	nativeOutput.status.Active = true // old writer may still be alive
	nativeOutput.status.WheelID = lease.WheelID
	nativeOutput.status.Model = lease.Model
	nativeOutput.status.LastCommand = "EMERGENCY STOP (unconfirmed worker)"
	nativeOutput.status.LastWrite = time.Now()
	nativeOutput.status.CommandCount++
	nativeOutput.status.EmergencyStops++
	if err != nil {
		nativeOutput.status.LastError = err.Error()
	} else {
		nativeOutput.status.LastError = "Neutralreport gesendet, aber alter Output-Worker ist nicht bestätigt beendet; Lease bleibt gesperrt"
	}
	nativeOutput.Unlock()
	return err
}

func NativeOutputEmergencyStop(s State) error {
	leaseBefore, hasLease := currentNativeOutputLease()
	ffbBefore := NativeFFBSnapshot()
	outBefore := NativeOutputSnapshot()
	needMarker := false
	if strings.TrimSpace(s.DataDir) != "" {
		needMarker, _ = RuntimeOutputRecoveryNeeded(s.DataDir)
	}
	if !hasLease && !ffbBefore.Active && !ffbBefore.Stopping && !outBefore.Active && !needMarker {
		return nil
	}

	// First ask the active worker to stop itself. A timeout is safety-significant:
	// the lease and recovery marker stay owned so no replacement writer can start.
	workerErr := stopNativeFFBSession(true)
	if workerErr != nil {
		if !hasLease {
			if cur, ok := currentNativeOutputLease(); ok {
				leaseBefore, hasLease = cur, true
			}
		}
		if hasLease {
			neutralErr := nativeOutputBestEffortNeutralizeLease(leaseBefore)
			if neutralErr != nil {
				return errors.Join(workerErr, fmt.Errorf("zusätzlicher Neutralreport fehlgeschlagen: %w", neutralErr))
			}
		}
		return fmt.Errorf("Output-Worker ist nicht bestätigt beendet; Lease/Recovery bleiben gesperrt: %w", workerErr)
	}

	target, targetErr := emergencyOutputTarget(s)
	if targetErr != nil {
		return targetErr
	}
	return nativeOutputSafeStopLease(target, false)
}

// NativeOutputRelease is fail-closed. It never drops bookkeeping behind a
// motor state that was not conclusively neutralized.
func NativeOutputRelease() error {
	workerErr := stopNativeFFBSession(false)
	if workerErr != nil {
		if lease, ok := currentNativeOutputLease(); ok {
			_ = nativeOutputBestEffortNeutralizeLease(lease)
		}
		return workerErr
	}
	if lease, ok := currentNativeOutputLease(); ok {
		if err := nativeOutputSafeStopLease(lease, false); err != nil {
			return err
		}
	}
	nativeOutput.Lock()
	stopWatchdogLocked()
	nativeOutput.status.Active = false
	nativeOutput.Unlock()
	return nil
}

func NativeOutputSnapshot() NativeOutputStatus {
	nativeOutput.Lock()
	defer nativeOutput.Unlock()
	return nativeOutput.status
}
