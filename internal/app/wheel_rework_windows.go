//go:build windows

package app

import (
	"fmt"
	"strings"

	"github.com/thelittlespace/LogiMate/internal/system"
)

// Build 002-004: stable in-page wheel navigation. These actions deliberately
// call the already-audited learning/profile/device backends instead of creating
// a second persistence or HID path.
var (
	wheelCalibrationActionRects [4]RECT
	wheelProfileActionRects     [4]RECT
	wheelDeviceActionRects      [4]RECT
	wheelReworkHoverKind        string
	wheelReworkHoverIndex       = -1
)

func clearWheelReworkRects() {
	for i := range wheelCalibrationActionRects {
		wheelCalibrationActionRects[i] = RECT{}
	}
	for i := range wheelProfileActionRects {
		wheelProfileActionRects[i] = RECT{}
	}
	for i := range wheelDeviceActionRects {
		wheelDeviceActionRects[i] = RECT{}
	}
}

func beginWheelReworkScroll(hdc uintptr, content RECT, virtualHeight int32) (top, visibleHeight int32, saved uintptr) {
	visibleTop := content.Top + 98
	visibleBottom := content.Bottom - 16
	visibleHeight = visibleBottom - visibleTop
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	contentScrollMax = virtualHeight - visibleHeight
	if contentScrollMax < 0 {
		contentScrollMax = 0
	}
	if contentScroll > contentScrollMax {
		contentScroll = contentScrollMax
	}
	if contentScroll < 0 {
		contentScroll = 0
	}
	top = content.Top + 101 - contentScroll
	saved, _, _ = pSaveDC.Call(hdc)
	pIntersectClipRect.Call(hdc, uintptr(content.Left+16), uintptr(visibleTop), uintptr(content.Right-14), uintptr(visibleBottom))
	return top, visibleHeight, saved
}

func endWheelReworkScroll(hdc uintptr, content RECT, visibleHeight, virtualHeight int32, saved uintptr) {
	if saved != 0 {
		pRestoreDC.Call(hdc, saved)
	}
	paintScrollBar(hdc, content, visibleHeight, virtualHeight)
}

