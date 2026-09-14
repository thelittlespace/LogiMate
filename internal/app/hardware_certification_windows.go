//go:build windows

package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/thelittlespace/LogiMate/internal/system"
)

func certificationStatusPrefix(item system.CertificationItem) string {
	if item.Passed {
		return "✓"
	}
	if item.Failed {
		return "✕"
	}
	return "○"
}

func certificationProgressText(p system.CertificationProgress) string {
	if p.Required == 0 {
		return "Noch kein zertifizierbares, eindeutig ausgewähltes Wheel."
	}
	return fmt.Sprintf("%s · %d/%d PASS · %d offen · %d fehlgeschlagen", p.Model, p.Passed, p.Required, p.Pending, p.Failed)
}

func showHardwareCertificationAssistant(s system.State) {
	if !system.HasReadableSelectedWheel(s) {
		messageBox(mainWnd, "Bitte zuerst ein eindeutig erkanntes Lenkrad auswählen.", "Hardware-Zertifizierung", MB_OK|MB_ICONWARNING)
		return
	}
	for {
		p := system.HardwareCertificationProgress(s)
		if p.Required == 0 {
			messageBox(mainWnd, "Das ausgewählte Gerät ist noch nicht vollständig PnP-verifiziert/modellbestätigt oder gehört nicht zu G25/G27/DFGT.", "Hardware-Zertifizierung", MB_OK|MB_ICONWARNING)
			return
		}
		choices := make([]string, 0, len(p.Items)+4)
		ids := make([]string, 0, len(p.Items)+4)
		choices = append(choices, "Automatische Basisprüfung\nNative PID, PnP- und Modellstatus aus dem aktuellen Windows-Snapshot prüfen")
		ids = append(ids, "__auto")
		choices = append(choices, "Geführte FFB-Testserie\nConstant, Spring, Damper, Friction und Autocenter nacheinander mit 15 % prüfen")
		ids = append(ids, "__ffb_suite")
		for _, it := range p.Items {
			detail := "Noch nicht geprüft"
			if it.Passed {
				detail = "PASS · " + it.Evidence
			}
			if it.Failed {
				detail = "FAIL · " + it.Evidence
			}
			if len(detail) > 118 {
				detail = detail[:118] + "…"
			}
			choices = append(choices, fmt.Sprintf("%s %s\n%s", certificationStatusPrefix(it), it.Label, detail))
			ids = append(ids, it.Check)
		}
		choices = append(choices, "Bericht exportieren\nAktuelle Evidenz als Markdown + JSON unter LogiMateData\\Certification speichern")
		ids = append(ids, "__export")
		choices = append(choices, "Zertifizierung für dieses Wheel zurücksetzen\nNur lokale Testevidenz löschen; Kalibrierungen/Profile bleiben erhalten")
		ids = append(ids, "__reset")

		idx := chooseCommand("LogiMate · Hardware Certification Assistant", "Physische Wheel-Zertifizierung", certificationProgressText(p)+"\n\nPASS wird nur gespeichert, wenn ein realer Test oder eine eindeutig automatisch beweisbare Hardwareeigenschaft vorliegt.", choices, 6800)
		if idx < 0 || idx >= len(ids) {
			return
		}
		id := ids[idx]
		switch id {
		case "__auto":
			check, pass, evidence := system.AutoCertificationEvidence(s)
			if check == "" {
				messageBox(mainWnd, evidence, "Automatische Basisprüfung", MB_OK|MB_ICONWARNING)
				continue
			}
			if err := system.RecordHardwareCertificationResult(s, check, pass, evidence); err != nil {
				messageBox(mainWnd, err.Error(), "Hardware-Zertifizierung", MB_OK|MB_ICONERROR)
			} else {
				icon := uint32(MB_ICONINFORMATION)
				if !pass {
					icon = MB_ICONWARNING
				}
				messageBox(mainWnd, fmt.Sprintf("%s: %s\r\n\r\n%s", system.CertificationCheckLabel(check), map[bool]string{true: "PASS", false: "FAIL"}[pass], evidence), "Automatische Basisprüfung", MB_OK|icon)
			}
		case "__ffb_suite":
			for _, effect := range []string{"constant-force", "spring", "damper", "friction", "autocenter"} {
				runHardwareCertificationCheck(s, effect)
			}
		case "__export":
			path, err := system.ExportHardwareCertificationReport(s)
			if err != nil {
				messageBox(mainWnd, err.Error(), "Zertifizierungsbericht", MB_OK|MB_ICONERROR)
			} else {
				messageBox(mainWnd, "Bericht gespeichert:\r\n\r\n"+path+"\r\n\r\nZusätzlich wurde eine JSON-Evidenzdatei erzeugt.", "Zertifizierungsbericht", MB_OK|MB_ICONINFORMATION)
			}
		case "__reset":
			if messageBox(mainWnd, "Lokale Hardware-Zertifizierung für dieses Wheel wirklich zurücksetzen?", "Zertifizierung zurücksetzen", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) == IDYES {
				if err := system.ResetHardwareCertification(s); err != nil {
					messageBox(mainWnd, err.Error(), "Hardware-Zertifizierung", MB_OK|MB_ICONERROR)
				}
			}
		default:
			runHardwareCertificationCheck(s, id)
		}
		refreshFastAsync()
	}
}

