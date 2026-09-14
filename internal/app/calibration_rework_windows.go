//go:build windows

package app

import (
	"fmt"
	"strings"

	"github.com/thelittlespace/LogiMate/internal/system"
)

// Build 003: non-modal calibration state owned by the Wheel/Calibration page.
// Captures are explicit clicks on the current valid HID sample; no calibration
// value is persisted until the complete wizard validates successfully.
type calibrationWizardState struct {
	Active bool
	Kind   string
	Step   int
	Error  string
	Status string

	RangeDegrees int
	SteerLeft    uint32
	SteerCenter  uint32
	SteerRight   uint32

	PedalBase     system.JoyState
	PedalExcluded map[string]bool
	PedalGas      calibrationPedalCapture
	PedalBrake    calibrationPedalCapture
	PedalClutch   calibrationPedalCapture
	Deadzone      float64
	Curve         string

	ButtonTarget string
	ButtonBefore system.JoyState

	GearSignatures map[string]int
}

type calibrationPedalCapture struct {
	Axis          string
	Rest, Pressed uint32
}

var (
	wheelCalibrationWizard         calibrationWizardState
	wheelCalibrationWizardRects    [6]RECT
	wheelCalibrationWizardHover    = -1
	wheelCalibrationWizardLiveRect RECT
)

var calibrationButtonTargets = []string{"Paddle L", "Paddle R", "Wheel 1", "Wheel 2", "Wheel 3", "Wheel 4", "Wheel 5", "Wheel 6", "Shifter 1", "Shifter 2", "Shifter 3", "Shifter 4", "Shifter 5", "Shifter 6", "Shifter 7", "Shifter 8"}
var calibrationRanges = []int{900, 720, 540, 360, 270}
var calibrationDeadzones = []float64{0, .02, .05, .10}
var calibrationCurves = []string{"linear", "progressiv", "weich"}

func resetCalibrationWizardRects() {
	for i := range wheelCalibrationWizardRects {
		wheelCalibrationWizardRects[i] = RECT{}
	}
}

func startInlineCalibration(kind string, s system.State) {
	w := calibrationWizardState{Active: true, Kind: kind, RangeDegrees: 900, Deadzone: .02, Curve: "linear", PedalExcluded: map[string]bool{}, GearSignatures: map[string]int{}}
	p := system.ReadControlProfile(s.DataDir, s.SelectedWheelID)
	if p.Steering.RangeDegrees > 0 {
		w.RangeDegrees = p.Steering.RangeDegrees
	}
	for _, target := range calibrationButtonTargets {
		if _, exists := p.Buttons[target]; !exists {
			w.ButtonTarget = target
			break
		}
	}
	if w.ButtonTarget == "" {
		w.ButtonTarget = calibrationButtonTargets[0]
	}
	switch kind {
	case "steering":
		w.Status = "Bereich prüfen, dann Lenkrad ganz nach links stellen."
	case "pedals":
		w.Status = "Alle Pedale vollständig loslassen und Ruheposition erfassen."
	case "button":
		w.Status = "Ziel wählen, alle Tasten loslassen und Ausgangslage erfassen."
	case "shifter":
		if !system.IsG25Model(s.WheelModel) && !system.IsG27Model(s.WheelModel) {
			w.Error = "Der H-Shifter-Assistent ist nur für G25/G27 verfügbar."
		}
		w.Status = "Neutral einlegen und den aktuellen Zustand erfassen."
	}
	wheelCalibrationWizard = w
	wheelCalibrationWizardHover = -1
	contentScroll = 0
	invalidate(mainWnd)
}

func closeInlineCalibration() {
	wheelCalibrationWizard = calibrationWizardState{}
	wheelCalibrationWizardHover = -1
	resetCalibrationWizardRects()
	invalidate(mainWnd)
}

func calibrationWizardTitle() string {
	switch wheelCalibrationWizard.Kind {
	case "steering":
		return "Lenkung kalibrieren"
	case "pedals":
		return "Pedale kalibrieren"
	case "button":
		return "Taste / Wippe lernen"
	case "shifter":
		return "H-Shifter lernen"
	default:
		return "Kalibrierung"
	}
}

