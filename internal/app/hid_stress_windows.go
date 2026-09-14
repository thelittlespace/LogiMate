//go:build windows

package app

import (
	"fmt"
	"strings"

	"github.com/thelittlespace/LogiMate/internal/system"
)

func hidStressLiveSummary(rep system.HIDStressLiveReport, path string) string {
	result := "FAIL"
	if rep.Passed() {
		result = "PASS"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Result: %s\r\n", result)
	fmt.Fprintf(&b, "Cycles: %d/%d\r\n", rep.CyclesCompleted, rep.CyclesRequested)
	fmt.Fprintf(&b, "Valid input samples: %d/%d\r\n", rep.ValidInputSamples, rep.InputSamples)
	fmt.Fprintf(&b, "Write failures: %d\r\n", rep.WriteFailures)
	fmt.Fprintf(&b, "Max write latency: %d ms\r\n", rep.MaxWriteMillis)
	fmt.Fprintf(&b, "Disconnect expected / observed: %v / %v\r\n", rep.DisconnectExpected, rep.DisconnectObserved)
	fmt.Fprintf(&b, "Output lease released: %v\r\n", rep.LeaseReleased)
	if strings.TrimSpace(rep.LastError) != "" {
		fmt.Fprintf(&b, "Last error: %s\r\n", rep.LastError)
	}
	if strings.TrimSpace(path) != "" {
		fmt.Fprintf(&b, "\r\nEvidence: %s", path)
	}
	b.WriteString("\r\n\r\nThis is HID-stress evidence only. Stable remains blocked until the reviewed physical HID-stress gate is explicitly promoted in the repository certification manifest.")
	return b.String()
}

func runHIDStressSoftwareDialog() {
	rep, err := system.HIDStressSoftwareSelfTest(10000)
	if err != nil {
		messageBox(mainWnd, "Software-Stress: FAIL\r\n\r\n"+err.Error(), "HID Stress · Software", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, fmt.Sprintf("Software-Stress: PASS\r\n\r\nIterationen: %d\r\nSafe Reports: %d\r\nFallback-Policy Checks: %d\r\nDeadline Checks: %d\r\nDauer: %s\r\n\r\n%s\r\n\r\nWichtig: Dieser Test ersetzt keine echte USB/HID-Hardwarevalidierung.", rep.Iterations, rep.SafeReportsBuilt, rep.UnsupportedChecks, rep.DeadlineChecks, rep.Duration, rep.Detail), "HID Stress · Software", MB_OK|MB_ICONINFORMATION)
}

func runHIDStressNormal(s system.State) {
	if !getUISettings().NativeWheelOutput {
		messageBox(mainWnd, "Für den echten HID-Schreibpfad muss Einstellungen → Native Wheel Output (Experimental) aktiviert sein. Der HID-Stresstest startet dabei keinen Motor-Effekt; er schreibt nur den bereits aktiven Lenkwinkel wiederholt idempotent.", "HID Stress · Live", MB_OK|MB_ICONWARNING)
		return
	}
	if messageBox(mainWnd, "Der HID-Stresstest öffnet/schließt den nativen HID-Output 100-mal und schreibt ausschließlich den bereits gewählten Lenkwinkel erneut. Parallel werden Direct-HID-Inputsamples geprüft.\r\n\r\nEs wird KEIN Constant Force, Spring, Damper, Friction oder Autocenter gestartet.\r\n\r\nFortfahren?", "HID Stress · 100 Zyklen", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
		return
	}
	rep, runErr := system.RunHIDStressLiveProbe(s, 100, false)
	path, exportErr := system.ExportHIDStressReport(s, rep)
	if exportErr != nil {
		path = "Export fehlgeschlagen: " + exportErr.Error()
	}
	if runErr != nil {
		messageBox(mainWnd, hidStressLiveSummary(rep, path)+"\r\n\r\nRun error: "+runErr.Error(), "HID Stress · Live", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, hidStressLiveSummary(rep, path), "HID Stress · Live", MB_OK|map[bool]uint32{true: MB_ICONINFORMATION, false: MB_ICONWARNING}[rep.Passed()])
}

func runHIDStressUSBRemoval(s system.State) {
	if !getUISettings().NativeWheelOutput {
		messageBox(mainWnd, "Aktiviere zuerst Einstellungen → Native Wheel Output (Experimental). Der adverse-I/O-Test nutzt ausschließlich einen nicht-motorischen Range-Report.", "HID Stress · USB Yank", MB_OK|MB_ICONWARNING)
		return
	}
	if messageBox(mainWnd, "ADVERSE-I/O TEST\r\n\r\nNach JA läuft ungefähr 10 Sekunden lang ein nicht-motorischer HID-Reopen/Write-Zyklus. Ziehe das USB-Kabel des Lenkrads während dieses Fensters ab.\r\n\r\nErwartet wird: ein I/O-Fehler, keine Wiederholung eines unbestätigten Writes, Freigabe des zentralen Output-Lease und später ein sauberer Reconnect.\r\n\r\nKein FFB-Effekt wird gestartet. Fortfahren?", "HID Stress · USB-Abziehen", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
		return
	}
	rep, runErr := system.RunHIDStressLiveProbe(s, 250, true)
	path, exportErr := system.ExportHIDStressReport(s, rep)
	if exportErr != nil {
		path = "Export fehlgeschlagen: " + exportErr.Error()
	}
	if rep.DisconnectObserved {
		messageBox(mainWnd, "Der HID-Abbruch wurde erkannt und der Stress-Lease ist beendet. Stecke das Lenkrad jetzt wieder ein und warte auf die automatische Neuerkennung. Danach den normalen 100-Zyklen-Test erneut ausführen, um den Reconnect-Pfad zu bestätigen.", "HID Stress · Reconnect", MB_OK|MB_ICONINFORMATION)
		refreshFastAsync()
	}
	if runErr != nil && !rep.Passed() {
		messageBox(mainWnd, hidStressLiveSummary(rep, path)+"\r\n\r\nRun error: "+runErr.Error(), "HID Stress · USB-Abziehen", MB_OK|MB_ICONWARNING)
		return
	}
	messageBox(mainWnd, hidStressLiveSummary(rep, path), "HID Stress · USB-Abziehen", MB_OK|map[bool]uint32{true: MB_ICONINFORMATION, false: MB_ICONWARNING}[rep.Passed()])
}

func showHIDStressAssistant(s system.State) {
	for {
		choices := []string{
			"Software Fault-Injection / Policy Stress\n10.000 sichere Report-/Fallback-/Deadline-Prüfungen ohne Hardware-Ausgabe",
			"Live HID Reopen/Write Stress · 100 Zyklen\nNicht-motorischer Range-Report + parallele Input-Samples auf dem echten Wheel",
			"Adverse I/O · USB während Test abziehen\nErwarteten Disconnect, fail-closed Abbruch und Lease-Freigabe als Evidenz erfassen",
			"Transport-Zähler anzeigen\nOpens, Writes, Timeouts, Partial Writes, CancelIoEx, Poison und Compatibility-Fallbacks",
			"Transport-Zähler zurücksetzen\nNur HID-Transport-Diagnosezähler auf null setzen",
		}
		idx := chooseCommand("LogiMate · HID-Stresstest", "HID-Stress / adverse I/O", "Software-Stress darf automatisch PASS sein. Der Stable-Gate 'hidStressValidated' bleibt jedoch false, bis die echte Hardware-Evidenz geprüft und bewusst freigegeben wurde.", choices, 6950)
		switch idx {
		case 0:
			runHIDStressSoftwareDialog()
		case 1:
			runHIDStressNormal(s)
		case 2:
			runHIDStressUSBRemoval(s)
		case 3:
			messageBox(mainWnd, system.NativeHIDTransportSummary(), "HID Transport Metrics", MB_OK|MB_ICONINFORMATION)
		case 4:
			if messageBox(mainWnd, "HID-Transport-Diagnosezähler wirklich zurücksetzen? Hardware-/Zertifizierungsevidenz wird nicht gelöscht.", "HID Stress · Zähler", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) == IDYES {
				system.ResetNativeHIDTransportMetrics()
			}
		default:
			return
		}
	}
}