func wheelPanelButton(hdc uintptr, r RECT, label string, accent uintptr, hovered bool) {
	fill := colPanel2
	border := colBorder
	text := colText
	if hovered {
		fill = colHover
		border = accent
	}
	drawRoundRect(hdc, r, 11, fill, border)
	pSetTextColor.Call(hdc, text)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
	drawText(hdc, label, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}

func calibrationStatus(s system.State) (steer, pedals, buttons, shifter string) {
	p := system.ReadControlProfile(s.DataDir, s.SelectedWheelID)
	steer = "Noch offen"
	if p.Steering.RangeDegrees > 0 && p.Steering.Left != p.Steering.Right {
		steer = fmt.Sprintf("%d° · gespeichert", p.Steering.RangeDegrees)
	}
	pedals = "Noch offen"
	m := s.Pedals
	if m.Gas != "" || m.Brake != "" || m.Clutch != "" {
		mapped := 0
		for _, x := range []string{m.Gas, m.Brake, m.Clutch} {
			if strings.TrimSpace(x) != "" {
				mapped++
			}
		}
		pedals = fmt.Sprintf("%d Achse(n) zugeordnet", mapped)
	}
	buttons = fmt.Sprintf("%d Mapping(s)", len(p.Buttons))
	if len(p.Buttons) == 0 {
		buttons = "Noch offen"
	}
	shifter = fmt.Sprintf("%d Signatur(en)", len(p.Gears))
	if len(p.Gears) == 0 {
		shifter = "Noch offen"
	}
	if system.IsDFGTModel(s.WheelModel) {
		shifter = "Nicht vorhanden (DFGT)"
	}
	return
}

func paintWheelCalibrationPanel(hdc uintptr, content RECT, s system.State) {
	d6ResetRects()
	clearWheelReworkRects()
	if wheelCalibrationWizard.Active {
		paintWheelCalibrationWizard(hdc, content, s)
		return
	}
	left, right := content.Left+20, content.Right-20
	gap := int32(12)
	h := int32(204)
	virtualHeight := h*2 + gap
	top, visibleHeight, saved := beginWheelReworkScroll(hdc, content, virtualHeight)
	w := (right - left - gap) / 2
	steer, pedals, buttons, shifter := calibrationStatus(s)
	cards := []struct {
		title, sub, status, detail, action, icon string
		accent                                   uintptr
	}{
		{"Lenkung", "Links · Mitte · Rechts · Arbeitsbereich", steer, "Die Kalibrierung wird an die physische Wheel-ID gebunden und nur aus gültigen Samples gespeichert.", "Lenkung kalibrieren", "↔", colAccent},
		{"Pedale", "Gas · Bremse · Kupplung · Deadzone", pedals, "Achse, Minimum/Maximum, Invertierung und Kurve werden gemeinsam als Layout gespeichert.", "Pedale kalibrieren", "▥", colGood},
		{"Tasten & Wippen", "Logische Namen statt wechselnder Button-Indizes", buttons, "Paddles, Wheel- und Shiftertasten lassen sich gezielt neu lernen, ohne andere Kalibrierungen zu ändern.", "Taste lernen", "●", rgb(174, 112, 255)},
		{"H-Shifter", "Neutral · 1–6 · Rückwärts", shifter, "Für G25/G27 werden acht eindeutige Shifter-Signaturen geprüft; überlappende Zustände werden abgelehnt.", "H-Shifter lernen", "H", rgb(255, 196, 96)},
	}
	for i, c := range cards {
		row, col := int32(i/2), int32(i%2)
		r := RECT{left + col*(w+gap), top + row*(h+gap), left + col*(w+gap) + w, top + row*(h+gap) + h}
		modernCard(hdc, r, c.accent)
		cardHeader(hdc, r, c.icon, c.title, c.sub, c.accent)
		pSetTextColor.Call(hdc, colText)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontBrand))
		sr := RECT{r.Left + 18, r.Top + 65, r.Right - 18, r.Top + 95}
		drawText(hdc, c.status, &sr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		dr := RECT{r.Left + 18, r.Top + 98, r.Right - 18, r.Bottom - 53}
		drawFittedParagraph(hdc, c.detail, dr, colMuted, fontSmall, fontSmall)
		br := RECT{r.Left + 18, r.Bottom - 45, r.Right - 18, r.Bottom - 12}
		wheelCalibrationActionRects[i] = br
		wheelPanelButton(hdc, br, c.action, c.accent, (wheelReworkHoverKind == "cal" && wheelReworkHoverIndex == i) || keyboardFocus == focusWheelCalibrationActionBase+i)
	}
	endWheelReworkScroll(hdc, content, visibleHeight, virtualHeight, saved)
}

func safePercent(v int) int {
	if v <= 0 {
		return 100
	}
	return v
}
func multPercent(vals ...int) int {
	v := 100.0
	for _, x := range vals {
		// A zero in an already-normalized gain is a real mute value (not a
		// missing/default marker). In particular D4 Advanced FFB schema v2
		// deliberately allows ConstantGainPercent=0. Effective Settings must
		// therefore show 0 %, not silently turn it back into 100 %.
		if x < 0 {
			x = 0
		}
		v *= float64(x) / 100.0
	}
	if v < 0 {
		v = 0
	}
	if v > 999 {
		v = 999
	}
	return int(v + 0.5)
}