func calibrationWizardStepText(s system.State) (string, string) {
	w := wheelCalibrationWizard
	switch w.Kind {
	case "steering":
		labels := []string{"Ganz links", "Mitte", "Ganz rechts", "Fertig"}
		i := w.Step
		if i >= len(labels) {
			i = len(labels) - 1
		}
		return fmt.Sprintf("Schritt %d/3 · %s", minInt(w.Step+1, 3), labels[i]), fmt.Sprintf("Arbeitsbereich %d° · Werte %d / %d / %d", w.RangeDegrees, w.SteerLeft, w.SteerCenter, w.SteerRight)
	case "pedals":
		steps := []string{"Ruheposition", "Gas voll", "Bremse voll", "Kupplung voll", "Fertig"}
		if system.IsDFGTModel(s.WheelModel) {
			steps = []string{"Ruheposition", "Gas voll", "Bremse voll", "Fertig"}
		}
		i := w.Step
		if i >= len(steps) {
			i = len(steps) - 1
		}
		return fmt.Sprintf("Schritt %d/%d · %s", minInt(w.Step+1, len(steps)-1), len(steps)-1, steps[i]), fmt.Sprintf("Deadzone %.0f %% · Kurve %s", w.Deadzone*100, w.Curve)
	case "button":
		if w.Step == 0 {
			return "Schritt 1/2 · Ausgangslage", "Ziel: " + w.ButtonTarget
		}
		return "Schritt 2/2 · Taste gedrückt halten", "Ziel: " + w.ButtonTarget
	case "shifter":
		labels := []string{"Neutral", "Gang 1", "Gang 2", "Gang 3", "Gang 4", "Gang 5", "Gang 6", "Rückwärts"}
		i := w.Step
		if i >= len(labels) {
			i = len(labels) - 1
		}
		return fmt.Sprintf("Schritt %d/8 · %s", minInt(w.Step+1, 8), labels[i]), fmt.Sprintf("%d von 8 eindeutigen Signaturen erfasst", len(w.GearSignatures))
	}
	return "", ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func paintWheelCalibrationWizard(hdc uintptr, content RECT, s system.State) {
	resetCalibrationWizardRects()
	left, right := content.Left+20, content.Right-20
	virtualHeight := int32(440)
	top, visibleHeight, saved := beginWheelReworkScroll(hdc, content, virtualHeight)
	card := RECT{left, top, right, top + 430}
	modernCard(hdc, card, colAccent)
	cardHeader(hdc, card, "◎", calibrationWizardTitle(), "Inline-Assistent · keine Werte werden vor Abschluss gespeichert", colAccent)
	step, detail := calibrationWizardStepText(s)
	rows := []infoRow{{"Aktueller Schritt", step, colAccent}, {"Konfiguration", detail, colGood}, {"Live Input", calibrationLiveSummary(system.ReadPreferredWheelInput(s)), colMuted}}
	wheelCalibrationWizardLiveRect = RECT{card.Left + 18, card.Top + 68, card.Right - 18, card.Top + 188}
	paintInfoRows(hdc, wheelCalibrationWizardLiveRect, rows)

	msg := wheelCalibrationWizard.Status
	accent := colGood
	if strings.TrimSpace(wheelCalibrationWizard.Error) != "" {
		msg = wheelCalibrationWizard.Error
		accent = colBad
	}
	box := RECT{card.Left + 18, card.Top + 198, card.Right - 18, card.Top + 275}
	drawRoundRect(hdc, box, 12, colPanel2, accent)
	tr := RECT{box.Left + 14, box.Top + 9, box.Right - 14, box.Bottom - 9}
	drawFittedParagraph(hdc, msg, tr, readableTextColor(accent, colPanel2), fontBody, fontSmall)

	// Secondary configuration actions are inline and cycle deterministic choices.
	if wheelCalibrationWizard.Kind == "steering" {
		wheelCalibrationWizardRects[1] = RECT{card.Left + 18, card.Top + 292, card.Left + 220, card.Top + 329}
		wheelPanelButton(hdc, wheelCalibrationWizardRects[1], fmt.Sprintf("Bereich: %d°", wheelCalibrationWizard.RangeDegrees), colAccent, wheelCalibrationWizardHover == 1)
	}
	if wheelCalibrationWizard.Kind == "pedals" {
		wheelCalibrationWizardRects[1] = RECT{card.Left + 18, card.Top + 292, card.Left + 220, card.Top + 329}
		wheelCalibrationWizardRects[2] = RECT{card.Left + 232, card.Top + 292, card.Left + 434, card.Top + 329}
		wheelPanelButton(hdc, wheelCalibrationWizardRects[1], fmt.Sprintf("Deadzone: %.0f %%", wheelCalibrationWizard.Deadzone*100), colAccent, wheelCalibrationWizardHover == 1)
		wheelPanelButton(hdc, wheelCalibrationWizardRects[2], "Kurve: "+wheelCalibrationWizard.Curve, rgb(174, 112, 255), wheelCalibrationWizardHover == 2)
	}
	if wheelCalibrationWizard.Kind == "button" && wheelCalibrationWizard.Step == 0 {
		wheelCalibrationWizardRects[1] = RECT{card.Left + 18, card.Top + 292, card.Left + 300, card.Top + 329}
		wheelPanelButton(hdc, wheelCalibrationWizardRects[1], "Ziel: "+wheelCalibrationWizard.ButtonTarget, colAccent, wheelCalibrationWizardHover == 1)
	}

	cancel := RECT{card.Left + 18, card.Bottom - 50, card.Left + 190, card.Bottom - 12}
	primary := RECT{card.Right - 260, card.Bottom - 50, card.Right - 18, card.Bottom - 12}
	wheelCalibrationWizardRects[0] = cancel
	wheelCalibrationWizardRects[3] = primary
	wheelPanelButton(hdc, cancel, "Abbrechen", colWarning, wheelCalibrationWizardHover == 0)
	wheelPanelButton(hdc, primary, calibrationPrimaryLabel(s), colGood, wheelCalibrationWizardHover == 3)
	endWheelReworkScroll(hdc, content, visibleHeight, virtualHeight, saved)
}

func calibrationLiveSummary(j system.JoyState) string {
	if ok, why := learningSampleOK(j); !ok {
		return "wartet · " + strings.ReplaceAll(why, "\r\n", " ")
	}
	return fmt.Sprintf("gültig · %s · %s · X=%d Y=%d Z=%d R=%d U=%d V=%d", emptyFallback(j.InputSource, "Input"), emptyFallback(j.LayoutID, "Layout"), j.X, j.Y, j.Z, j.R, j.U, j.V)
}

func calibrationPrimaryLabel(s system.State) string {
	w := wheelCalibrationWizard
	switch w.Kind {
	case "steering":
		return []string{"Links erfassen", "Mitte erfassen", "Rechts erfassen", "Gespeichert"}[minInt(w.Step, 3)]
	case "pedals":
		if w.Step == 0 {
			return "Ruheposition erfassen"
		}
		if w.Step == 1 {
			return "Gas erfassen"
		}
		if w.Step == 2 {
			return "Bremse erfassen"
		}
		if !system.IsDFGTModel(s.WheelModel) && w.Step == 3 {
			return "Kupplung erfassen"
		}
		return "Gespeichert"
	case "button":
		if w.Step == 0 {
			return "Ausgangslage erfassen"
		}
		return "Gedrückte Taste erfassen"
	case "shifter":
		return "Zustand erfassen"
	}
	return "Weiter"
}

func calibrationWizardHitTest(x, y int32) int {
	if !wheelCalibrationWizard.Active || currentPage != pageWheel || wheelSubtab != wheelSubtabCalibration {
		return -1
	}
	for i, r := range wheelCalibrationWizardRects {
		if pointIn(r, x, y) {
			return i
		}
	}
	return -1
}

func calibrationWizardOnMouseMove(x, y int32) bool {
	old := wheelCalibrationWizardHover
	wheelCalibrationWizardHover = calibrationWizardHitTest(x, y)
	return old != wheelCalibrationWizardHover
}

func calibrationWizardHandleClick(x, y int32, s system.State) bool {
	i := calibrationWizardHitTest(x, y)
	if i < 0 {
		return false
	}
	if i == 0 {
		closeInlineCalibration()
		return true
	}
	if i == 1 {
		switch wheelCalibrationWizard.Kind {
		case "steering":
			wheelCalibrationWizard.RangeDegrees = nextIntChoice(calibrationRanges, wheelCalibrationWizard.RangeDegrees)
		case "pedals":
			wheelCalibrationWizard.Deadzone = nextFloatChoice(calibrationDeadzones, wheelCalibrationWizard.Deadzone)
		case "button":
			wheelCalibrationWizard.ButtonTarget = nextStringChoice(calibrationButtonTargets, wheelCalibrationWizard.ButtonTarget)
		}
		wheelCalibrationWizard.Error = ""
		invalidate(mainWnd)
		return true
	}
	if i == 2 && wheelCalibrationWizard.Kind == "pedals" {
		wheelCalibrationWizard.Curve = nextStringChoice(calibrationCurves, wheelCalibrationWizard.Curve)
		wheelCalibrationWizard.Error = ""
		invalidate(mainWnd)
		return true
	}
	if i == 3 {
		captureInlineCalibration(s)
		invalidate(mainWnd)
		return true
	}
	return true
}

func nextIntChoice(vals []int, current int) int {
	for i, v := range vals {
		if v == current {
			return vals[(i+1)%len(vals)]
		}
	}
	return vals[0]
}
func nextFloatChoice(vals []float64, current float64) float64 {
	for i, v := range vals {
		if v == current {
			return vals[(i+1)%len(vals)]
		}
	}
	return vals[0]
}
func nextStringChoice(vals []string, current string) string {
	for i, v := range vals {
		if v == current {
			return vals[(i+1)%len(vals)]
		}
	}
	return vals[0]
}

func inlineCalibrationSample(s system.State) (system.JoyState, bool) {
	j := system.ReadPreferredWheelInput(s)
	if ok, why := learningSampleOK(j); !ok {
		wheelCalibrationWizard.Error = why
		return j, false
	}
	return j, true
}

func captureInlineCalibration(s system.State) {
	wheelCalibrationWizard.Error = ""
	w := &wheelCalibrationWizard
	switch w.Kind {
	case "steering":
		j, ok := inlineCalibrationSample(s)
		if !ok {
			return
		}
		switch w.Step {
		case 0:
			w.SteerLeft = j.X
			w.Step = 1
			w.Status = "Lenkrad exakt in die Mitte stellen und erfassen."
		case 1:
			w.SteerCenter = j.X
			w.Step = 2
			w.Status = "Lenkrad ganz nach rechts stellen und erfassen."
		case 2:
			w.SteerRight = j.X
			c := system.SteeringCalibration{Left: w.SteerLeft, Center: w.SteerCenter, Right: w.SteerRight, RangeDegrees: w.RangeDegrees}
			if err := system.SaveSteeringCalibration(s.DataDir, s.SelectedWheelID, c); err != nil {
				w.Error = err.Error()
				return
			}
			w.Step = 3
			w.Status = fmt.Sprintf("Gespeichert: %d / %d / %d · %d°", w.SteerLeft, w.SteerCenter, w.SteerRight, w.RangeDegrees)
			setActionFeedback("Lenkung wurde gespeichert.")
			refreshFastAsync()
		}
	case "pedals":
		captureInlinePedal(s)
	case "button":
		j, ok := inlineCalibrationSample(s)
		if !ok {
			return
		}
		if w.Step == 0 {
			w.ButtonBefore = j
			w.Step = 1
			w.Status = "Jetzt nur „" + w.ButtonTarget + "“ gedrückt halten und erneut erfassen."
			return
		}
		bit, ok := system.FirstPressedButton(w.ButtonBefore, j)
		if !ok {
			w.Error = "Es wurde nicht genau eine neu gedrückte Taste erkannt. Ausgangslage bleibt erhalten; nur eine Taste drücken."
			return
		}
		if err := system.SaveButtonMapping(s.DataDir, s.SelectedWheelID, w.ButtonTarget, bit); err != nil {
			w.Error = err.Error()
			return
		}
		w.Status = fmt.Sprintf("%s wurde auf Button %d gespeichert.", w.ButtonTarget, bit+1)
		setActionFeedback(w.Status)
		refreshFastAsync()
		closeInlineCalibration()
	case "shifter":
		captureInlineShifter(s)
	}
}

func captureInlinePedal(s system.State) {
	w := &wheelCalibrationWizard
	j, ok := inlineCalibrationSample(s)
	if !ok {
		return
	}
	if w.Step == 0 {
		w.PedalBase = j
		w.PedalExcluded = map[string]bool{}
		w.Step = 1
		w.Status = "Gas vollständig durchtreten, gedrückt halten und erfassen."
		return
	}
	base := w.PedalBase
	if j.InputSource != base.InputSource || j.LayoutID != base.LayoutID || j.WheelID != base.WheelID || j.SessionID != base.SessionID {
		w.Error = "Die Eingabequelle hat sich während der Kalibrierung geändert. Nichts wurde gespeichert."
		return
	}
	axis := system.StrongestMovedAxis(base, j, w.PedalExcluded)
	if axis == "" {
		w.Error = "Keine eindeutige freie Achse erkannt. Das aktuelle Pedal über den gesamten Weg bewegen und gedrückt halten."
		return
	}
	rest, rok := system.AxisRawValue(base, axis)
	pressed, pok := system.AxisRawValue(j, axis)
	if !rok || !pok || rest == pressed {
		w.Error = "Die erkannte Achse liefert keinen nutzbaren Ruhe-/Vollanschlag."
		return
	}
	w.PedalExcluded[axis] = true
	c := calibrationPedalCapture{Axis: axis, Rest: rest, Pressed: pressed}
	switch w.Step {
	case 1:
		w.PedalGas = c
		w.Step = 2
		w.Status = "Gas loslassen, Bremse vollständig durchtreten und erfassen."
	case 2:
		w.PedalBrake = c
		if system.IsDFGTModel(s.WheelModel) {
			saveInlinePedals(s)
			return
		}
		w.Step = 3
		w.Status = "Bremse loslassen, Kupplung vollständig durchtreten und erfassen."
	case 3:
		w.PedalClutch = c
		saveInlinePedals(s)
	}
}

func saveInlinePedals(s system.State) {
	w := &wheelCalibrationWizard
	m := system.PedalMapping{
		InputSource:      w.PedalBase.InputSource,
		LayoutID:         w.PedalBase.LayoutID,
		Gas:              w.PedalGas.Axis,
		Brake:            w.PedalBrake.Axis,
		GasCalibration:   axisCal(w.PedalGas.Rest, w.PedalGas.Pressed, w.Curve, w.Deadzone),
		BrakeCalibration: axisCal(w.PedalBrake.Rest, w.PedalBrake.Pressed, w.Curve, w.Deadzone),
	}
	if w.PedalClutch.Axis != "" {
		m.Clutch = w.PedalClutch.Axis
		m.ClutchCalibration = axisCal(w.PedalClutch.Rest, w.PedalClutch.Pressed, w.Curve, w.Deadzone)
	}
	if err := system.SavePedalMappingForWheel(s.DataDir, s.SelectedWheelID, s.WheelModel, s.ActiveMode, m); err != nil {
		w.Error = err.Error()
		return
	}
	w.Step = 4
	if system.IsDFGTModel(s.WheelModel) {
		w.Step = 3
	}
	w.Status = fmt.Sprintf("Gespeichert: Gas %s · Bremse %s", w.PedalGas.Axis, w.PedalBrake.Axis)
	if w.PedalClutch.Axis != "" {
		w.Status += " · Kupplung " + w.PedalClutch.Axis
	}
	setActionFeedback("Pedalkalibrierung wurde gespeichert.")
	refreshFastAsync()
}

func captureInlineShifter(s system.State) {
	w := &wheelCalibrationWizard
	if strings.TrimSpace(w.Error) != "" && (!system.IsG25Model(s.WheelModel) && !system.IsG27Model(s.WheelModel)) {
		return
	}
	j, ok := inlineCalibrationSample(s)
	if !ok {
		return
	}
	sig := system.GearSignature(j)
	if sig == "" {
		w.Error = "Für diesen Zustand konnte keine stabile Shifter-Signatur gebildet werden."
		return
	}
	if old, exists := w.GearSignatures[sig]; exists {
		w.Error = fmt.Sprintf("Diese Signatur wurde bereits für %s erfasst.", gearName(old))
		return
	}
	gears := []int{0, 1, 2, 3, 4, 5, 6, -1}
	gear := gears[minInt(w.Step, len(gears)-1)]
	w.GearSignatures[sig] = gear
	w.Step++
	if w.Step >= len(gears) {
		if err := system.SaveGearMapping(s.DataDir, s.SelectedWheelID, w.GearSignatures); err != nil {
			w.Error = err.Error()
			w.Step--
			return
		}
		w.Status = "Neutral, 1–6 und Rückwärts wurden als acht eindeutige Signaturen gespeichert."
		setActionFeedback("H-Shifter-Mapping wurde gespeichert.")
		refreshFastAsync()
		closeInlineCalibration()
		return
	}
	labels := []string{"Neutral", "Gang 1", "Gang 2", "Gang 3", "Gang 4", "Gang 5", "Gang 6", "Rückwärts"}
	w.Status = labels[w.Step] + " einlegen und Zustand erfassen."
}
