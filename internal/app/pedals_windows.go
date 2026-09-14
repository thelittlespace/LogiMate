//go:build windows

package app

import (
	"fmt"

	"github.com/thelittlespace/LogiMate/internal/system"
)

func learnPedals(s system.State) {
	if !system.HasActionableSelectedWheel(s) {
		if system.IsCompatibilityModel(s.WheelModel) {
			messageBox(mainWnd, "Das C294-Lenkrad ist noch nicht eindeutig als G25, G27 oder Driving Force GT bestätigt. Bitte zuerst das Modell bestätigen.", "Pedale kalibrieren", MB_OK|MB_ICONWARNING)
		} else {
			messageBox(mainWnd, "Kein eindeutig ausgewähltes unterstütztes Logitech-Lenkrad erkannt.", "Pedale kalibrieren", MB_OK|MB_ICONWARNING)
		}
		return
	}

	messageBox(mainWnd, "Alle Pedale vollständig loslassen und anschließend OK wählen. LogiMate speichert diese Werte als Ruheposition.", "Pedale kalibrieren · Ruheposition", MB_OK|MB_ICONINFORMATION)
	base := system.ReadPreferredWheelInput(s)
	if !base.Found || !base.SampleValid {
		messageBox(mainWnd, "Es liegt noch kein gültiger Eingangssample vor. Bitte das Lenkrad/Pedale kurz bewegen und erneut versuchen.\r\n\r\n"+base.Error, "Pedale kalibrieren", MB_OK|MB_ICONERROR)
		return
	}

	excluded := map[string]bool{}
	type captured struct {
		axis          string
		rest, pressed uint32
	}
	capture := func(label string) (captured, bool) {
		messageBox(mainWnd, label+" vollständig durchtreten, gedrückt halten und dann OK wählen.", "Pedale kalibrieren · "+label, MB_OK|MB_ICONINFORMATION)
		now := system.ReadPreferredWheelInput(s)
		if !now.Found || !now.SampleValid {
			messageBox(mainWnd, "Kein gültiger Eingangssample. "+now.Error, "Pedale kalibrieren", MB_OK|MB_ICONERROR)
			return captured{}, false
		}
		if now.InputSource != base.InputSource || now.LayoutID != base.LayoutID || now.WheelID != base.WheelID || now.SessionID != base.SessionID {
			messageBox(mainWnd, "Die Eingabequelle hat sich während der Kalibrierung geändert. Aus Sicherheitsgründen wird nichts gespeichert.", "Pedale kalibrieren", MB_OK|MB_ICONWARNING)
			return captured{}, false
		}
		axis := system.StrongestMovedAxis(base, now, excluded)
		if axis == "" {
			messageBox(mainWnd, "Keine eindeutige freie Achse erkannt. Bitte das Pedal über den gesamten Weg bewegen und erneut versuchen.", "Pedale kalibrieren", MB_OK|MB_ICONWARNING)
			return captured{}, false
		}
		excluded[axis] = true
		rest, rok := system.AxisRawValue(base, axis)
		pressed, pok := system.AxisRawValue(now, axis)
		if !rok || !pok || rest == pressed {
			messageBox(mainWnd, "Die Achse liefert keinen nutzbaren Minimum/Maximum-Bereich.", "Pedale kalibrieren", MB_OK|MB_ICONWARNING)
			return captured{}, false
		}
		return captured{axis: axis, rest: rest, pressed: pressed}, true
	}

	gas, ok := capture("Gas")
	if !ok {
		return
	}
	brake, ok := capture("Bremse")
	if !ok {
		return
	}
	clutch := captured{}
	if !system.IsDFGTModel(s.WheelModel) {
		clutch, ok = capture("Kupplung")
		if !ok {
			return
		}
	}

	deadzone := pedalDeadzoneChoice()
	curve := pedalCurveChoice()
	m := system.PedalMapping{
		InputSource:      base.InputSource,
		LayoutID:         base.LayoutID,
		Gas:              gas.axis,
		Brake:            brake.axis,
		GasCalibration:   axisCal(gas.rest, gas.pressed, curve, deadzone),
		BrakeCalibration: axisCal(brake.rest, brake.pressed, curve, deadzone),
	}
	if clutch.axis != "" {
		m.Clutch = clutch.axis
		m.ClutchCalibration = axisCal(clutch.rest, clutch.pressed, curve, deadzone)
	}

	if err := system.SavePedalMappingForWheel(s.DataDir, s.SelectedWheelID, s.WheelModel, s.ActiveMode, m); err != nil {
		messageBox(mainWnd, err.Error(), "Pedale kalibrieren", MB_OK|MB_ICONERROR)
		return
	}

	summary := fmt.Sprintf("Pedale gespeichert:\r\nGas = %s (%d → %d)\r\nBremse = %s (%d → %d)", gas.axis, gas.rest, gas.pressed, brake.axis, brake.rest, brake.pressed)
	if clutch.axis != "" {
		summary += fmt.Sprintf("\r\nKupplung = %s (%d → %d)", clutch.axis, clutch.rest, clutch.pressed)
	}
	summary += fmt.Sprintf("\r\n\r\nDeadzone: %.0f %%\r\nKurve: %s\r\nInvertierung wird pro Achse automatisch aus Ruhe/Vollanschlag bestimmt.", deadzone*100, curve)
	messageBox(mainWnd, summary, "Pedale kalibriert", MB_OK|MB_ICONINFORMATION)
	refreshAsync()
}