func recordCertificationFromUser(s system.State, check, prompt, evidence string) {
	result := messageBox(mainWnd, prompt+"\r\n\r\nWar der Test erfolgreich?", "Hardware-Zertifizierung · "+system.CertificationCheckLabel(check), MB_YESNO|MB_ICONINFORMATION|MB_DEFBUTTON2)
	passed := result == IDYES
	if !passed {
		evidence = "Benutzer meldet Test als fehlgeschlagen. " + evidence
	}
	if err := system.RecordHardwareCertificationResult(s, check, passed, evidence); err != nil {
		messageBox(mainWnd, err.Error(), "Hardware-Zertifizierung", MB_OK|MB_ICONERROR)
		return
	}
	setActionFeedback(fmt.Sprintf("Zertifizierung %s: %s", system.CertificationCheckLabel(check), map[bool]string{true: "PASS", false: "FAIL"}[passed]))
}

func runHardwareCertificationCheck(s system.State, check string) {
	switch check {
	case "native-mode-switch":
		c, pass, evidence := system.AutoCertificationEvidence(s)
		if c == "" {
			messageBox(mainWnd, evidence, "Native Mode", MB_OK|MB_ICONWARNING)
			return
		}
		if err := system.RecordHardwareCertificationResult(s, c, pass, evidence); err != nil {
			messageBox(mainWnd, err.Error(), "Hardware-Zertifizierung", MB_OK|MB_ICONERROR)
			return
		}
		messageBox(mainWnd, fmt.Sprintf("%s\r\n\r\n%s", map[bool]string{true: "PASS", false: "FAIL"}[pass], evidence), "Native Mode / PID", MB_OK|map[bool]uint32{true: MB_ICONINFORMATION, false: MB_ICONWARNING}[pass])
	case "steering":
		certifySteering(s)
	case "pedals":
		certifyPedals(s)
	case "buttons":
		certifyButtons(s)
	case "shifter":
		certifyShifter(s)
	case "range":
		certifyRange(s)
	case "constant-force", "spring", "damper", "friction", "autocenter":
		certifyMotorEffect(s, check)
	case "game-ffb":
		recordCertificationFromUser(s, check,
			"Starte ein unterstütztes Spiel/den Telemetrie-Adapter und prüfe in der Erweiterten Ansicht, dass Game-Frames eintreffen, PlayerControl gültig ist und Force nur während der aktiven Session anliegt.",
			"Physischer Game-FFB-/Telemetry-Test durch Benutzer bestätigt; Details siehe exportierte Engine-/Adapterdiagnose.")
	case "reconnect":
		recordCertificationFromUser(s, check,
			"Ziehe das Wheel im neutralen Zustand ab, warte bis LogiMate den Disconnect zeigt, stecke es wieder ein und prüfe, dass dasselbe Wheel/Session-Ziel neu gebunden wird und Live-HID-Daten wieder laufen.",
			"Disconnect/Reconnect auf realer Hardware durchgeführt; Zielbindung und Live-Input nach Reconnect geprüft.")
	case "usb-removal":
		if messageBox(mainWnd, "Dieser Test darf nur mit neutralem bzw. sehr niedrigem kurzzeitigem Output durchgeführt werden. Nach dem Abziehen muss LogiMate fail-closed bleiben und beim Wiederanschließen keinen alten Force-Wert fortsetzen.\r\n\r\nTestanleitung anzeigen/fortfahren?", "USB-Removal Safety", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
			return
		}
		recordCertificationFromUser(s, check,
			"Führe den USB-Abzug während eines begrenzten Testoutputs durch. Nach Reconnect darf kein alter Force-Wert wieder anlaufen; Recovery/Lease müssen sicher sein.",
			"USB-removal fail-closed behavior physically checked by user.")
	case "process-kill":
		recordCertificationFromUser(s, check,
			"Für diesen Test LogiMate während eines begrenzten Testoutputs hart beenden. Danach neu starten und prüfen, ob der Recovery-Marker erkannt wird und kein latched Force verbleibt. Markiere PASS erst nach dem Neustart.",
			"Process-kill/restart recovery physically checked; no latched force observed.")
	case "suspend-resume":
		recordCertificationFromUser(s, check,
			"Windows in Standby schicken und wieder aufwecken. Danach muss das Wheel neu erkannt werden, alter Output darf nicht fortgesetzt werden und Live-HID muss wieder binden.",
			"Suspend/resume physically checked; rebind successful and no stale output observed.")
	case "modern-legacy-rollback":
		recordCertificationFromUser(s, check,
			"Modern → Legacy → Modern vollständig durchführen. Prüfe Backup, Neustartgrenzen, Wheel-Erkennung und abschließenden Native-HID-Betrieb. Bei einem Fehler muss Rollback/Journal den Zustand korrekt wiederherstellen.",
			"Modern/Legacy/Modern migration cycle physically checked on this wheel.")
	}
}