func paintWheelProfilesPanel(hdc uintptr, content RECT, s system.State) {
	d6ResetRects()
	clearWheelReworkRects()
	left, right := content.Left+20, content.Right-20
	gap := int32(12)
	virtualHeight := int32(407)
	top, visibleHeight, saved := beginWheelReworkScroll(hdc, content, virtualHeight)
	wp := system.ReadActiveNativeEngineProfile(s.DataDir, s.SelectedWheelID)
	adv, _ := system.ReadSelectedAdvancedFFBConfig(s)
	safety := system.ReadNativeFFBConfig(s.DataDir, s.SelectedWheelID)
	gp, exe, gok := system.ActiveGameProfile(s.DataDir)
	gameName := "Kein aktives Spielprofil"
	gameMaster, gameConstant := 100, 100
	if gok {
		gameName = gp.Name + " · " + exe
		gameMaster = safePercent(gp.MasterGainPercent)
		gameConstant = safePercent(gp.ConstantGainPercent)
	}
	effMaster := multPercent(wp.MasterGainPercent, gameMaster)
	effConstant := multPercent(wp.MasterGainPercent, wp.ConstantGainPercent, gameMaster, gameConstant, adv.ConstantGainPercent)
	applied := effConstant
	if safety.ConstantLimit > 0 && applied > safety.ConstantLimit {
		applied = safety.ConstantLimit
	}

	upperH := int32(175)
	half := (right - left - gap) / 2
	wheel := RECT{left, top, left + half, top + upperH}
	game := RECT{wheel.Right + gap, top, right, top + upperH}
	modernCard(hdc, wheel, colAccent)
	cardHeader(hdc, wheel, "◉", "Wheel-Profil", wp.Name+" · pro physischem Wheel", colAccent)
	paintInfoRows(hdc, RECT{wheel.Left + 16, wheel.Top + 65, wheel.Right - 16, wheel.Bottom - 52}, []infoRow{{"Master", fmt.Sprintf("%d %%", wp.MasterGainPercent), colAccent}, {"Lenkwinkel", fmt.Sprintf("%d°", wp.RotationDegrees), colGood}, {"Constant", fmt.Sprintf("%d %%", wp.ConstantGainPercent), colAccent}})
	br := RECT{wheel.Left + 18, wheel.Bottom - 43, wheel.Right - 18, wheel.Bottom - 11}
	wheelProfileActionRects[0] = br
	wheelPanelButton(hdc, br, "Wheel-Profile verwalten", colAccent, (wheelReworkHoverKind == "profile" && wheelReworkHoverIndex == 0) || keyboardFocus == focusWheelProfileActionBase+0)
	modernCard(hdc, game, colGood)
	cardHeader(hdc, game, "▶", "Spielprofil", gameName+" · Auto-Apply opt-in", colGood)
	gdetail := "Kein passendes Vordergrundspiel aktiv. Es gelten die Wheel-Werte als Basis."
	if gok {
		gdetail = fmt.Sprintf("Master %d %% · Constant %d %% · Engine %s", gameMaster, gameConstant, emptyFallback(gp.EngineProfile, "Wheel-Profil"))
	}
	paintCardParagraph(hdc, game, gdetail, 70)
	br = RECT{game.Left + 18, game.Bottom - 43, game.Right - 18, game.Bottom - 11}
	wheelProfileActionRects[1] = br
	wheelPanelButton(hdc, br, "Game-Profile verwalten", colGood, (wheelReworkHoverKind == "profile" && wheelReworkHoverIndex == 1) || keyboardFocus == focusWheelProfileActionBase+1)

	y := wheel.Bottom + gap
	effective := RECT{left, y, right, y + 220}
	modernCard(hdc, effective, rgb(174, 112, 255))
	cardHeader(hdc, effective, "=", "Effektive Einstellungen", "Welche Ebene gewinnt am Ende wirklich?", rgb(174, 112, 255))
	rows := []infoRow{
		{"Wheel Master × Game Master", fmt.Sprintf("%d %% × %d %% → %d %%", wp.MasterGainPercent, gameMaster, effMaster), colAccent},
		{"Constant-Kette", fmt.Sprintf("Wheel %d %% × Game %d %% × Pipeline %d %% → %d %%", wp.ConstantGainPercent, gameConstant, adv.ConstantGainPercent, effConstant), colGood},
		{"Hard Safety Cap", fmt.Sprintf("%d %% → tatsächlich maximal %d %%", safety.ConstantLimit, applied), colWarning},
		{"Signalformung", fmt.Sprintf("%s · Deadband %d %% · MinForce %d %% · Smooth %d %%", emptyFallback(adv.Preset, "Custom"), adv.DeadbandPercent, adv.MinimumForcePercent, adv.SmoothingPercent), rgb(174, 112, 255)},
	}
	paintInfoRows(hdc, RECT{effective.Left + 18, effective.Top + 67, effective.Right - 18, effective.Bottom - 55}, rows)
	bw := (effective.Right - effective.Left - 54) / 2
	b1 := RECT{effective.Left + 18, effective.Bottom - 43, effective.Left + 18 + bw, effective.Bottom - 11}
	b2 := RECT{b1.Right + 18, effective.Bottom - 43, effective.Right - 18, effective.Bottom - 11}
	wheelProfileActionRects[2] = b1
	wheelProfileActionRects[3] = b2
	wheelPanelButton(hdc, b1, "Force Feedback öffnen", colAccent, (wheelReworkHoverKind == "profile" && wheelReworkHoverIndex == 2) || keyboardFocus == focusWheelProfileActionBase+2)
	wheelPanelButton(hdc, b2, "Profile als JSON exportieren", colGood, (wheelReworkHoverKind == "profile" && wheelReworkHoverIndex == 3) || keyboardFocus == focusWheelProfileActionBase+3)
	endWheelReworkScroll(hdc, content, visibleHeight, virtualHeight, saved)
}

