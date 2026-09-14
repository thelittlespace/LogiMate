//go:build windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thelittlespace/LogiMate/internal/system"
)

var wheelAdvancedView bool

const (
	inputToolButton = 5201 + iota
	inputToolShifter
	inputToolSteering
	inputToolPedals
	inputToolRawToggle
	inputToolRawCopy
	inputToolNativeOutput
	inputToolGameProfiles
	inputToolTelemetry
	inputToolHealth
	inputToolHIDStress
	inputToolReleaseTrust
	inputToolCertification
)

func showInputTools(s system.State) {
	if !system.HasReadableSelectedWheel(s) {
		messageBox(mainWnd, "Kein eindeutig ausgewähltes Lenkrad für Eingabelernen vorhanden.", "Kalibrierung", MB_OK|MB_ICONWARNING)
		return
	}
	labels := []string{
		"Taste lernen\nEine physische Taste einem LogiMate-Namen zuordnen",
		"H-Shifter lernen\nNeutral, 1–6 und R erfassen und Schalter prüfen",
		"Lenkwinkel kalibrieren\nLinks, Mitte, Rechts und Arbeitsbereich erfassen",
		"Pedale kalibrieren\nAchse, Minimum/Maximum, Invertierung, Deadzone und Kurve erfassen",
		map[bool]string{true: "Erweiterte Ansicht schließen\nZur kompakten Lenkradansicht zurückkehren", false: "Erweiterte Ansicht öffnen\nAlle Live-, HID-, Rohdaten- und Engine-Werte direkt auf der Lenkradseite anzeigen"}[wheelAdvancedView],
		"Rohreport kopieren\nAktuellen Direct-HID-Report als Hex/Byte-Tabelle kopieren",
	}
	ids := []int32{inputToolButton, inputToolShifter, inputToolSteering, inputToolPedals, inputToolRawToggle, inputToolRawCopy}
	if getUISettings().NativeWheelOutput {
		labels = append(labels, "Native Output (Experimental)\nRotation, G27-LEDs und kurzzeitiges Autocenter sicher testen")
		ids = append(ids, inputToolNativeOutput)
	}
	labels = append(labels,
		"Game-Profile\nSpiel erkennen, Wheel-Engine-Profil sicher zuordnen und exportieren",
		"Telemetry Hub\nLokale Telemetrie starten, Status und LED-/Force-Vorschau prüfen",
		"Engine Health\nSelbsttests, Zustandsinvarianten, Recovery und Release-Readiness prüfen",
		"HID Stress / adverse I/O\nReopen/Write-Zyklen, USB-Yank und fail-closed Transportdiagnose prüfen",
		"Release Trust / Authenticode\nSignaturstatus des laufenden Builds und Stable-Signing-Gate prüfen",
		"Hardware Certification Assistant\nPhysische G25/G27/DFGT-Tests Schritt für Schritt durchführen und Evidenz speichern",
	)
	ids = append(ids, inputToolGameProfiles, inputToolTelemetry, inputToolHealth, inputToolHIDStress, inputToolReleaseTrust, inputToolCertification)
	idx := chooseCommand("LogiMate · Eingaben", "Kalibrieren, lernen und Rohdaten prüfen", "Alle Lernwerte werden pro physischem Lenkrad gespeichert. Lesen und Kalibrieren verändert keine Treiber und sendet kein Force Feedback.", labels, 5201)
	if idx < 0 || idx >= len(ids) {
		return
	}
	switch ids[idx] {
	case inputToolButton:
		learnButton(s)
	case inputToolShifter:
		learnHShifter(s)
	case inputToolSteering:
		learnSteering(s)
	case inputToolPedals:
		learnPedals(s)
	case inputToolRawToggle:
		wheelAdvancedView = !wheelAdvancedView
		contentScroll = 0
		setActionFeedback(map[bool]string{true: "Erweiterte Lenkradansicht aktiv.", false: "Standardansicht aktiv."}[wheelAdvancedView])
		invalidate(mainWnd)
	case inputToolRawCopy:
		copyRawReport(s)
	case inputToolNativeOutput:
		showNativeOutputTools(s)
	case inputToolGameProfiles:
		showGameProfileTools(s)
	case inputToolTelemetry:
		showTelemetryTools(s)
	case inputToolHealth:
		showEngineHealth(s)
	case inputToolHIDStress:
		showHIDStressAssistant(s)
	case inputToolReleaseTrust:
		showReleaseTrust()
	case inputToolCertification:
		showHardwareCertificationAssistant(s)
	}
}

func chooseCommand(title, instruction, content string, choices []string, base int32) int {
	return modernChoiceDialog(mainWnd, title, instruction, content, choices, base)
}

func learningSampleOK(j system.JoyState) (bool, string) {
	if !j.Found || !j.SampleValid {
		msg := strings.TrimSpace(j.Error)
		if msg == "" {
			msg = "Noch kein gültiger Eingabereport für die aktuelle Gerätesitzung vorhanden."
		}
		return false, msg
	}
	state := strings.ToLower(strings.TrimSpace(j.ConnectionState))
	if strings.Contains(state, "malformed") || strings.Contains(state, "read error") || strings.Contains(state, "reconnect") {
		msg := "Der aktuelle Eingabepfad ist nicht gesund (" + j.ConnectionState + "). Kalibrierwerte werden erst nach einem wieder gültigen Sample gespeichert."
		if strings.TrimSpace(j.LastInputError) != "" {
			msg += "\r\n\r\nLetzter Fehler: " + j.LastInputError
		}
		return false, msg
	}
	return true, ""
}