func certifySteering(s system.State) {
	if messageBox(mainWnd, "Lenkrad in die MITTE stellen und OK wählen.", "Zertifizierung · Lenkung 1/3", MB_OK|MB_ICONINFORMATION) != IDOK {
		return
	}
	center := system.ReadPreferredWheelInput(s)
	if !center.Found || !center.SampleValid {
		messageBox(mainWnd, firstNonEmptyApp(center.Error, center.LastInputError, "Kein gültiger HID-Sample"), "Lenkung", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, "Lenkrad ganz nach LINKS stellen und OK wählen.", "Zertifizierung · Lenkung 2/3", MB_OK|MB_ICONINFORMATION)
	left := system.ReadPreferredWheelInput(s)
	messageBox(mainWnd, "Lenkrad ganz nach RECHTS stellen und OK wählen.", "Zertifizierung · Lenkung 3/3", MB_OK|MB_ICONINFORMATION)
	right := system.ReadPreferredWheelInput(s)
	if !left.SampleValid || !right.SampleValid || center.XMax <= center.XMin {
		messageBox(mainWnd, "Ungültige Samples oder Achsenbereich.", "Lenkung", MB_OK|MB_ICONERROR)
		return
	}
	span := int64(right.X) - int64(left.X)
	if span < 0 {
		span = -span
	}
	minSpan := int64(center.XMax-center.XMin) / 2
	pass := span >= minSpan && center.X != left.X && center.X != right.X
	evidence := fmt.Sprintf("Direct input steering samples: left=%d center=%d right=%d range=%d..%d source=%s layout=%s", left.X, center.X, right.X, center.XMin, center.XMax, center.InputSource, center.LayoutID)
	if err := system.RecordHardwareCertificationResult(s, "steering", pass, evidence); err != nil {
		messageBox(mainWnd, err.Error(), "Lenkung", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, map[bool]string{true: "Lenkung eindeutig über den Bereich erkannt.", false: "Lenkungsbereich war nicht eindeutig genug."}[pass]+"\r\n\r\n"+evidence, "Zertifizierung · Lenkung", MB_OK|map[bool]uint32{true: MB_ICONINFORMATION, false: MB_ICONWARNING}[pass])
}

func certifyPedals(s system.State) {
	messageBox(mainWnd, "Alle Pedale vollständig LOSLASSEN und OK wählen.", "Zertifizierung · Pedale 1/4", MB_OK|MB_ICONINFORMATION)
	rest := system.ReadPreferredWheelInput(s)
	if !rest.SampleValid {
		messageBox(mainWnd, "Kein gültiger Input-Sample.", "Pedale", MB_OK|MB_ICONERROR)
		return
	}
	names := []string{"Gas", "Bremse"}
	axes := []func(system.JoyState) uint32{func(j system.JoyState) uint32 { return j.Y }, func(j system.JoyState) uint32 { return j.Z }}
	restVals := []uint32{rest.Y, rest.Z}
	if !system.IsDFGTModel(s.WheelModel) {
		names = append(names, "Kupplung")
		axes = append(axes, func(j system.JoyState) uint32 { return j.R })
		restVals = append(restVals, rest.R)
	}
	diffs := make([]uint64, 0, len(names))
	for i, name := range names {
		messageBox(mainWnd, name+" vollständig DURCHTRETEN, die anderen Pedale loslassen und OK wählen.", fmt.Sprintf("Zertifizierung · Pedale %d/%d", i+2, len(names)+1), MB_OK|MB_ICONINFORMATION)
		j := system.ReadPreferredWheelInput(s)
		if !j.SampleValid {
			messageBox(mainWnd, "Kein gültiger Input-Sample bei "+name+".", "Pedale", MB_OK|MB_ICONERROR)
			return
		}
		v := axes[i](j)
		a, b := uint64(v), uint64(restVals[i])
		if a > b {
			diffs = append(diffs, a-b)
		} else {
			diffs = append(diffs, b-a)
		}
	}
	pass := true
	for _, d := range diffs {
		if d < 16 {
			pass = false
		}
	}
	evidence := fmt.Sprintf("Pedal raw deltas %s=%d", names[0], diffs[0])
	for i := 1; i < len(names); i++ {
		evidence += fmt.Sprintf(", %s=%d", names[i], diffs[i])
	}
	evidence += fmt.Sprintf("; source=%s layout=%s", rest.InputSource, rest.LayoutID)
	if err := system.RecordHardwareCertificationResult(s, "pedals", pass, evidence); err != nil {
		messageBox(mainWnd, err.Error(), "Pedale", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, map[bool]string{true: "Pedalbewegungen wurden eindeutig erkannt.", false: "Mindestens eine Pedalachse änderte sich nicht ausreichend."}[pass]+"\r\n\r\n"+evidence, "Zertifizierung · Pedale", MB_OK|map[bool]uint32{true: MB_ICONINFORMATION, false: MB_ICONWARNING}[pass])
}

func certifyButtons(s system.State) {
	messageBox(mainWnd, "Alle Tasten und Wippen LOSLASSEN und OK wählen.", "Zertifizierung · Tasten 1/2", MB_OK|MB_ICONINFORMATION)
	before := system.ReadPreferredWheelInput(s)
	if !before.SampleValid {
		messageBox(mainWnd, "Kein gültiger Input-Sample.", "Tasten", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, "Jetzt EINE beliebige Lenkradtaste oder Wippe GEDRÜCKT halten und mit der Maus OK wählen.", "Zertifizierung · Tasten 2/2", MB_OK|MB_ICONINFORMATION)
	time.Sleep(30 * time.Millisecond)
	after := system.ReadPreferredWheelInput(s)
	changed := before.Buttons != after.Buttons || before.PaddleLeft != after.PaddleLeft || before.PaddleRight != after.PaddleRight
	evidence := fmt.Sprintf("buttons before=0x%08X after=0x%08X paddles L=%v/%v R=%v/%v source=%s", before.Buttons, after.Buttons, before.PaddleLeft, after.PaddleLeft, before.PaddleRight, after.PaddleRight, after.InputSource)
	if err := system.RecordHardwareCertificationResult(s, "buttons", changed, evidence); err != nil {
		messageBox(mainWnd, err.Error(), "Tasten", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, map[bool]string{true: "Tastenänderung erkannt.", false: "Keine eindeutige Tastenänderung erkannt."}[changed]+"\r\n\r\n"+evidence, "Zertifizierung · Tasten", MB_OK|map[bool]uint32{true: MB_ICONINFORMATION, false: MB_ICONWARNING}[changed])
}

func certifyShifter(s system.State) {
	if system.IsDFGTModel(s.WheelModel) {
		messageBox(mainWnd, "DFGT besitzt keinen H-Shifter; dieser Check gehört nicht zur DFGT-Matrix.", "H-Shifter", MB_OK|MB_ICONINFORMATION)
		return
	}
	steps := []int{0, 1, 2, 3, 4, 5, 6, -1}
	labels := []string{"Neutral", "Gang 1", "Gang 2", "Gang 3", "Gang 4", "Gang 5", "Gang 6", "Rückwärts"}
	seen := map[int]bool{}
	var seq []string
	for i, g := range steps {
		messageBox(mainWnd, labels[i]+" einlegen und OK wählen.", fmt.Sprintf("Zertifizierung · H-Shifter %d/%d", i+1, len(steps)), MB_OK|MB_ICONINFORMATION)
		j := system.ReadPreferredWheelInput(s)
		seq = append(seq, fmt.Sprintf("%s=%d", labels[i], j.Gear))
		if j.SampleValid && j.Gear == g {
			seen[g] = true
		}
	}
	pass := len(seen) == len(steps)
	evidence := "Gear sequence: " + strings.Join(seq, ", ")
	if err := system.RecordHardwareCertificationResult(s, "shifter", pass, evidence); err != nil {
		messageBox(mainWnd, err.Error(), "H-Shifter", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, map[bool]string{true: "Alle acht Shifter-Zustände eindeutig erkannt.", false: "Mindestens ein Shifter-Zustand wurde nicht korrekt erkannt."}[pass]+"\r\n\r\n"+evidence, "Zertifizierung · H-Shifter", MB_OK|map[bool]uint32{true: MB_ICONINFORMATION, false: MB_ICONWARNING}[pass])
}

func certifyRange(s system.State) {
	if !getUISettings().NativeWheelOutput {
		messageBox(mainWnd, "Aktiviere zuerst Einstellungen → Native Wheel Output (Experimental).", "Range-Zertifizierung", MB_OK|MB_ICONWARNING)
		return
	}
	if messageBox(mainWnd, "LogiMate setzt das Wheel kurz auf 540° und anschließend wieder auf 900°. Es wird kein Motor-FFB gestartet. Fortfahren?", "Range-Zertifizierung", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
		return
	}
	if err := system.NativeOutputApplyRange(s, 540); err != nil {
		if recordErr := system.RecordHardwareCertificationResult(s, "range", false, "540° write failed: "+err.Error()); recordErr != nil {
			recordDiagnosticEvent("error", "CERT-RANGE-WRITE", "Hardware Certification", "Range-Fehler konnte nicht gespeichert werden", recordErr.Error())
		}
		messageBox(mainWnd, err.Error(), "Range", MB_OK|MB_ICONERROR)
		return
	}
	ok := messageBox(mainWnd, "Ist der mechanische/virtuelle Lenkwinkel jetzt erkennbar auf 540° begrenzt?", "Range-Zertifizierung", MB_YESNO|MB_ICONINFORMATION|MB_DEFBUTTON2) == IDYES
	restoreErr := system.NativeOutputApplyRange(s, 900)
	evidence := "540° command accepted; user observed range change=" + fmt.Sprint(ok)
	if restoreErr != nil {
		evidence += "; 900° restore error=" + restoreErr.Error()
		ok = false
	}
	if err := system.RecordHardwareCertificationResult(s, "range", ok, evidence); err != nil {
		recordDiagnosticEvent("error", "CERT-RANGE-SAVE", "Hardware Certification", "Range-Evidenz konnte nicht gespeichert werden", err.Error())
		messageBox(mainWnd, err.Error(), "Range-Zertifizierung", MB_OK|MB_ICONERROR)
	}
}

func certifyMotorEffect(s system.State, check string) {
	if !getUISettings().NativeWheelOutput {
		messageBox(mainWnd, "Aktiviere zuerst Einstellungen → Native Wheel Output (Experimental).", "FFB-Zertifizierung", MB_OK|MB_ICONWARNING)
		return
	}
	if messageBox(mainWnd, "Dieser Test erzeugt kurz Lenkkraft/Widerstand. Hände locker lassen, nichts einklemmen. Jeder Motor-Test besitzt einen Watchdog bzw. wird danach über Emergency Stop neutralisiert. Fortfahren?", "FFB-Zertifizierung", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
		return
	}
	var err error
	switch check {
	case "constant-force":
		err = system.StartNativeConstantForceTest(s, 15)
	case "spring":
		err = system.StartNativeSpringTest(s, 15)
	case "damper":
		err = system.StartNativeDamperTest(s, 15)
	case "friction":
		err = system.StartNativeFrictionTest(s, 15)
	case "autocenter":
		err = system.NativeOutputTestAutocenter(s, 15, 2)
	}
	if err != nil {
		if recordErr := system.RecordHardwareCertificationResult(s, check, false, "hardware command failed: "+err.Error()); recordErr != nil {
			recordDiagnosticEvent("error", "CERT-FFB-WRITE", "Hardware Certification", "FFB-Fehler konnte nicht gespeichert werden", recordErr.Error())
		}
		messageBox(mainWnd, err.Error(), "FFB-Zertifizierung", MB_OK|MB_ICONERROR)
		return
	}
	time.Sleep(300 * time.Millisecond)
	felt := messageBox(mainWnd, "War der erwartete Effekt am echten Lenkrad klar spürbar und blieb das Wheel kontrollierbar?", "FFB-Zertifizierung · "+system.CertificationCheckLabel(check), MB_YESNO|MB_ICONINFORMATION|MB_DEFBUTTON2) == IDYES
	stopErr := system.NativeOutputEmergencyStop(s)
	snap := system.NativeFFBSnapshot()
	evidence := fmt.Sprintf("physical effect observed=%v; frames=%d clips=%d lastError=%s", felt, snap.Frames, snap.ClipEvents, firstNonEmptyApp(snap.LastError, "none"))
	pass := felt && stopErr == nil
	if stopErr != nil {
		evidence += "; emergency stop error=" + stopErr.Error()
	}
	if err := system.RecordHardwareCertificationResult(s, check, pass, evidence); err != nil {
		recordDiagnosticEvent("error", "CERT-FFB-SAVE", "Hardware Certification", "FFB-Evidenz konnte nicht gespeichert werden", err.Error())
		messageBox(mainWnd, err.Error(), "FFB-Zertifizierung", MB_OK|MB_ICONERROR)
	}
}

func firstNonEmptyApp(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return "—"
}