func paintWheelDevicePanel(hdc uintptr, content RECT, s system.State) {
	d6ResetRects()
	clearWheelReworkRects()
	left, right := content.Left+20, content.Right-20
	gap := int32(12)
	virtualHeight := int32(505)
	top, visibleHeight, saved := beginWheelReworkScroll(hdc, content, virtualHeight)
	j := getLiveJoy()
	selected := "Kein Ziel"
	mode := s.ActiveMode
	identity := s.DetectionEvidence
	if w, ok := system.SelectedWheel(s); ok {
		selected = system.WheelDeviceLabel(w)
		mode = w.Mode
		if strings.TrimSpace(identity) == "" {
			identity = w.Evidence
		}
	}
	main := RECT{left, top, right, top + 205}
	modernCard(hdc, main, colAccent)
	cardHeader(hdc, main, "▣", "Geräteidentität", selected+" · frische Windows-/HID-Evidenz", colAccent)
	paintInfoRows(hdc, RECT{main.Left + 18, main.Top + 65, main.Right - 18, main.Bottom - 14}, []infoRow{
		{"Modus", emptyFallback(mode, "—"), colAccent}, {"Wheel ID", emptyFallback(s.SelectedWheelID, "—"), colGood}, {"Evidenz", emptyFallback(identity, "Noch nicht eindeutig"), colWarning}, {"Input", emptyFallback(j.InputSource, emptyFallback(j.Selection, "—")), rgb(174, 112, 255)},
	})
	y := main.Bottom + gap
	half := (right - left - gap) / 2
	hid := RECT{left, y, left + half, y + 185}
	raw := RECT{hid.Right + gap, y, right, y + 185}
	modernCard(hdc, hid, colGood)
	cardHeader(hdc, hid, "H", "HID & Reports", fmt.Sprintf("%d Kandidat(en)", len(s.HIDCandidates)), colGood)
	detail := fmt.Sprintf("Input Report: %d Byte\nOutput Report: %d Byte\nSample: %s · %.1f Hz", len(j.RawReport), func() uint16 {
		if len(s.HIDCandidates) > 0 {
			return s.HIDCandidates[0].OutputReportLength
		}
		return 0
	}(), map[bool]string{true: "gültig", false: "wartet"}[j.SampleValid], j.ReportRateHz)
	paintCardParagraph(hdc, hid, detail, 70)
	modernCard(hdc, raw, rgb(174, 112, 255))
	cardHeader(hdc, raw, "≡", "Rohdaten", emptyFallback(j.LayoutID, "Layout unbekannt"), rgb(174, 112, 255))
	paintCardParagraph(hdc, raw, system.RawReportPretty(j), 70)
	labels := []string{"Geräte verwalten", "Rohreport kopieren", "Erweiterte Live-Ansicht", "Windows Controller"}
	for i, label := range labels {
		row, col := int32(i/2), int32(i%2)
		bw := (right - left - gap) / 2
		r := RECT{left + col*(bw+gap), y + 198 + row*46, left + col*(bw+gap) + bw, y + 235 + row*46}
		wheelDeviceActionRects[i] = r
		wheelPanelButton(hdc, r, label, colAccent, (wheelReworkHoverKind == "device" && wheelReworkHoverIndex == i) || keyboardFocus == focusWheelDeviceActionBase+i)
	}
	endWheelReworkScroll(hdc, content, visibleHeight, virtualHeight, saved)
}

func wheelReworkHitTest(x, y int32) (string, int) {
	if currentPage != pageWheel {
		return "", -1
	}
	var rects []RECT
	kind := ""
	switch wheelSubtab {
	case wheelSubtabCalibration:
		rects = wheelCalibrationActionRects[:]
		kind = "cal"
	case wheelSubtabProfiles:
		rects = wheelProfileActionRects[:]
		kind = "profile"
	case wheelSubtabDevice:
		rects = wheelDeviceActionRects[:]
		kind = "device"
	}
	for i, r := range rects {
		if pointIn(r, x, y) {
			return kind, i
		}
	}
	return "", -1
}