func learnButton(s system.State) {
	targets := []string{"Paddle L", "Paddle R", "Wheel 1", "Wheel 2", "Wheel 3", "Wheel 4", "Wheel 5", "Wheel 6", "Shifter 1", "Shifter 2", "Shifter 3", "Shifter 4", "Shifter 5", "Shifter 6", "Shifter 7", "Shifter 8"}
	idx := chooseCommand("LogiMate · Taste lernen", "Welche Funktion möchtest du lernen?", "Wähle zuerst den LogiMate-Namen. Danach hältst du die gewünschte physische Taste gedrückt.", targets, 5300)
	if idx < 0 {
		return
	}
	messageBox(mainWnd, "Lass jetzt alle Lenkrad- und Shiftertasten los und klicke auf OK. LogiMate speichert diesen Zustand als Ausgangslage.", "Taste lernen · Ausgangslage", MB_OK|MB_ICONINFORMATION)
	before := system.ReadPreferredWheelInput(s)
	if ok, why := learningSampleOK(before); !ok {
		messageBox(mainWnd, why, "Taste lernen", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, "Halte jetzt die gewünschte physische Taste gedrückt und klicke mit der Maus auf OK, während du sie weiter gedrückt hältst.", "Taste lernen · "+targets[idx], MB_OK|MB_ICONINFORMATION)
	time.Sleep(35 * time.Millisecond)
	after := system.ReadPreferredWheelInput(s)
	if ok, why := learningSampleOK(after); !ok {
		messageBox(mainWnd, why, "Taste lernen", MB_OK|MB_ICONERROR)
		return
	}
	bit, ok := system.FirstPressedButton(before, after)
	if !ok {
		messageBox(mainWnd, "Es wurde nicht genau eine neu gedrückte Taste erkannt. Bitte nur eine Taste gedrückt halten und erneut versuchen.", "Taste lernen", MB_OK|MB_ICONWARNING)
		return
	}
	if err := system.SaveButtonMapping(s.DataDir, s.SelectedWheelID, targets[idx], bit); err != nil {
		messageBox(mainWnd, err.Error(), "Taste lernen", MB_OK|MB_ICONERROR)
		return
	}
	setActionFeedback(fmt.Sprintf("%s wurde auf Button %d gelernt.", targets[idx], bit+1))
	refreshFastAsync()
}

func learnHShifter(s system.State) {
	if !system.HasActionableSelectedWheel(s) {
		messageBox(mainWnd, "Bitte zuerst das exakte Lenkradmodell bestätigen.", "H-Shifter lernen", MB_OK|MB_ICONWARNING)
		return
	}
	if !system.IsG25Model(s.WheelModel) && !system.IsG27Model(s.WheelModel) {
		messageBox(mainWnd, "Der H-Shifter-Lernmodus ist für G25 und G27 vorgesehen. Das Driving Force GT besitzt keinen 6-Gang-H-Shifter.", "H-Shifter lernen", MB_OK|MB_ICONINFORMATION)
		return
	}
	steps := []struct {
		label string
		gear  int
	}{{"Neutral", 0}, {"Gang 1", 1}, {"Gang 2", 2}, {"Gang 3", 3}, {"Gang 4", 4}, {"Gang 5", 5}, {"Gang 6", 6}, {"Rückwärts", -1}}
	signatures := map[string]int{}
	for _, step := range steps {
		messageBox(mainWnd, "Lege jetzt „"+step.label+"“ ein und klicke anschließend auf OK. Lenkrad und Pedale dürfen dabei bewegt werden; LogiMate speichert nur die Shifter-Signatur.", "H-Shifter lernen · "+step.label, MB_OK|MB_ICONINFORMATION)
		j := system.ReadPreferredWheelInput(s)
		if ok, why := learningSampleOK(j); !ok {
			messageBox(mainWnd, why, "H-Shifter lernen", MB_OK|MB_ICONERROR)
			return
		}
		sig := system.GearSignature(j)
		if sig == "" {
			messageBox(mainWnd, "Für diesen Eingabepfad konnte keine stabile Shifter-Signatur gebildet werden.", "H-Shifter lernen", MB_OK|MB_ICONWARNING)
			return
		}
		if old, exists := signatures[sig]; exists {
			messageBox(mainWnd, fmt.Sprintf("Die Hardware liefert für %s dieselbe Schalter-Signatur wie für %s. Das deutet auf eine fehlerhafte/überlappende Shifter-Erkennung hin.\r\n\r\nSignatur: %s", step.label, gearName(old), sig), "H-Shifter prüfen", MB_OK|MB_ICONWARNING)
			return
		}
		signatures[sig] = step.gear
	}
	if err := system.SaveGearMapping(s.DataDir, s.SelectedWheelID, signatures); err != nil {
		messageBox(mainWnd, err.Error(), "H-Shifter lernen", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, "Neutral, 1–6 und Rückwärts liefern acht eindeutige Signaturen. Das H-Shifter-Mapping wurde für dieses physische Lenkrad gespeichert.", "H-Shifter gelernt", MB_OK|MB_ICONINFORMATION)
	refreshFastAsync()
}

func gearName(g int) string {
	if g < 0 {
		return "Rückwärts"
	}
	if g == 0 {
		return "Neutral"
	}
	return fmt.Sprintf("Gang %d", g)
}

func captureSteeringPoint(s system.State, label string) (uint32, bool) {
	messageBox(mainWnd, "Lenkrad "+label+" stellen und dort halten. Danach OK wählen.", "Lenkwinkel kalibrieren", MB_OK|MB_ICONINFORMATION)
	j := system.ReadPreferredWheelInput(s)
	if ok, why := learningSampleOK(j); !ok {
		messageBox(mainWnd, why, "Lenkwinkel kalibrieren", MB_OK|MB_ICONERROR)
		return 0, false
	}
	return j.X, true
}

func learnSteering(s system.State) {
	if !system.HasReadableSelectedWheel(s) {
		return
	}
	ranges := []string{"900° · voller G25/G27/DFGT-Bereich", "720°", "540°", "360°", "270°"}
	vals := []int{900, 720, 540, 360, 270}
	idx := chooseCommand("LogiMate · Lenkwinkel", "Arbeitsbereich auswählen", "Der gewählte Bereich wird anschließend über ganz links → Mitte → ganz rechts kalibriert.", ranges, 5500)
	if idx < 0 {
		return
	}
	left, ok := captureSteeringPoint(s, "ganz nach links")
	if !ok {
		return
	}
	center, ok := captureSteeringPoint(s, "exakt in die Mitte")
	if !ok {
		return
	}
	right, ok := captureSteeringPoint(s, "ganz nach rechts")
	if !ok {
		return
	}
	c := system.SteeringCalibration{Left: left, Center: center, Right: right, RangeDegrees: vals[idx]}
	if err := system.SaveSteeringCalibration(s.DataDir, s.SelectedWheelID, c); err != nil {
		messageBox(mainWnd, err.Error(), "Lenkwinkel kalibrieren", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, fmt.Sprintf("Lenkwinkel gespeichert.\r\n\r\nLinks: %d\r\nMitte: %d\r\nRechts: %d\r\nBereich: ±%.0f°", left, center, right, float64(vals[idx])/2), "Lenkwinkel kalibriert", MB_OK|MB_ICONINFORMATION)
	refreshFastAsync()
}

func copyRawReport(s system.State) {
	j := system.ReadPreferredWheelInput(s)
	text := system.RawReportPretty(j)
	if len(j.RawReport) > 0 {
		text += "\r\nHEX: " + system.RawReportHex(j)
	}
	if setClipboardText(mainWnd, text) {
		setActionFeedback("Aktueller Rohreport wurde kopiert.")
	} else {
		messageBox(mainWnd, "Zwischenablage konnte nicht geöffnet werden.", "Rohreport", MB_OK|MB_ICONERROR)
	}
}

func pedalCurveChoice() string {
	choices := []string{"Linear\nDirekte 1:1-Kennlinie", "Progressiv\nFeiner am Anfang, stärker zum Ende", "Weich\nSchneller Anstieg am Anfang"}
	idx := chooseCommand("LogiMate · Pedalkurve", "Kennlinie auswählen", "Die Kurve wird pro physischem Wheel gespeichert und erst nach Minimum/Maximum/Invertierung angewendet.", choices, 5600)
	switch idx {
	case 1:
		return "progressiv"
	case 2:
		return "weich"
	default:
		return "linear"
	}
}

func pedalDeadzoneChoice() float64 {
	choices := []string{
		"0 %\nKeine zusätzliche Deadzone",
		"2 %\nEmpfohlen für saubere Pedale",
		"5 %\nFür leichtes Pedalrauschen",
		"10 %\nFür deutliches Pedalrauschen oder verschlissene Potis",
	}
	idx := chooseCommand("LogiMate · Pedal-Deadzone", "Deadzone auswählen", "Der Bereich am Ruhepunkt wird pro physischem Wheel gespeichert und anschließend neu auf 0–100 % skaliert.", choices, 5650)
	switch idx {
	case 0:
		return 0
	case 2:
		return .05
	case 3:
		return .10
	default:
		return .02
	}
}

func axisCal(rest, pressed uint32, curve string, deadzone float64) system.AxisCalibration {
	min, max := rest, pressed
	if min > max {
		min, max = max, min
	}
	return system.AxisCalibration{Min: min, Max: max, Inverted: pressed < rest, Deadzone: deadzone, Curve: curve}
}

func axisValue(j system.JoyState, axis string) uint32 { v, _ := system.AxisRawValue(j, axis); return v }

func explainInputTelemetry(j system.JoyState) string {
	age := "—"
	if j.LastReportAge > 0 {
		age = fmt.Sprintf("%.2f s", j.LastReportAge.Seconds())
	}
	err := strings.TrimSpace(j.LastInputError)
	if err == "" {
		err = "kein Fehler"
	}
	return fmt.Sprintf("%.1f Hz · letzter Report %s · %d Reconnects · %s", j.ReportRateHz, age, j.Reconnects, err)
}

func showNativeOutputTools(s system.State) {
	if !getUISettings().NativeWheelOutput {
		messageBox(mainWnd, "Aktiviere zuerst Einstellungen → Native Wheel Output (Experimental).", "Native Output", MB_OK|MB_ICONWARNING)
		return
	}
	choices := []string{
		"Hardware-Steuerung\nLenkwinkel, G27 Rev-LEDs und kurzzeitiges Autocenter",
		"FFB-Effekttests\nConstant, Spring, Damper und Friction mit Watchdog testen",
		"Wheel-Engine-Profil\nAktives Profil wählen, anwenden oder Custom-Profil erstellen",
		"FFB-Signalformung\nLow-Pass, Smoothing, Deadband, Minimum Force und Response Curve",
		"FFB-Sicherheitsprofil\nHarte Motorlimits, Slew-Limit und Watchdog pro Wheel",
		"FFB Engine Status\nPipeline, Clipping, Latenz, Owner und Heartbeat",
		"Engine Dry Run\nMixer und alle vier Effekt-Slots ohne Motorbewegung prüfen",
		"EMERGENCY STOP\nAlle vier FFB-Slots, Autocenter und G27-LEDs stoppen",
		"Output-Status\nLetzten HID-Befehl, Fehler und Stop-Zähler anzeigen",
	}
	idx := chooseCommand("LogiMate · Native Wheel Engine", "Experimentelle native Wheel-Ausgabe", "Nur für ein eindeutig ausgewähltes, PnP-verifiziertes Wheel im Generic-HID-Modus. Es darf kein zweiter Hardware-Writer aktiv sein. Alle FFB-Hardwaretests sind kurzzeitig, profiliert und durch harte Safety-Limits begrenzt.", choices, 5750)
	switch idx {
	case 0:
		showNativeHardwareControls(s)
	case 1:
		showNativeFFBEffectTests(s)
	case 2:
		showNativeEngineProfiles(s)
	case 3:
		showAdvancedFFBTuning(s)
	case 4:
		showNativeFFBSafetyProfile(s)
	case 5:
		showNativeFFBEngineStatus(s)
	case 6:
		messageBox(mainWnd, system.NativeEngineDryRun(s.DataDir, s.SelectedWheelID), "Native Engine Dry Run", MB_OK|MB_ICONINFORMATION)
	case 7:
		if err := system.NativeOutputEmergencyStop(s); err != nil {
			messageBox(mainWnd, err.Error(), "Emergency Stop", MB_OK|MB_ICONERROR)
		} else {
			setActionFeedback("Native Output und alle FFB-Slots sicher gestoppt.")
		}
	case 8:
		st := system.NativeOutputSnapshot()
		last := "—"
		if !st.LastWrite.IsZero() {
			last = st.LastWrite.Format("15:04:05")
		}
		messageBox(mainWnd, fmt.Sprintf("Aktiv: %v\r\nWheel: %s\r\nModell: %s\r\nLetzter Befehl: %s\r\nLetzter Fehler: %s\r\nBefehle: %d\r\nEmergency Stops: %d\r\nWatchdog Stops: %d\r\nLetzter Write: %s", st.Active, st.WheelID, st.Model, st.LastCommand, st.LastError, st.CommandCount, st.EmergencyStops, st.WatchdogStops, last), "Native Output Status", MB_OK|MB_ICONINFORMATION)
	}
}

func showNativeHardwareControls(s system.State) {
	choices := []string{
		"Lenkwinkel senden\n270 / 360 / 540 / 720 / 900° direkt ans Wheel senden",
		"Rev-LEDs testen\nCapability-basiert 0–5 LEDs setzen",
		"Autocenter kurz testen\nMaximal 30 %, automatischer Watchdog",
	}
	idx := chooseCommand("Native Wheel Engine · Hardware", "Direkte Hardware-Steuerung", "Diese Befehle verwenden denselben exklusiven Output-Gate wie die FFB-Engine.", choices, 5800)
	switch idx {
	case 0:
		ranges := []string{"900°", "720°", "540°", "360°", "270°"}
		vals := []int{900, 720, 540, 360, 270}
		r := chooseCommand("Native Output · Lenkwinkel", "Hardwarebereich setzen", "Der Befehl wird unmittelbar an das ausgewählte Wheel gesendet.", ranges, 5850)
		if r >= 0 {
			if err := system.NativeOutputApplyRange(s, vals[r]); err != nil {
				messageBox(mainWnd, err.Error(), "Native Output", MB_OK|MB_ICONERROR)
			} else {
				setActionFeedback(fmt.Sprintf("Native Lenkwinkel: %d°", vals[r]))
			}
		}
	case 1:
		if !system.SelectedWheelHasRPMLEDs(s) {
			messageBox(mainWnd, "Das ausgewählte Wheel besitzt keine freigegebene Rev-LED-Capability.", "Native Output", MB_OK|MB_ICONINFORMATION)
			return
		}
		levels := []string{"Aus", "1 LED", "2 LEDs", "3 LEDs", "4 LEDs", "5 LEDs"}
		l := chooseCommand("Rev-LEDs", "LED-Bar testen", "EMERGENCY STOP schaltet die LEDs ebenfalls wieder aus.", levels, 5900)
		if l >= 0 {
			var mask byte
			if l > 0 {
				mask = byte((1 << uint(l)) - 1)
			}
			if err := system.NativeOutputSetLEDs(s, mask); err != nil {
				messageBox(mainWnd, err.Error(), "Native Output", MB_OK|MB_ICONERROR)
			} else {
				setActionFeedback(fmt.Sprintf("Rev-LED-Maske 0x%02X gesendet.", mask))
			}
		}
	case 2:
		if messageBox(mainWnd, "Das Lenkrad kann sich kurz selbst zentrieren. Hände locker lassen und nichts einklemmen. Der Watchdog setzt den Effekt automatisch auf 0. Fortfahren?", "Autocenter-Test", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
			return
		}
		levels := []string{"10 % · sanft", "20 %", "30 % · Audit-Maximum"}
		vals := []int{10, 20, 30}
		l := chooseCommand("Autocenter-Test", "Stärke auswählen", "Der Test ist absichtlich kurz und begrenzt.", levels, 6000)
		if l >= 0 {
			if err := system.NativeOutputTestAutocenter(s, vals[l], 2); err != nil {
				messageBox(mainWnd, err.Error(), "Native Output", MB_OK|MB_ICONERROR)
			} else {
				setActionFeedback(fmt.Sprintf("Autocenter %d %% · Watchdog aktiv.", vals[l]))
			}
		}
	}
}

func showNativeFFBEffectTests(s system.State) {
	choices := []string{
		"Constant Force\nKurzer Links-/Rechts-Krafttest über den einheitlichen Native-Output",
		"Spring\nZentrierender Condition-Test über Slot 1",
		"Damper\nGeschwindigkeitsabhängiger Widerstands-Test über Slot 2",
		"Friction\nReibungs-Condition-Test über Slot 3",
	}
	idx := chooseCommand("Native FFB · Effekttests", "Welchen Effekt möchtest du testen?", "Jeder Test besitzt einen Watchdog. Profil- und Safety-Gains können die tatsächlich angelegte Stärke weiter reduzieren.", choices, 6050)
	if idx < 0 {
		return
	}
	if messageBox(mainWnd, "Dieser Hardwaretest kann Motorwiderstand bzw. Lenkkraft erzeugen. Hände locker lassen, nichts einklemmen und jederzeit EMERGENCY STOP verwenden. Fortfahren?", "Native FFB Hardwaretest", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
		return
	}
	if idx == 0 {
		levels := []string{"5 % nach links", "10 % nach links", "5 % nach rechts", "10 % nach rechts"}
		vals := []int{-5, -10, 5, 10}
		l := chooseCommand("Native FFB · Constant", "Kraft auswählen", "Die angelegte Kraft läuft zusätzlich durch aktives Engine-Profil und Safety-Profil.", levels, 6100)
		if l >= 0 {
			if err := system.StartNativeConstantForceTest(s, vals[l]); err != nil {
				messageBox(mainWnd, err.Error(), "Native FFB", MB_OK|MB_ICONERROR)
			} else {
				setActionFeedback(fmt.Sprintf("Constant-Test %+d %% angefordert · Watchdog aktiv.", vals[l]))
			}
		}
		return
	}
	levels := []string{"10 % · sanft", "20 %", "30 % · Test-Maximum"}
	vals := []int{10, 20, 30}
	l := chooseCommand("Native FFB · Condition", "Anforderung auswählen", "Die tatsächliche Stärke wird vom Force-Mixer aus Engine-Profil und Safety-Limits berechnet.", levels, 6150)
	if l < 0 {
		return
	}
	var err error
	name := ""
	switch idx {
	case 1:
		name, err = "Spring", system.StartNativeSpringTest(s, vals[l])
	case 2:
		name, err = "Damper", system.StartNativeDamperTest(s, vals[l])
	case 3:
		name, err = "Friction", system.StartNativeFrictionTest(s, vals[l])
	}
	if err != nil {
		messageBox(mainWnd, err.Error(), "Native FFB", MB_OK|MB_ICONERROR)
		return
	}
	setActionFeedback(fmt.Sprintf("%s-Test %d %% angefordert · Watchdog aktiv.", name, vals[l]))
}

func showNativeEngineProfiles(s system.State) {
	if strings.TrimSpace(s.SelectedWheelID) == "" {
		messageBox(mainWnd, "Kein physisches Wheel ausgewählt.", "Wheel-Engine-Profil", MB_OK|MB_ICONWARNING)
		return
	}
	profiles := system.ListNativeEngineProfiles(s.DataDir, s.SelectedWheelID)
	active := system.ReadActiveNativeEngineProfile(s.DataDir, s.SelectedWheelID)
	choices := make([]string, 0, len(profiles)+1)
	for _, p := range profiles {
		prefix := ""
		if strings.EqualFold(p.Name, active.Name) {
			prefix = "AKTIV · "
		}
		choices = append(choices, prefix+system.NativeEngineProfileSummary(p))
	}
	choices = append(choices, "Custom-Profil erstellen\nRotation und Gain-Charakteristik für dieses physische Wheel speichern")
	idx := chooseCommand("LogiMate · Wheel-Engine-Profile", "Profil auswählen", "Ein Profil speichert die gewünschte Rotation und Gain-Stufen. Harte Motorlimits bleiben separat im FFB-Sicherheitsprofil und können durch ein Profil niemals erhöht werden.", choices, 6300)
	if idx < 0 {
		return
	}
	if idx == len(profiles) {
		createCustomNativeEngineProfile(s)
		return
	}
	p := profiles[idx]
	if messageBox(mainWnd, "Profil „"+p.Name+"“ aktivieren? Dabei wird nur der Lenkwinkel unmittelbar an die Hardware gesendet. Motor-FFB startet dadurch nicht.", "Wheel-Engine-Profil", MB_YESNO|MB_ICONINFORMATION|MB_DEFBUTTON2) != IDYES {
		return
	}
	if err := system.ApplyNativeEngineProfile(s, p.Name); err != nil {
		messageBox(mainWnd, err.Error(), "Wheel-Engine-Profil", MB_OK|MB_ICONERROR)
		return
	}
	setActionFeedback("Wheel-Engine-Profil aktiv: " + system.NativeEngineProfileSummary(p))
}

func createCustomNativeEngineProfile(s system.State) {
	ranges := []string{"900°", "720°", "540°", "360°", "270°"}
	rangeVals := []int{900, 720, 540, 360, 270}
	r := chooseCommand("Custom Wheel-Engine-Profil", "Lenkwinkel", "Wähle den Hardware-Arbeitsbereich des Profils.", ranges, 6400)
	if r < 0 {
		return
	}
	masters := []string{"50 % · sanft", "75 % · ausgewogen", "100 % · direkt"}
	masterVals := []int{50, 75, 100}
	m := chooseCommand("Custom Wheel-Engine-Profil", "Master Gain", "Das separate Safety-Profil bleibt die harte Obergrenze.", masters, 6450)
	if m < 0 {
		return
	}
	characters := []string{
		"Komfort\nConstant 55 · Spring 45 · Damper 55 · Friction 45 %",
		"Balanced\nConstant 75 · Spring 65 · Damper 55 · Friction 50 %",
		"Direkt\nConstant 100 · Spring 85 · Damper 70 · Friction 65 %",
		"Individuell\nConstant, Spring, Damper und Friction getrennt einstellen",
	}
	gains := [][4]int{{55, 45, 55, 45}, {75, 65, 55, 50}, {100, 85, 70, 65}}
	g := chooseCommand("Custom Wheel-Engine-Profil", "Charakteristik", "Diese Werte werden vor den harten Safety-Limits angewendet.", characters, 6500)
	if g < 0 {
		return
	}
	selected := [4]int{}
	if g < len(gains) {
		selected = gains[g]
	} else {
		labels := []string{"0 %", "25 %", "50 %", "75 %", "100 %"}
		vals := []int{0, 25, 50, 75, 100}
		names := []string{"Constant Force", "Spring", "Damper", "Friction"}
		for i, name := range names {
			x := chooseCommand("Custom Wheel-Engine-Profil", name+" Gain", "Per-Effect-Gain vor dem separaten harten Safety-Limit.", labels, int32(6510+i*10))
			if x < 0 {
				return
			}
			selected[i] = vals[x]
		}
	}
	p := system.NativeEngineProfile{Name: "Custom", RotationDegrees: rangeVals[r], MasterGainPercent: masterVals[m], ConstantGainPercent: selected[0], SpringGainPercent: selected[1], DamperGainPercent: selected[2], FrictionGainPercent: selected[3]}
	if err := system.SaveNativeEngineProfile(s.DataDir, s.SelectedWheelID, p); err != nil {
		messageBox(mainWnd, err.Error(), "Custom-Profil", MB_OK|MB_ICONERROR)
		return
	}
	if err := system.ApplyNativeEngineProfile(s, p.Name); err != nil {
		messageBox(mainWnd, err.Error(), "Custom-Profil", MB_OK|MB_ICONERROR)
		return
	}
	setActionFeedback("Custom Wheel-Engine-Profil gespeichert und aktiviert: " + system.NativeEngineProfileSummary(p))
}

func showAdvancedFFBTuning(s system.State) {
	current, ok := system.ReadSelectedAdvancedFFBConfig(s)
	if !ok {
		messageBox(mainWnd, "Kein physisches Wheel ausgewählt.", "FFB-Signalformung", MB_OK|MB_ICONWARNING)
		return
	}
	presets, _ := system.SelectedAdvancedFFBPresets(s)
	choices := make([]string, 0, len(presets)+2)
	for _, p := range presets {
		choices = append(choices, p.Name+"\n"+p.Description)
	}
	choices = append(choices, "Custom\nDeadband, Minimum Force, Response Curve, Low-Pass und Smoothing einzeln wählen")
	choices = append(choices, "Erweiterten FFB-Status anzeigen\n"+system.SelectedAdvancedFFBConfigSummary(s))
	idx := chooseCommand("LogiMate · FFB-Signalformung", "Signalformung vor dem Safety-Mixer", "Die Signalformung verändert nur den semantischen Game-Force-Stream. Das FFB-Sicherheitsprofil wird IMMER danach angewendet und kann von Minimum Force oder Response Curves nicht überschritten werden.", choices, 6320)
	if idx < 0 {
		return
	}
	if idx < len(presets) {
		cfg := presets[idx].Config
		if err := system.SaveSelectedAdvancedFFBConfig(s, cfg); err != nil {
			messageBox(mainWnd, err.Error(), "FFB-Signalformung", MB_OK|MB_ICONERROR)
			return
		}
		setActionFeedback("FFB-Signalformung gespeichert: " + system.SelectedAdvancedFFBConfigSummary(s))
		return
	}
	if idx == len(presets)+1 {
		messageBox(mainWnd, system.SelectedAdvancedFFBConfigSummary(s), "FFB-Signalformung", MB_OK|MB_ICONINFORMATION)
		return
	}

	cfg := current
	cfg.Enabled = true
	cfg.Preset = "Custom"
	deadLabels := []string{"0 %", "1 %", "2 %", "3 %", "5 %"}
	deadVals := []int{0, 1, 2, 3, 5}
	d := chooseCommand("FFB Custom", "Deadband", "Kleine Eingangskräfte innerhalb dieser Zone werden auf 0 gesetzt; der Rest wird auf den vollen Bereich neu skaliert.", deadLabels, 6330)
	if d < 0 {
		return
	}
	cfg.DeadbandPercent = deadVals[d]
	minLabels := []string{"0 % · keine Anhebung", "2 %", "3 %", "5 % · experimentell"}
	minVals := []int{0, 2, 3, 5}
	m := chooseCommand("FFB Custom", "Minimum Force", "Wird NACH Deadband/Response Curve, aber VOR dem harten Safety-Cap angewendet.", minLabels, 6340)
	if m < 0 {
		return
	}
	cfg.MinimumForcePercent = minVals[m]
	curveLabels := []string{"0.80 · mehr Details um die Mitte", "0.90", "1.00 · linear", "1.10", "1.25 · sanfter um die Mitte"}
	curveVals := []float64{.80, .90, 1, 1.10, 1.25}
	c := chooseCommand("FFB Custom", "Response Curve", "Exponent <1 hebt kleine Kräfte relativ an; >1 macht die Mitte sanfter. Das Safety-Cap bleibt danach aktiv.", curveLabels, 6350)
	if c < 0 {
		return
	}
	cfg.ResponseExponent = curveVals[c]
	lpfLabels := []string{"Aus", "25 Hz · sehr glatt", "35 Hz · glatt", "45 Hz · ausgewogen", "60 Hz · direkt", "80 Hz · sehr direkt"}
	lpfVals := []int{0, 25, 35, 45, 60, 80}
	l := chooseCommand("FFB Custom", "Low-Pass", "Ein dynamischer 1-Pole-Low-Pass reduziert hochfrequentes Rattern. 0 deaktiviert ihn.", lpfLabels, 6360)
	if l < 0 {
		return
	}
	cfg.LowPassHz = lpfVals[l]
	smoothLabels := []string{"0 %", "4 %", "8 %", "12 %", "20 %"}
	smoothVals := []int{0, 4, 8, 12, 20}
	sm := chooseCommand("FFB Custom", "Smoothing", "Zusätzliche exponentielle Glättung hinter dem Low-Pass.", smoothLabels, 6370)
	if sm < 0 {
		return
	}
	cfg.SmoothingPercent = smoothVals[sm]
	if err := system.SaveSelectedAdvancedFFBConfig(s, cfg); err != nil {
		messageBox(mainWnd, err.Error(), "FFB-Signalformung", MB_OK|MB_ICONERROR)
		return
	}
	setActionFeedback("FFB Custom gespeichert: " + system.SelectedAdvancedFFBConfigSummary(s))
}

func showNativeFFBEngineStatus(s system.State) {
	st := system.NativeFFBSnapshot()
	game := system.NativeGameOutputSnapshot()
	heartbeat := "OK"
	if !system.NativeFFBHeartbeatHealthy(time.Now()) {
		heartbeat = "FEHLER / überfällig"
	}
	last := "—"
	if !st.LastHeartbeat.IsZero() {
		last = st.LastHeartbeat.Format("15:04:05.000")
	}
	profile := system.ReadActiveNativeEngineProfile(s.DataDir, s.SelectedWheelID)
	pipeline := system.SelectedAdvancedFFBConfigSummary(s)
	messageBox(mainWnd, fmt.Sprintf("Aktiv: %v\r\nOwner Generation: %d\r\nWheel: %s\r\nModell: %s\r\nEffekt: %s\r\nProfil: %s\r\nConstant: %+d %%\r\nSpring: %d %%\r\nDamper: %d %%\r\nFriction: %d %%\r\nFrames: %d\r\nSafety-Mixer-Clips: %d\r\nSlew-begrenzt: %d\r\nEffektwechsel: %d\r\nWatchdog Stops: %d\r\nEmergency Stops: %d\r\nHeartbeat: %s\r\nLetzter Heartbeat: %s\r\nLetzter Fehler: %s\r\n\r\nGame-FFB-Signalverarbeitung\r\nAdapter: %s\r\nPreset: %s\r\nTransport: %s\r\nRaw / shaped / applied: %+.3f / %+.3f / %+d %%\r\nPipeline-Clips: %d\r\nSafety-Clips: %d\r\nDeadband-Hits: %d\r\nMinimum-Force Anwendungen: %d\r\nSample-Latenz Ø / Max: %s / %s\r\nPumps / HID Writes: %d / %d\r\n\r\n%s\r\n%s\r\n%s", st.Active, st.Generation, st.WheelID, st.Model, st.Effect, st.ProfileName, st.Applied, st.SpringApplied, st.DamperApplied, st.FrictionApplied, st.Frames, st.ClipEvents, st.SlewLimited, st.EffectTransitions, st.WatchdogStops, st.EmergencyStops, heartbeat, last, st.LastError, game.Adapter, game.FFBPreset, game.TransportBackend, game.RequestedForce, game.ShapedForce, game.AppliedPercent, game.PipelineClips, game.SafetyClips, game.PipelineDeadband, game.PipelineMinForce, game.SampleLatency, game.MaxSampleLatency, game.Pumps, game.Writes, system.NativeEngineProfileSummary(profile), pipeline, system.NativeFFBConfigSummary(st.Config)), "Native FFB Engine", MB_OK|MB_ICONINFORMATION)
}

func showNativeFFBSafetyProfile(s system.State) {
	if strings.TrimSpace(s.SelectedWheelID) == "" {
		messageBox(mainWnd, "Kein physisches Wheel ausgewählt.", "FFB-Sicherheitsprofil", MB_OK|MB_ICONWARNING)
		return
	}
	presets := []string{
		"Sanft\nMaster 50 % · Constant cap 5 % · Conditions 10 % · Slew 1 %/20ms · 900ms Watchdog",
		"Standard Audit\nMaster 75 % · Constant cap 8 % · Conditions 20 % · Slew 2 %/20ms · 1200ms Watchdog",
		"Audit Maximum\nMaster 100 % · Constant cap 10 % · Conditions 30 % · Slew 3 %/20ms · 1500ms Watchdog",
	}
	idx := chooseCommand("LogiMate · FFB-Sicherheitsprofil", "Harte Sicherheitsgrenzen auswählen", "Diese Limits gelten pro physischem Wheel und werden nach allen Wheel-Engine-Profil-Gains angewendet. Sie können von Profilen oder einer späteren Spielquelle nicht überschritten werden.", presets, 6200)
	if idx < 0 {
		return
	}
	cfgs := []system.NativeFFBConfig{
		{MasterGainPercent: 50, ConstantLimit: 5, SpringGain: 10, DamperGain: 10, FrictionGain: 10, SlewPerTick: 1, WatchdogMS: 900},
		{MasterGainPercent: 75, ConstantLimit: 8, SpringGain: 20, DamperGain: 20, FrictionGain: 20, SlewPerTick: 2, WatchdogMS: 1200},
		{MasterGainPercent: 100, ConstantLimit: 10, SpringGain: 30, DamperGain: 30, FrictionGain: 30, SlewPerTick: 3, WatchdogMS: 1500},
	}
	if err := system.SaveNativeFFBConfig(s.DataDir, s.SelectedWheelID, cfgs[idx]); err != nil {
		messageBox(mainWnd, err.Error(), "FFB-Sicherheitsprofil", MB_OK|MB_ICONERROR)
		return
	}
	setActionFeedback("FFB-Sicherheitsprofil gespeichert: " + system.NativeFFBConfigSummary(cfgs[idx]))
}

func showGameProfileTools(s system.State) {
	choices := []string{"Profil für aktuelle Vordergrund-App anlegen", "OpenG27-Profile importieren", "Wreckfest-2-Preset aus Importformat hinzufügen", "Gespeicherte Profile anzeigen", "Aktive Game-Session anzeigen", "Profile als JSON exportieren"}
	i := chooseCommand("LogiMate · Game-Profile", "Spielbezogene Wheel-Profile", "Auto-Apply darf Profil und Lenkwinkel setzen, startet aber niemals automatisch FFB.", choices, 6500)
	switch i {
	case 0:
		exe := system.ForegroundProcessNameNative()
		if exe == "" {
			messageBox(mainWnd, "Es konnte keine Vordergrund-App erkannt werden.", "Game-Profile", MB_OK|MB_ICONWARNING)
			return
		}
		profiles := system.ListNativeEngineProfiles(s.DataDir, s.SelectedWheelID)
		labels := make([]string, len(profiles))
		for j, p := range profiles {
			labels[j] = p.Name + "\n" + system.NativeEngineProfileSummary(p)
		}
		pidx := chooseCommand("Game-Profil · "+exe, "Wheel-Engine-Profil auswählen", "Dieses Profil wird nur sicher angewendet; Motor-FFB startet nie automatisch.", labels, 6600)
		if pidx < 0 {
			return
		}
		gp := system.GameProfile{Name: exe, Executables: []string{exe}, Enabled: true, AutoApply: true, ForegroundOnly: false, EngineProfile: profiles[pidx].Name, MasterGainPercent: 100, ConstantGainPercent: 100, SpringGainPercent: 100, DamperGainPercent: 100, FrictionGainPercent: 100, LEDPolicy: "off"}
		if err := system.SaveGameProfile(s.DataDir, gp); err != nil {
			messageBox(mainWnd, err.Error(), "Game-Profile", MB_OK|MB_ICONERROR)
			return
		}
		setActionFeedback("Game-Profil für " + exe + " gespeichert.")
	case 1:
		path := system.FindOpenG27ProfileFile()
		if path == "" {
			messageBox(mainWnd, "Keine OpenG27 game-profiles.json gefunden. Erwartet wird %APPDATA%\\OpenG27\\game-profiles.json (oder der alte OpenG27FFB-Pfad).", "Legacy-Profilimport", MB_OK|MB_ICONWARNING)
			return
		}
		report, err := system.ImportOpenG27GameProfilesFile(s.DataDir, s.SelectedWheelID, path)
		if err != nil {
			messageBox(mainWnd, err.Error(), "Legacy-Profilimport", MB_OK|MB_ICONERROR)
			return
		}
		messageBox(mainWnd, system.OpenG27ProfileImportSummary(report)+"\r\n\r\nSafety-Profile wurden nicht verändert. Motor-FFB startet durch den Import nicht automatisch.", "Legacy-Profilimport", MB_OK|MB_ICONINFORMATION)
		setActionFeedback(fmt.Sprintf("Legacy-Import: %d Profile importiert.", report.Imported))
	case 2:
		report, err := system.InstallOpenG27Wreckfest2Preset(s.DataDir, s.SelectedWheelID)
		if err != nil {
			messageBox(mainWnd, err.Error(), "Wreckfest 2 Preset", MB_OK|MB_ICONERROR)
			return
		}
		messageBox(mainWnd, system.OpenG27ProfileImportSummary(report)+"\r\n\r\nPino-Telemetrie, Game-FFB (opt-in) und RPM-LEDs sind vorbereitet.", "Wreckfest 2 Preset", MB_OK|MB_ICONINFORMATION)
	case 3:
		ps := system.ListGameProfiles(s.DataDir)
		if len(ps) == 0 {
			messageBox(mainWnd, "Noch keine Game-Profile gespeichert.", "Game-Profile", MB_OK|MB_ICONINFORMATION)
			return
		}
		var b strings.Builder
		for _, p := range ps {
			fmt.Fprintf(&b, "%s · EXE=%s · Engine=%s · Auto=%v · Telemetry=%s\r\n", p.Name, strings.Join(p.Executables, ","), p.EngineProfile, p.AutoApply, p.TelemetryAdapter)
		}
		messageBox(mainWnd, b.String(), "Game-Profile", MB_OK|MB_ICONINFORMATION)
	case 4:
		gs := system.GameSessionSnapshot()
		messageBox(mainWnd, fmt.Sprintf("Aktiv: %v\r\nProzess: %s\r\nProfil: %s\r\nEngine: %s\r\nAuto angewendet: %v\r\nStatus: %s", gs.Active, gs.Process, gs.ProfileName, gs.EngineProfile, gs.AutoApplied, gs.Message), "Game-Session", MB_OK|MB_ICONINFORMATION)
	case 5:
		b, err := system.ExportGameProfilesJSON(s.DataDir)
		if err != nil {
			messageBox(mainWnd, err.Error(), "Export", MB_OK|MB_ICONERROR)
			return
		}
		path := filepath.Join(s.DataDir, "game-profiles-export.json")
		if err = os.WriteFile(path, b, 0644); err != nil {
			messageBox(mainWnd, err.Error(), "Export", MB_OK|MB_ICONERROR)
			return
		}
		setActionFeedback("Game-Profile exportiert: " + path)
	}
}

func showTelemetryTools(s system.State) {
	choices := []string{"Lokalen JSON-Adapter starten · 127.0.0.1:27100", "Wreckfest 2 Pino starten · 127.0.0.1:23123", "Native Game Output · FFB/LEDs starten", "Native Game Output · Hardware-Ausgabe stoppen", "Telemetry Adapter stoppen", "Status anzeigen", "RPM/LED + Force sicher vorschauen", "Adapter-Katalog anzeigen"}
	i := chooseCommand("LogiMate · Telemetry Hub", "Lokale Telemetrie", "Netzwerkempfang ist auf Loopback beschränkt. Vorschau sendet keinerlei Motorbefehle.", choices, 6700)
	switch i {
	case 0:
		if err := system.StartLocalTelemetryJSON(27100); err != nil {
			messageBox(mainWnd, err.Error(), "Telemetry", MB_OK|MB_ICONERROR)
		} else {
			setActionFeedback("Telemetry Hub läuft auf 127.0.0.1:27100.")
		}
	case 1:
		if err := system.StartWreckfestPinoTelemetry(23123); err != nil {
			messageBox(mainWnd, err.Error(), "Wreckfest 2 Pino", MB_OK|MB_ICONERROR)
		} else {
			setActionFeedback("Wreckfest-2-Pino lauscht auf 127.0.0.1:23123.")
		}
	case 2:
		p, exe, ok := system.ActiveGameProfile(s.DataDir)
		if !ok {
			messageBox(mainWnd, "Kein aktives Game-Profil erkannt.", "Native Game Output", MB_OK|MB_ICONWARNING)
			return
		}
		if err := system.StartNativeGameOutput(s, p, exe); err != nil {
			messageBox(mainWnd, err.Error(), "Native Game Output", MB_OK|MB_ICONERROR)
			return
		}
		setActionFeedback("Native Game-Output gestartet. Adapter- und Safety-Limits bleiben aktiv.")
	case 3:
		system.StopNativeGameOutput("user")
		setActionFeedback("Native Game-Output gestoppt.")
	case 4:
		system.StopNativeGameOutput("telemetry-stopped")
		system.StopTelemetry()
		setActionFeedback("Telemetry Hub gestoppt.")
	case 5:
		t := system.TelemetrySnapshot()
		messageBox(mainWnd, fmt.Sprintf("Running: %v\r\nAdapter: %s\r\nAdresse: %s\r\nPakete: %d (%.1f Hz)\r\nFrames: %d (%.1f Hz)\r\nAlter letzter Frame: %s\r\nUngültig: %d\r\nStale: %v\r\nLetzter Fehler: %s\r\n\r\n%s", t.Running, t.Adapter, t.Address, t.Packets, t.PacketRateHz, t.Frames, t.FrameRateHz, t.LastFrameAge.Round(time.Millisecond), t.Invalid, t.Stale, t.LastError, system.NativeGameOutputSummary()), "Telemetry Status", MB_OK|MB_ICONINFORMATION)
	case 6:
		t := system.TelemetrySnapshot()
		f := t.LastFrame
		messageBox(mainWnd, fmt.Sprintf("RPM: %d / Redline %d\r\nLogiMate LED-Maske: 0x%02X\r\nForce roh: %.3f\r\nForce sicher: %+d%%\r\nPhysics: %v · PlayerControl: %v\r\n\r\nNur Vorschau — keine neue Hardware-Ausgabe.", f.RPM, f.RPMRedline, system.RPMToG27LEDMask(f.RPM, f.RPMRedline, f.RPMMax), f.Force, system.TelemetryForcePercent(f), f.Physics, f.PlayerControl), "Telemetry Vorschau", MB_OK|MB_ICONINFORMATION)
	case 7:
		var b strings.Builder
		for _, a := range system.RegisteredTelemetryAdapters() {
			fmt.Fprintf(&b, "%s (%s) · Port %d · Auto=%v · RPM=%v · Force=%v · FFB-Auth=%v\r\n%s\r\n\r\n", a.Name, a.ID, a.Port, a.Automatic, a.ProvidesRPM, a.ProvidesForce, a.ProvidesFFBAuth, a.Setup)
		}
		messageBox(mainWnd, b.String(), "Telemetry Adapter", MB_OK|MB_ICONINFORMATION)
	}
}

func showReleaseTrust() {
	text := system.ReleaseTrustSummary(Version)
	text += "\r\n\r\nStable-Vertrag:\r\n- LogiMate.exe muss vor dem Einbetten in den Installer gültig Authenticode-signiert sein.\r\n- LogiMate-Setup-x64.exe muss nach dem Build ebenfalls gültig signiert sein.\r\n- Beide Artefakte müssen denselben erwarteten Publisher-Thumbprint tragen.\r\n- Der Stable-Build bleibt zusätzlich durch Hardware-, Recovery-, Migration-, UIA- und HID-Stress-Evidenz gesperrt."
	messageBox(mainWnd, text, "LogiMate · Release Trust", MB_OK|MB_ICONINFORMATION)
}

func buildEngineHealthText(s system.State, tests []system.SelfTestResult) string {
	inv := system.ValidateStateInvariants(s)
	ready := system.BuildReadinessReport(s)
	rec, detail := system.RuntimeOutputRecoveryNeeded(s.DataDir)
	ffb := system.NativeFFBSnapshot()
	tel := system.TelemetrySnapshot()
	var b strings.Builder
	fmt.Fprintf(&b, "LIVE · %s · automatische Aktualisierung\r\n", time.Now().Format("15:04:05.000"))
	fmt.Fprintf(&b, "Readiness Score: %d/100 · Code-Audit vollständig: %v\r\n", ready.Score, ready.CodeAuditComplete)
	fmt.Fprintf(&b, "Wheel: %s · Mode: %s · Native engine: standalone\r\n", s.WheelModel, s.ActiveMode)
	phase, activationDetail, activationRunning := system.NativeActivationStatus()
	fmt.Fprintf(&b, "Native activation: phase=%s running=%v detail=%s\r\n", phase, activationRunning, activationDetail)
	fmt.Fprintf(&b, "Native FFB: active=%v effect=%s requested=%+d%% applied=%+d%% heartbeat=%v\r\n", ffb.Active, ffb.Effect, ffb.Requested, ffb.Applied, system.NativeFFBHeartbeatHealthy(time.Now()))
	fmt.Fprintf(&b, "Telemetry: running=%v adapter=%s frames=%d stale=%v\r\n", tel.Running, tel.Adapter, tel.Frames, tel.Stale)
	fmt.Fprintf(&b, "Output-Recovery-Marker: %v %s\r\n\r\nSelbsttests:\r\n", rec, detail)
	for _, t := range tests {
		fmt.Fprintf(&b, "- %s: %v (%s)\r\n", t.Name, t.Passed, t.Detail)
	}
	b.WriteString("\r\nLegacy builder · Native protocol comparison:\r\n")
	b.WriteString(system.FusionC2ProtocolShadowSummary())
	b.WriteString("\r\n\r\nNative scheduler diagnostics:\r\n")
	b.WriteString(system.FusionC3SchedulerSummary())
	b.WriteString("\r\n\r\nNative G27 parser diagnostics:\r\n")
	b.WriteString(system.FusionC4ParserSummary())
	b.WriteString("\r\n\r\nNative Game Adapter:\r\n")
	b.WriteString(system.NativeGameOutputSummary())
	b.WriteString("\r\n\r\nStandalone Native Engine:\r\n")
	b.WriteString(system.FusionC7ParitySummary(s))
	b.WriteString("\r\n\r\nAccessibility / UI Automation:\r\n")
	b.WriteString(accessibilityRuntimeSummary())
	b.WriteString("\r\n\r\nHID Stress / adverse I/O:\r\n")
	if stress, err := system.HIDStressSoftwareSelfTest(512); err != nil {
		b.WriteString("Software policy stress: FAIL · " + err.Error() + "\r\n")
	} else {
		fmt.Fprintf(&b, "Software policy stress: PASS · %s · %s\r\n", stress.Detail, stress.Duration.Round(time.Microsecond))
	}
	b.WriteString(system.NativeHIDTransportSummary())
	b.WriteString("\r\n")
	b.WriteString(system.FusionC4DeviceLifecycleSummary(s))
	b.WriteString("\r\n\r\nRelease Trust / Authenticode:\r\n")
	b.WriteString(system.ReleaseTrustSummary(Version))
	b.WriteString("\r\n\r\nState-Invarianten:\r\n")
	if len(inv) == 0 {
		b.WriteString("- keine Verletzungen\r\n")
	}
	for _, x := range inv {
		fmt.Fprintf(&b, "- %s/%s: %s\r\n", x.Severity, x.Code, x.Message)
	}
	b.WriteString("\r\nExterne 1.0-Gates:\r\n- " + strings.Join(ready.ExternalGates, "\r\n- "))
	return b.String()
}

func showEngineHealth(s system.State) {
	// Engine Health used to build one string and pass it to messageBox(), so the
	// values could never change while the window was open. The modern dialog now
	// owns a timer and calls this provider on the UI thread every 500 ms.
	tests := system.RunInternalSelfTests(s.DataDir)
	lastTests := time.Now()
	provider := func() string {
		stateMu.RLock()
		live := appStateSnapshot()
		stateMu.RUnlock()
		if live.DataDir == "" {
			live = s
		}
		if time.Since(lastTests) >= 4*time.Second {
			tests = system.RunInternalSelfTests(live.DataDir)
			lastTests = time.Now()
		}
		return buildEngineHealthText(live, tests)
	}
	_, _ = runModernDialog(modernDialogSpec{
		Parent: mainWnd, WindowTitle: "LogiMate · Engine Health",
		Heading: "Engine Health · Live", Subtitle: "FFB, Parser, Telemetrie und Safety-Zustand werden live aktualisiert",
		Kind: "info", Content: provider(), LiveContent: provider, RefreshMS: 500,
		Buttons:   []modernDialogButton{{ID: IDOK, Title: "Schließen", Primary: true}},
		DefaultID: IDOK, CancelID: IDOK, Width: 860, Height: 720,
	})
}