func wheelReworkOnMouseMove(x, y int32) bool {
	if wheelCalibrationWizard.Active {
		return calibrationWizardOnMouseMove(x, y)
	}
	ok, oi := wheelReworkHoverKind, wheelReworkHoverIndex
	wheelReworkHoverKind, wheelReworkHoverIndex = wheelReworkHitTest(x, y)
	return ok != wheelReworkHoverKind || oi != wheelReworkHoverIndex
}

func wheelReworkActivateFocus(s system.State, focusID int) bool {
	var r RECT
	switch {
	case focusID >= focusWheelCalibrationActionBase && focusID < focusWheelCalibrationActionBase+len(wheelCalibrationActionRects):
		if currentPage != pageWheel || wheelSubtab != wheelSubtabCalibration || wheelCalibrationWizard.Active {
			return false
		}
		r = wheelCalibrationActionRects[focusID-focusWheelCalibrationActionBase]
	case focusID >= focusWheelProfileActionBase && focusID < focusWheelProfileActionBase+len(wheelProfileActionRects):
		if currentPage != pageWheel || wheelSubtab != wheelSubtabProfiles {
			return false
		}
		r = wheelProfileActionRects[focusID-focusWheelProfileActionBase]
	case focusID >= focusWheelDeviceActionBase && focusID < focusWheelDeviceActionBase+len(wheelDeviceActionRects):
		if currentPage != pageWheel || wheelSubtab != wheelSubtabDevice {
			return false
		}
		r = wheelDeviceActionRects[focusID-focusWheelDeviceActionBase]
	default:
		return false
	}
	if r.Right <= r.Left || r.Bottom <= r.Top {
		return true
	}
	return wheelReworkHandleClick((r.Left+r.Right)/2, (r.Top+r.Bottom)/2, s)
}

func wheelReworkHandleClick(x, y int32, s system.State) bool {
	if wheelCalibrationWizard.Active {
		return calibrationWizardHandleClick(x, y, s)
	}
	kind, i := wheelReworkHitTest(x, y)
	if kind == "" {
		return false
	}
	switch kind {
	case "cal":
		if !system.HasReadableSelectedWheel(s) {
			queueNotice("Kein eindeutig lesbares Lenkrad ausgewählt.", "Kalibrierung", MB_OK|MB_ICONWARNING)
			return true
		}
		switch i {
		case 0:
			startInlineCalibration("steering", s)
		case 1:
			startInlineCalibration("pedals", s)
		case 2:
			startInlineCalibration("button", s)
		case 3:
			startInlineCalibration("shifter", s)
		}
	case "profile":
		switch i {
		case 0:
			showNativeEngineProfiles(s)
		case 1:
			showGameProfileTools(s)
		case 2:
			wheelSubtab = wheelSubtabFFB
			contentScroll = 0
		case 3:
			b, err := system.ExportGameProfilesJSON(s.DataDir)
			if err != nil {
				queueNotice(err.Error(), "Profile exportieren", MB_OK|MB_ICONERROR)
				break
			}
			path := s.DataDir + "\\game-profiles-export.json"
			if err := osWriteFileCompat(path, b); err != nil {
				queueNotice(err.Error(), "Profile exportieren", MB_OK|MB_ICONERROR)
			} else {
				setActionFeedback("Game-Profile exportiert: " + path)
			}
		}
	case "device":
		switch i {
		case 0:
			showWheelManager(s)
		case 1:
			copyRawReport(s)
		case 2:
			wheelSubtab = wheelSubtabLive
			wheelAdvancedView = true
			contentScroll = 0
		case 3:
			if err := system.OpenGameControllers(); err != nil {
				queueNotice(err.Error(), "Windows Controller", MB_OK|MB_ICONERROR)
			}
		}
	}
	invalidate(mainWnd)
	return true
}

// Tiny wrapper keeps filesystem mutation centralized in this UI file and makes
// the profile export branch easy to regression-test without duplicating flags.
func osWriteFileCompat(path string, data []byte) error {
	return system.AtomicWriteFile(path, data, 0600)
}
