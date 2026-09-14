//go:build windows

package app

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unsafe"

	"github.com/thelittlespace/LogiMate/internal/system"
)

const (
	wheelSubtabLive = iota
	wheelSubtabFFB
	wheelSubtabCalibration
	wheelSubtabProfiles
	wheelSubtabDevice
	wheelSubtabCount
)

const (
	d6FocusTabLive        = 420
	d6FocusTabFFB         = 421
	d6FocusTabCalibration = 422
	d6FocusTabProfiles    = 423
	d6FocusTabDevice      = 424
	d6FocusSliderBase     = 430
	d6FocusToggle         = 450
	d6FocusNativeOutput   = 451
	d6FocusProfileBase    = 460
	d6FocusShapeBase      = 470
	d6FocusProbe          = 479
	d6FocusTestBase       = 480
	d62FocusViewBase      = 500
)

type d6SliderKind int

const (
	d6Master d6SliderKind = iota
	d6Constant
	d6Spring
	d6Damper
	d6Friction
	d6Rotation
	d6GameConstant
	d6Transient
	d6Deadband
	d6MinimumForce
	d6Response
	d6LowPass
	d6Smoothing
	d6OutputLimit
	d6TestStrength
	d6SliderCount
)

const (
	d62ViewBasis = iota
	d62ViewEffects
	d62ViewSignal
	d62ViewLiveTest
	d62ViewCount
)

type d6FFBDraft struct {
	valid        bool
	wheelID      string
	profile      system.NativeEngineProfile
	advanced     system.AdvancedFFBConfig
	testStrength int
}

type d6PanelStatus struct {
	Level string // info | good | warn | error
	Title string
	Text  string
	At    time.Time
}

var (
	wheelSubtab      = wheelSubtabLive
	wheelSubtabRects [wheelSubtabCount]RECT
	wheelSubtabHover = -1

	d6Draft            d6FFBDraft
	d6SliderRects      [d6SliderCount]RECT
	d6SliderHover      = -1
	d6SliderDrag       = -1
	d6ToggleRect       RECT
	d6NativeOutputRect RECT
	d6ProbeRect        RECT
	d6ProfileRects     [3]RECT
	d6ShapeRects       [4]RECT
	d6TestRects        [6]RECT
	d62ViewRects       [d62ViewCount]RECT
	d62ViewHover       = -1
	d62ActiveView      = d62ViewBasis
	d62LiveMonitorRect RECT
	d6HoverKind        string
	d6HoverIndex       = -1
	d6PressedKind      string
	d6PressedIndex     = -1
	d6Status           d6PanelStatus
)

func d6PointInRect(r RECT, x, y int32) bool {
	return r.Right > r.Left && r.Bottom > r.Top && x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom
}

func d6ResetRects() {
	for i := range d6SliderRects {
		d6SliderRects[i] = RECT{}
	}
	for i := range d6ProfileRects {
		d6ProfileRects[i] = RECT{}
	}
	for i := range d6ShapeRects {
		d6ShapeRects[i] = RECT{}
	}
	for i := range d6TestRects {
		d6TestRects[i] = RECT{}
	}
	for i := range d62ViewRects {
		d62ViewRects[i] = RECT{}
	}
	d62LiveMonitorRect = RECT{}
	d6ToggleRect = RECT{}
	d6NativeOutputRect = RECT{}
	d6ProbeRect = RECT{}
}

func d6SetStatus(level, title, text string) {
	d6Status = d6PanelStatus{Level: level, Title: strings.TrimSpace(title), Text: strings.TrimSpace(text), At: time.Now()}
}

func d6StatusColor(level string) uintptr {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "good":
		return colGood
	case "warn":
		return colWarning
	case "error":
		return colBad
	default:
		return colAccent
	}
}

func d6HIDShareLabel(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "protected-reader":
		return "geschützter Writer"
	case "shared-rw":
		return "Geteilter HID-Writer"
	case "shared-g27-session":
		return "G27 Unified Session"
	default:
		return "noch nicht geprüft"
	}
}

func d6ProbeNativeWriter(s system.State, automatic bool) (system.NativeOutputProbeResult, error) {
	result, err := system.ProbeNativeOutputTransport(s)
	if err != nil {
		conflicts := system.NativeHIDConflictProcesses()
		detail := err.Error()
		if len(conflicts) > 0 {
			detail = fmt.Sprintf("%s · erkannte konkurrierende Software: %s", detail, strings.Join(conflicts, ", "))
		}
		title := "HID blockiert"
		if automatic {
			title = "FFB-Start blockiert · HID"
		}
		d6SetStatus("error", title, detail)
		return result, err
	}
	if result.ShareMode == "shared-g27-session" {
		if !automatic && system.SelectedWheelHasRPMLEDs(s) {
			if pingErr := system.NativeOutputPulseLEDs(s, 220*time.Millisecond); pingErr != nil {
				d6SetStatus("error", "G27 Output-Ping fehlgeschlagen", pingErr.Error())
				return result, pingErr
			}
			d6SetStatus("good", "G27 Unified HID · Output-Ping gesendet", "Input und Output teilen dieselbe C29B-Session · die fünf RPM-LEDs sollten kurz aufgeleuchtet haben · Backend: "+result.Backend)
		} else {
			d6SetStatus("good", "G27 Unified HID bereit", "Input und Output verwenden dieselbe C29B-Gerätesession · Backend: "+result.Backend)
		}
	} else if result.ShareMode == "shared-rw" {
		d6SetStatus("warn", "HID bereit · Shared", "Writer kooperativ über FILE_SHARE_READ|WRITE geöffnet · Backend: "+result.Backend)
	} else {
		d6SetStatus("good", "HID bereit", "Writer geöffnet · "+d6HIDShareLabel(result.ShareMode)+" · Backend: "+result.Backend)
	}
	return result, nil
}

func paintWheelSubtabs(hdc uintptr, content RECT) {
	labels := []string{"Live", "Force Feedback", "Kalibrierung", "Profile", "Gerät"}
	left := content.Left + 20
	top := content.Top + 53
	gap := int32(7)
	widths := []int32{82, 142, 124, 92, 82}
	for i, label := range labels {
		r := RECT{left, top, left + widths[i], top + 34}
		wheelSubtabRects[i] = r
		selected := wheelSubtab == i
		fill, border, text := colPanel2, colBorder, colMuted
		if selected {
			fill, border = blendColor(colAccentSoft, colPanel2, 28), colAccent
			text = readableTextColor(colAccent, fill)
		} else if wheelSubtabHover == i {
			fill = colHover
		}
		focusID := d6FocusTabLive + i
		if keyboardFocus == focusID {
			border = colFocusRing
		}
		drawRoundRect(hdc, r, 10, fill, border)
		pSetTextColor.Call(hdc, text)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
		drawText(hdc, label, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		left = r.Right + gap
	}
}

func wheelSubtabHitTest(x, y int32) int {
	if currentPage != pageWheel {
		return -1
	}
	for i, r := range wheelSubtabRects {
		if d6PointInRect(r, x, y) {
			return i
		}
	}
	return -1
}

func ensureD6Draft(s system.State) bool {
	if strings.TrimSpace(s.SelectedWheelID) == "" {
		d6Draft = d6FFBDraft{}
		return false
	}
	if d6Draft.valid && strings.EqualFold(d6Draft.wheelID, s.SelectedWheelID) {
		return true
	}
	p := system.ReadActiveNativeEngineProfile(s.DataDir, s.SelectedWheelID)
	a, ok := system.ReadSelectedAdvancedFFBConfig(s)
	if !ok {
		return false
	}
	d6Draft = d6FFBDraft{valid: true, wheelID: s.SelectedWheelID, profile: p, advanced: a, testStrength: system.NativeManualFFBTestDefaultPercent}
	return true
}

func d6SliderMeta(kind d6SliderKind) (label string, minV, maxV int, value int, display string) {
	p, a := d6Draft.profile, d6Draft.advanced
	switch kind {
	case d6Master:
		return "Gesamtstärke", 1, 100, p.MasterGainPercent, fmt.Sprintf("%d %%", p.MasterGainPercent)
	case d6Constant:
		return "Constant Force", 0, 100, p.ConstantGainPercent, fmt.Sprintf("%d %%", p.ConstantGainPercent)
	case d6Spring:
		return "Spring", 0, 100, p.SpringGainPercent, fmt.Sprintf("%d %%", p.SpringGainPercent)
	case d6Damper:
		return "Damper", 0, 100, p.DamperGainPercent, fmt.Sprintf("%d %%", p.DamperGainPercent)
	case d6Friction:
		return "Friction", 0, 100, p.FrictionGainPercent, fmt.Sprintf("%d %%", p.FrictionGainPercent)
	case d6Rotation:
		vals := []int{270, 360, 540, 720, 900}
		idx := 4
		for i, v := range vals {
			if p.RotationDegrees == v {
				idx = i
				break
			}
		}
		return "Lenkwinkel", 0, 4, idx, fmt.Sprintf("%d°", vals[idx])
	case d6GameConstant:
		return "Game Constant Gain", 0, 150, a.ConstantGainPercent, fmt.Sprintf("%d %%", a.ConstantGainPercent)
	case d6Transient:
		return "Transient Gain", 0, 150, a.TransientGainPercent, fmt.Sprintf("%d %%", a.TransientGainPercent)
	case d6Deadband:
		return "Deadband", 0, 20, a.DeadbandPercent, fmt.Sprintf("%d %%", a.DeadbandPercent)
	case d6MinimumForce:
		return "Minimum Force", 0, 20, a.MinimumForcePercent, fmt.Sprintf("%d %%", a.MinimumForcePercent)
	case d6Response:
		v := int(math.Round((a.ResponseExponent - .5) * 100))
		return "Response Curve", 0, 150, v, fmt.Sprintf("%.2f", a.ResponseExponent)
	case d6LowPass:
		return "Low-Pass", 0, 100, a.LowPassHz, map[bool]string{true: "Aus", false: fmt.Sprintf("%d Hz", a.LowPassHz)}[a.LowPassHz == 0]
	case d6Smoothing:
		return "Smoothing", 0, 90, a.SmoothingPercent, fmt.Sprintf("%d %%", a.SmoothingPercent)
	case d6OutputLimit:
		return "Pre-Safety Limit", 10, 100, a.OutputLimitPercent, fmt.Sprintf("%d %%", a.OutputLimitPercent)
	case d6TestStrength:
		return "Teststärke", 1, system.NativeManualFFBTestMaxPercent, d6Draft.testStrength, fmt.Sprintf("%d %%", d6Draft.testStrength)
	}
	return "", 0, 1, 0, ""
}

func d6SetSliderValue(kind d6SliderKind, v int) {
	_, minV, maxV, _, _ := d6SliderMeta(kind)
	if v < minV {
		v = minV
	}
	if v > maxV {
		v = maxV
	}
	switch kind {
	case d6Master:
		d6Draft.profile.MasterGainPercent = v
	case d6Constant:
		d6Draft.profile.ConstantGainPercent = v
	case d6Spring:
		d6Draft.profile.SpringGainPercent = v
	case d6Damper:
		d6Draft.profile.DamperGainPercent = v
	case d6Friction:
		d6Draft.profile.FrictionGainPercent = v
	case d6Rotation:
		vals := []int{270, 360, 540, 720, 900}
		d6Draft.profile.RotationDegrees = vals[v]
	case d6GameConstant:
		d6Draft.advanced.ConstantGainPercent = v
	case d6Transient:
		d6Draft.advanced.TransientGainPercent = v
	case d6Deadband:
		d6Draft.advanced.DeadbandPercent = v
	case d6MinimumForce:
		d6Draft.advanced.MinimumForcePercent = v
	case d6Response:
		d6Draft.advanced.ResponseExponent = .5 + float64(v)/100
	case d6LowPass:
		if v < 5 {
			v = 0
		}
		d6Draft.advanced.LowPassHz = v
	case d6Smoothing:
		d6Draft.advanced.SmoothingPercent = v
	case d6OutputLimit:
		d6Draft.advanced.OutputLimitPercent = v
	case d6TestStrength:
		d6Draft.testStrength = v
	}
	switch {
	case kind <= d6Rotation:
		d6Draft.profile.Name = "Custom"
	case kind >= d6GameConstant && kind <= d6OutputLimit:
		d6Draft.advanced.Preset = "Custom"
	}
}

func d6SliderValueFromX(kind d6SliderKind, r RECT, x int32) int {
	_, minV, maxV, _, _ := d6SliderMeta(kind)
	if r.Right <= r.Left {
		return minV
	}
	if x < r.Left {
		x = r.Left
	}
	if x > r.Right {
		x = r.Right
	}
	f := float64(x-r.Left) / float64(r.Right-r.Left)
	return minV + int(math.Round(f*float64(maxV-minV)))
}

func d62SliderEnabled(kind d6SliderKind) bool {
	// No current gameadapter publishes a semantic Transient channel. Keeping the
	// saved value visible is useful for forward compatibility, but presenting it
	// as an active slider was misleading in D6.0/D6.1.
	return kind != d6Transient
}

func d62SliderBadge(kind d6SliderKind) string {
	switch kind {
	case d6Master:
		return "MASTER"
	case d6Constant:
		return "GAME+TEST"
	case d6Spring, d6Damper, d6Friction:
		return "CONDITION"
	case d6Rotation:
		return "HARDWARE"
	case d6GameConstant, d6Deadband, d6MinimumForce, d6Response, d6LowPass, d6Smoothing, d6OutputLimit:
		return "GAME"
	case d6Transient:
		return "NICHT AKTIV"
	case d6TestStrength:
		return "TEST"
	default:
		return ""
	}
}

func d6DrawSlider(hdc uintptr, kind d6SliderKind, r RECT) {
	label, minV, maxV, value, display := d6SliderMeta(kind)
	enabled := d62SliderEnabled(kind)
	d6SliderRects[kind] = r
	textColor := colText
	mutedColor := colMuted
	trackColor := colTrack
	accentColor := colAccent
	if !enabled {
		textColor = colMuted2
		mutedColor = colMuted2
		trackColor = blendColor(colTrack, colPanel2, 45)
		accentColor = colMuted2
	}
	pSetTextColor.Call(hdc, textColor)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	lr := RECT{r.Left, r.Top, r.Right - 168, r.Top + 22}
	drawText(hdc, label, &lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	badge := d62SliderBadge(kind)
	if badge != "" {
		pSetTextColor.Call(hdc, mutedColor)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		br := RECT{r.Right - 160, r.Top, r.Right - 72, r.Top + 22}
		drawText(hdc, badge, &br, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
	pSetTextColor.Call(hdc, mutedColor)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	vr := RECT{r.Right - 68, r.Top, r.Right, r.Top + 22}
	drawText(hdc, display, &vr, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)

	track := RECT{r.Left, r.Top + 31, r.Right, r.Top + 39}
	fillRoundRect(hdc, track, 4, trackColor)
	f := 0.0
	if maxV > minV {
		f = float64(value-minV) / float64(maxV-minV)
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	px := track.Left + int32(math.Round(f*float64(track.Right-track.Left)))
	if px > track.Left {
		fillRoundRect(hdc, RECT{track.Left, track.Top, px, track.Bottom}, 4, accentColor)
	}
	knob := RECT{px - 7, track.Top - 4, px + 7, track.Bottom + 4}
	border := accentColor
	if enabled && (d6SliderHover == int(kind) || d6SliderDrag == int(kind)) {
		border = colFocusRing
	}
	if enabled && keyboardFocus == d6FocusSliderBase+int(kind) {
		border = colFocusRing
	}
	knobFill := colToggleKnobOn
	if !enabled {
		knobFill = colMuted2
	}
	ellipse(hdc, knob, knobFill, border)
}

func d6DrawSectionTitle(hdc uintptr, r RECT, title, desc string) {
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSection))
	tr := RECT{r.Left + 16, r.Top + 11, r.Right - 16, r.Top + 34}
	drawText(hdc, title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	if desc != "" {
		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		dr := RECT{r.Left + 16, r.Top + 34, r.Right - 16, r.Top + 55}
		drawText(hdc, desc, &dr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func d6DrawChip(hdc uintptr, r RECT, label string, selected, hovered, focused bool) {
	fill, border, text := colPanel, colBorder, colMuted
	if selected {
		fill, border = blendColor(colAccentSoft, colPanel, 32), colAccent
		text = readableTextColor(colAccent, fill)
	}
	if hovered {
		fill = colHover
	}
	if focused {
		border = colFocusRing
	}
	drawRoundRect(hdc, r, 10, fill, border)
	pSetTextColor.Call(hdc, text)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
	drawText(hdc, label, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}

func d6DrawDisabledChip(hdc uintptr, r RECT, label string, focused bool) {
	border := colBorder
	if focused {
		border = colFocusRing
	}
	drawRoundRect(hdc, r, 10, colPanel, border)
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
	drawText(hdc, label, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}

func d6DrawInlineStatus(hdc uintptr, r RECT) {
	if strings.TrimSpace(d6Status.Title) == "" && strings.TrimSpace(d6Status.Text) == "" {
		return
	}
	accent := d6StatusColor(d6Status.Level)
	fillRoundRect(hdc, RECT{r.Left, r.Top, r.Left + 4, r.Bottom}, 2, accent)
	titleW := (r.Right - r.Left) * 28 / 100
	if titleW < 120 {
		titleW = 120
	}
	if titleW > 220 {
		titleW = 220
	}
	pSetTextColor.Call(hdc, readableTextColor(accent, colPanel2))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	title := RECT{r.Left + 12, r.Top, r.Left + titleW, r.Bottom}
	drawText(hdc, d6Status.Title, &title, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	text := RECT{r.Left + titleW + 8, r.Top, r.Right, r.Bottom}
	drawText(hdc, d6Status.Text, &text, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}

func d62DrawViewNav(hdc uintptr, r RECT) {
	labels := []string{"Basis", "Effekte", "Signalformung", "Live & Test"}
	widths := []int32{104, 112, 148, 132}
	x := r.Left
	for i, label := range labels {
		vr := RECT{x, r.Top, x + widths[i], r.Bottom}
		d62ViewRects[i] = vr
		d6DrawChip(hdc, vr, label, d62ActiveView == i, d62ViewHover == i, keyboardFocus == d62FocusViewBase+i)
		x = vr.Right + 8
	}
}

func d62DrawInfoLine(hdc uintptr, r RECT, label, value string, valueColor uintptr) {
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
	lr := RECT{r.Left, r.Top, r.Left + 160, r.Bottom}
	drawText(hdc, label, &lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, valueColor)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	vr := RECT{r.Left + 166, r.Top, r.Right, r.Bottom}
	drawText(hdc, value, &vr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}

func paintWheelFFBControlPanel(hdc uintptr, content RECT, s system.State) {
	d6ResetRects()
	visibleTop := content.Top + 98
	visibleBottom := content.Bottom - 16
	visibleHeight := visibleBottom - visibleTop
	virtualHeight := int32(650)
	switch d62ActiveView {
	case d62ViewSignal:
		virtualHeight = 840
	case d62ViewLiveTest:
		virtualHeight = 730
	case d62ViewEffects:
		virtualHeight = 610
	}
	contentScrollMax = virtualHeight - visibleHeight
	if contentScrollMax < 0 {
		contentScrollMax = 0
	}
	if contentScroll > contentScrollMax {
		contentScroll = contentScrollMax
	}
	y := content.Top + 101 - contentScroll
	left, right := content.Left+20, content.Right-20
	gap := int32(12)

	saved, _, _ := pSaveDC.Call(hdc)
	pIntersectClipRect.Call(hdc, uintptr(content.Left+16), uintptr(visibleTop), uintptr(content.Right-14), uintptr(visibleBottom))

	if !ensureD6Draft(s) {
		r := RECT{left, y, right, y + 130}
		drawRoundRect(hdc, r, 16, colPanel2, colBorder)
		d6DrawSectionTitle(hdc, r, "Force Feedback", "Kein eindeutig ausgewähltes Wheel")
		tx := RECT{r.Left + 16, r.Top + 66, r.Right - 16, r.Bottom - 16}
		drawFittedParagraph(hdc, "Wähle zuerst ein eindeutig erkanntes G25, G27 oder Driving Force GT. FFB-Einstellungen werden pro physischem Wheel gespeichert.", tx, colMuted, fontBody, fontSmall)
		if saved != 0 {
			pRestoreDC.Call(hdc, saved)
		}
		paintScrollBar(hdc, content, visibleHeight, virtualHeight)
		return
	}

	nativeEnabled := getUISettings().NativeWheelOutput
	if strings.TrimSpace(d6Status.Title) == "" {
		if nativeEnabled {
			d6SetStatus("info", "Bereit", "Einstellungen werden gespeichert; laufendes Game-FFB erhält kompatible Änderungen jetzt live.")
		} else {
			d6SetStatus("warn", "Native Output aus", "Tuning bleibt editierbar. Hardware-Kommandos und Motor-Tests bleiben gesperrt.")
		}
	}

	// Persistent header. The runtime state is deliberately separated from the
	// four editing views so the user can always see what is actually active.
	head := RECT{left, y, right, y + 148}
	drawRoundRect(hdc, head, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSection))
	tr := RECT{head.Left + 16, head.Top + 10, head.Right - 330, head.Top + 36}
	drawText(hdc, "Force Feedback · Wheel Control Panel", &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	sr := RECT{head.Left + 16, head.Top + 37, head.Right - 16, head.Top + 60}
	drawText(hdc, "LogiMate FFB · verifizierte Wirkbereiche · Live-Tuning · Safety Mixer bleibt letzte Instanz", &sr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	d6NativeOutputRect = RECT{head.Right - 295, head.Top + 12, head.Right - 154, head.Top + 43}
	nativeLabel := "Native Output AUS"
	if nativeEnabled {
		nativeLabel = "Native Output AN"
	}
	d6DrawChip(hdc, d6NativeOutputRect, nativeLabel, nativeEnabled, d6HoverKind == "native-output", keyboardFocus == d6FocusNativeOutput)
	d6ToggleRect = RECT{head.Right - 145, head.Top + 12, head.Right - 16, head.Top + 43}
	toggleLabel := "FFB Pipeline AUS"
	if d6Draft.advanced.Enabled {
		toggleLabel = "FFB Pipeline AN"
	}
	d6DrawChip(hdc, d6ToggleRect, toggleLabel, d6Draft.advanced.Enabled, d6HoverKind == "toggle", keyboardFocus == d6FocusToggle)

	game := system.NativeGameOutputSnapshot()
	runtime := "Kein Game-FFB aktiv"
	runtimeColor := colMuted2
	if game.Active && game.FFBEnabled {
		runtime = fmt.Sprintf("LIVE · %s · Spielprofil %s · Game Gain %d×%d×%d %% · Engine %s · Rev %d", game.Adapter, advancedValue(game.Profile), game.GameFFBGainPercent, game.GameMasterGainPercent, game.GameConstantGainPercent, advancedValue(game.EngineProfile), game.TuningRevision)
		runtimeColor = colGood
	}
	pSetTextColor.Call(hdc, runtimeColor)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	rr := RECT{head.Left + 16, head.Top + 66, head.Right - 16, head.Top + 87}
	drawText(hdc, runtime, &rr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	metrics := system.NativeHIDTransportMetricsSnapshot()
	hidText := fmt.Sprintf("HID Writer: %s · Backend: %s · Shared-Fallbacks: %d", d6HIDShareLabel(metrics.LastShareMode), advancedValue(metrics.LastBackend), metrics.SharingFallbacks)
	if conflicts := system.NativeHIDConflictProcesses(); len(conflicts) > 0 {
		hidText += " · Konflikt: " + strings.Join(conflicts, ", ")
	}
	hidColor := colMuted2
	if metrics.LastShareMode == "shared-rw" {
		hidColor = colWarning
	} else if metrics.LastShareMode == "shared-g27-session" {
		hidColor = colGood
	}
	pSetTextColor.Call(hdc, hidColor)
	hr := RECT{head.Left + 16, head.Top + 88, head.Right - 16, head.Top + 108}
	drawText(hdc, hidText, &hr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	d6DrawInlineStatus(hdc, RECT{head.Left + 16, head.Top + 115, head.Right - 16, head.Bottom - 9})
	y = head.Bottom + gap

	nav := RECT{left, y, right, y + 38}
	d62DrawViewNav(hdc, nav)
	y = nav.Bottom + gap

	switch d62ActiveView {
	case d62ViewBasis:
		presets := RECT{left, y, right, y + 92}
		drawRoundRect(hdc, presets, 16, colPanel2, colBorder)
		d6DrawSectionTitle(hdc, presets, "Wheel-Profil", "Grundcharakter des ausgewählten Lenkrads")
		bx := presets.Left + 16
		bw := int32(126)
		for i, name := range []string{"Sanft", "Ausgewogen", "Direkt"} {
			r := RECT{bx + int32(i)*(bw+8), presets.Top + 52, bx + int32(i)*(bw+8) + bw, presets.Bottom - 10}
			d6ProfileRects[i] = r
			selected := strings.EqualFold(d6Draft.profile.Name, []string{"Gentle", "Balanced", "Direct"}[i])
			d6DrawChip(hdc, r, name, selected, d6HoverKind == "profile" && d6HoverIndex == i, keyboardFocus == d6FocusProfileBase+i)
		}
		y = presets.Bottom + gap

		colW := (right - left - gap) / 2
		baseCard := RECT{left, y, left + colW, y + 180}
		stateCard := RECT{baseCard.Right + gap, y, right, y + 180}
		drawRoundRect(hdc, baseCard, 16, colPanel2, colBorder)
		drawRoundRect(hdc, stateCard, 16, colPanel2, colBorder)
		d6DrawSectionTitle(hdc, baseCard, "Basis", "Gesamtstärke und Hardware-Lenkwinkel")
		d6DrawSlider(hdc, d6Master, RECT{baseCard.Left + 18, baseCard.Top + 64, baseCard.Right - 18, baseCard.Top + 108})
		d6DrawSlider(hdc, d6Rotation, RECT{baseCard.Left + 18, baseCard.Top + 119, baseCard.Right - 18, baseCard.Top + 163})

		d6DrawSectionTitle(hdc, stateCard, "Wirksamkeit", "Was aktuell wirklich am Runtime-Pfad ankommt")
		activeText := "beim nächsten Game-FFB Start"
		activeColor := colMuted
		if game.Active && game.FFBEnabled {
			activeText = "live aktualisierbar"
			activeColor = colGood
		}
		d62DrawInfoLine(hdc, RECT{stateCard.Left + 18, stateCard.Top + 64, stateCard.Right - 18, stateCard.Top + 86}, "Master / Constant", activeText, activeColor)
		d62DrawInfoLine(hdc, RECT{stateCard.Left + 18, stateCard.Top + 91, stateCard.Right - 18, stateCard.Top + 113}, "Spring/Damper/Friction", "Condition-Effekte / Tests", colWarning)
		rotText := "gespeichert"
		rotColor := colMuted
		if nativeEnabled {
			rotText = "Hardware-Write bei Änderung"
			rotColor = colGood
		}
		d62DrawInfoLine(hdc, RECT{stateCard.Left + 18, stateCard.Top + 118, stateCard.Right - 18, stateCard.Top + 140}, "Lenkwinkel", rotText, rotColor)
		d62DrawInfoLine(hdc, RECT{stateCard.Left + 18, stateCard.Top + 145, stateCard.Right - 18, stateCard.Top + 167}, "Safety", system.NativeFFBConfigSummary(system.ReadNativeFFBConfig(s.DataDir, s.SelectedWheelID)), colMuted)

	case d62ViewEffects:
		card := RECT{left, y, right, y + 276}
		drawRoundRect(hdc, card, 16, colPanel2, colBorder)
		d6DrawSectionTitle(hdc, card, "Effekt-Gains", "Constant beeinflusst den aktuellen Game-Force-Pfad; Condition-Effekte nur wenn sie tatsächlich erzeugt werden")
		colW := (card.Right - card.Left - 54) / 2
		lx := card.Left + 18
		rx := card.Left + 36 + colW
		d6DrawSlider(hdc, d6Constant, RECT{lx, card.Top + 70, lx + colW, card.Top + 114})
		d6DrawSlider(hdc, d6Spring, RECT{lx, card.Top + 134, lx + colW, card.Top + 178})
		d6DrawSlider(hdc, d6Damper, RECT{rx, card.Top + 70, rx + colW, card.Top + 114})
		d6DrawSlider(hdc, d6Friction, RECT{rx, card.Top + 134, rx + colW, card.Top + 178})
		note := RECT{card.Left + 18, card.Top + 202, card.Right - 18, card.Bottom - 14}
		drawFittedParagraph(hdc, "Hinweis: Der derzeitige Telemetrie-Gamepfad liefert semantische Constant Force. Spring, Damper und Friction sind echte Engine-Gains, werden aber nur hör-/fühlbar, wenn ein Spiel/Adapter oder der sichere Live-Test genau diesen Condition-Effekt anfordert.", note, colMuted2, fontSmall, fontSmall)

	case d62ViewSignal:
		presets := RECT{left, y, right, y + 92}
		drawRoundRect(hdc, presets, 16, colPanel2, colBorder)
		d6DrawSectionTitle(hdc, presets, "Signalformung", "FFB-Presets · vor dem harten Safety-Mixer")
		sx := presets.Left + 16
		pw := (presets.Right - presets.Left - 32 - 3*8) / 4
		for i, name := range []string{"Neutral", "Smooth", "Responsive", "Compensated"} {
			r := RECT{sx + int32(i)*(pw+8), presets.Top + 52, sx + int32(i)*(pw+8) + pw, presets.Bottom - 10}
			d6ShapeRects[i] = r
			selected := strings.HasPrefix(strings.ToLower(d6Draft.advanced.Preset), strings.ToLower(name))
			d6DrawChip(hdc, r, name, selected, d6HoverKind == "shape" && d6HoverIndex == i, keyboardFocus == d6FocusShapeBase+i)
		}
		y = presets.Bottom + gap
		colW := (right - left - gap) / 2
		leftCard := RECT{left, y, left + colW, y + 294}
		rightCard := RECT{leftCard.Right + gap, y, right, y + 294}
		drawRoundRect(hdc, leftCard, 16, colPanel2, colBorder)
		drawRoundRect(hdc, rightCard, 16, colPanel2, colBorder)
		d6DrawSectionTitle(hdc, leftCard, "Game-FFB Formung", "Diese Werte werden bei laufendem Game-FFB live übernommen")
		d6DrawSectionTitle(hdc, rightCard, "Filter & Limit", "Transient bleibt bis zu einem echten Adapter-Kanal deaktiviert")
		sy := leftCard.Top + 66
		for i, k := range []d6SliderKind{d6GameConstant, d6Deadband, d6MinimumForce, d6Response} {
			d6DrawSlider(hdc, k, RECT{leftCard.Left + 18, sy + int32(i)*55, leftCard.Right - 18, sy + int32(i)*55 + 44})
		}
		sy = rightCard.Top + 66
		for i, k := range []d6SliderKind{d6Transient, d6LowPass, d6Smoothing, d6OutputLimit} {
			d6DrawSlider(hdc, k, RECT{rightCard.Left + 18, sy + int32(i)*55, rightCard.Right - 18, sy + int32(i)*55 + 44})
		}
		y = leftCard.Bottom + gap
		note := RECT{left, y, right, y + 88}
		drawRoundRect(hdc, note, 14, colPanel2, colBorder)
		pSetTextColor.Call(hdc, colWarning)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
		nr := RECT{note.Left + 16, note.Top + 11, note.Right - 16, note.Top + 32}
		drawText(hdc, "Transient Gain ist derzeit bewusst gesperrt", &nr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		nr = RECT{note.Left + 16, note.Top + 34, note.Right - 16, note.Bottom - 9}
		drawFittedParagraph(hdc, "Kein aktuell registrierter Adapter liefert einen separaten Transient-Force-Kanal. Frühere Alpha-Builds zeigten den Regler trotzdem als aktiv – das war irreführend.", nr, colMuted2, fontSmall, fontSmall)

	case d62ViewLiveTest:
		live := RECT{left, y, right, y + 190}
		d62LiveMonitorRect = live
		drawRoundRect(hdc, live, 16, colPanel2, colBorder)
		d6DrawSectionTitle(hdc, live, "Live FFB Monitor", "Nur dieser Bereich wird vom 250-ms-Live-Timer aktualisiert")
		ffb := system.NativeFFBSnapshot()
		nativeOut := system.NativeOutputSnapshot()
		barsLeft := live.Left + 18
		barsRight := live.Right - 18
		vals := []struct {
			name string
			v    float64
			text string
		}{
			{"Raw", math.Abs(game.RequestedForce), fmt.Sprintf("%+.3f", game.RequestedForce)},
			{"Shaped", math.Abs(game.ShapedForce), fmt.Sprintf("%+.3f", game.ShapedForce)},
			{"Applied", math.Abs(float64(game.AppliedPercent)) / 100, fmt.Sprintf("%+d %%", game.AppliedPercent)},
		}
		monitorMode := "Game / Telemetrie"
		if ffb.Active {
			monitorMode = "Hardware-Test · " + ffb.Effect
			requestedMax := system.NativeManualFFBTestMaxPercent
			appliedMax := system.NativeManualFFBTestMaxPercent
			applied := ffb.Applied
			switch ffb.Effect {
			case "spring-test":
				applied = ffb.SpringApplied
			case "damper-test":
				applied = ffb.DamperApplied
			case "friction-test":
				applied = ffb.FrictionApplied
			}
			vals = []struct {
				name string
				v    float64
				text string
			}{
				{"Requested", math.Abs(float64(ffb.Requested)) / float64(requestedMax), fmt.Sprintf("%d %%", ffb.Requested)},
				{"Applied", math.Abs(float64(applied)) / float64(appliedMax), fmt.Sprintf("%d %%", applied)},
				{"Test cap", 1, fmt.Sprintf("%d %%", appliedMax)},
			}
		} else if nativeOut.Active && strings.Contains(strings.ToLower(nativeOut.LastCommand), "autocenter") {
			monitorMode = "Hardware-Test · autocenter"
			frac := float64(d6Draft.testStrength) / float64(system.NativeManualFFBTestMaxPercent)
			vals = []struct {
				name string
				v    float64
				text string
			}{
				{"Requested", frac, fmt.Sprintf("%d %%", d6Draft.testStrength)},
				{"Spring set", frac, "Set + Enable"},
				{"Dauer", 1, "3 s"},
			}
		}
		for i, v := range vals {
			yy := live.Top + 62 + int32(i)*31
			pSetTextColor.Call(hdc, colMuted)
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
			lr := RECT{barsLeft, yy, barsLeft + 70, yy + 20}
			drawText(hdc, v.name, &lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			vr := RECT{barsRight - 82, yy, barsRight, yy + 20}
			drawText(hdc, v.text, &vr, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
			track := RECT{barsLeft + 76, yy + 7, barsRight - 90, yy + 14}
			fillRoundRect(hdc, track, 4, colTrack)
			frac := v.v
			if frac > 1 {
				frac = 1
			}
			if frac < 0 {
				frac = 0
			}
			if frac > 0 {
				fillRoundRect(hdc, RECT{track.Left, track.Top, track.Left + int32(float64(track.Right-track.Left)*frac), track.Bottom}, 4, colAccent)
			}
		}
		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		detail := RECT{live.Left + 18, live.Bottom - 34, live.Right - 18, live.Bottom - 10}
		drawText(hdc, fmt.Sprintf("%s · Frames %d · Clips Pipeline/Safety %d/%d · HID %s · Latenz Ø %s · Tuning Rev %d", monitorMode, ffb.Frames, game.PipelineClips, game.SafetyClips, game.TransportBackend, game.SampleLatency, game.TuningRevision), &detail, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		y = live.Bottom + gap

		test := RECT{left, y, right, y + 188}
		drawRoundRect(hdc, test, 16, colPanel2, colBorder)
		d6DrawSectionTitle(hdc, test, "Sicherer Live-Test", "Output zuerst prüfen · direkte 1–30 % Hardwaretests · automatisch nach 3 s neutralisiert")
		d6ProbeRect = RECT{test.Right - 124, test.Top + 12, test.Right - 16, test.Top + 43}
		d6DrawChip(hdc, d6ProbeRect, "Output prüfen", false, d6HoverKind == "probe", keyboardFocus == d6FocusProbe)
		testSlider := RECT{test.Left + 18, test.Top + 64, test.Left + 260, test.Top + 108}
		d6DrawSlider(hdc, d6TestStrength, testSlider)
		labels := []string{"Constant", "Spring", "Damper", "Friction", "Autocenter", "STOP"}
		bx := test.Left + 290
		avail := test.Right - bx - 16
		tw := (avail - int32(len(labels)-1)*7) / int32(len(labels))
		for i, label := range labels {
			r := RECT{bx + int32(i)*(tw+7), test.Top + 69, bx + int32(i)*(tw+7) + tw, test.Top + 105}
			d6TestRects[i] = r
			selected := false
			if i == 5 {
				selected = system.NativeOutputLeaseSnapshot().Active || ffb.Active
			} else if ffb.Active {
				want := []string{"constant-test", "spring-test", "damper-test", "friction-test", ""}[i]
				selected = want != "" && strings.EqualFold(ffb.Effect, want)
			} else if i == 4 && nativeOut.Active && strings.Contains(strings.ToLower(nativeOut.LastCommand), "autocenter") {
				selected = true
			}
			pressed := d6PressedKind == "test" && d6PressedIndex == i
			if !nativeEnabled && i != 5 {
				d6DrawDisabledChip(hdc, r, label, keyboardFocus == d6FocusTestBase+i)
			} else {
				d6DrawChip(hdc, r, label, selected || pressed, d6HoverKind == "test" && d6HoverIndex == i, keyboardFocus == d6FocusTestBase+i)
			}
		}
		tx := RECT{test.Left + 18, test.Top + 124, test.Right - 18, test.Bottom - 12}
		drawFittedParagraph(hdc, "Die Teststärke ist direkt und wird nicht durch Wheel-/Game-Profile abgeschwächt. Damper und Friction wirken als Widerstand beim Bewegen – während des Tests das Lenkrad drehen. Normales Game-FFB behält seine separaten Safety-Caps.", tx, colMuted2, fontSmall, fontSmall)
	}

	if saved != 0 {
		pRestoreDC.Call(hdc, saved)
	}
	paintScrollBar(hdc, content, visibleHeight, virtualHeight)
}

func d6FFBHitTest(x, y int32) (kind string, index int) {
	if currentPage != pageWheel || wheelSubtab != wheelSubtabFFB {
		return "", -1
	}
	for i, r := range d62ViewRects {
		if d6PointInRect(r, x, y) {
			return "view", i
		}
	}
	if d6PointInRect(d6ToggleRect, x, y) {
		return "toggle", 0
	}
	if d6PointInRect(d6NativeOutputRect, x, y) {
		return "native-output", 0
	}
	if d6PointInRect(d6ProbeRect, x, y) {
		return "probe", 0
	}
	for i, r := range d6SliderRects {
		if d62SliderEnabled(d6SliderKind(i)) && d6PointInRect(r, x, y) {
			return "slider", i
		}
	}
	for i, r := range d6ProfileRects {
		if d6PointInRect(r, x, y) {
			return "profile", i
		}
	}
	for i, r := range d6ShapeRects {
		if d6PointInRect(r, x, y) {
			return "shape", i
		}
	}
	for i, r := range d6TestRects {
		if d6PointInRect(r, x, y) {
			return "test", i
		}
	}
	return "", -1
}

func d6OnMouseMove(x, y int32) bool {
	oldTab, oldSlider, oldKind, oldIndex, oldView := wheelSubtabHover, d6SliderHover, d6HoverKind, d6HoverIndex, d62ViewHover
	wheelSubtabHover = wheelSubtabHitTest(x, y)
	d6SliderHover = -1
	d62ViewHover = -1
	d6HoverKind = ""
	d6HoverIndex = -1
	if kind, idx := d6FFBHitTest(x, y); kind != "" {
		d6HoverKind, d6HoverIndex = kind, idx
		if kind == "slider" {
			d6SliderHover = idx
		}
		if kind == "view" {
			d62ViewHover = idx
		}
	}
	if d6SliderDrag >= 0 {
		r := d6SliderRects[d6SliderDrag]
		d6SetSliderValue(d6SliderKind(d6SliderDrag), d6SliderValueFromX(d6SliderKind(d6SliderDrag), r, x))
		invalidate(mainWnd)
		return true
	}
	return oldTab != wheelSubtabHover || oldSlider != d6SliderHover || oldKind != d6HoverKind || oldIndex != d6HoverIndex || oldView != d62ViewHover
}

func d6OnMouseDown(x, y int32) bool {
	if tab := wheelSubtabHitTest(x, y); tab >= 0 {
		d6PressedKind, d6PressedIndex = "tab", tab
		setCapture(mainWnd)
		invalidate(mainWnd)
		return true
	}
	kind, idx := d6FFBHitTest(x, y)
	if kind == "" || idx < 0 {
		return false
	}
	if kind == "slider" {
		d6SliderDrag = idx
		setKeyboardFocusID(d6FocusSliderBase + idx)
		setCapture(mainWnd)
		r := d6SliderRects[idx]
		d6SetSliderValue(d6SliderKind(idx), d6SliderValueFromX(d6SliderKind(idx), r, x))
		invalidate(mainWnd)
		return true
	}
	// Build 009: arm every custom-painted control on mouse-down. This prevents
	// the generic bottom action bar or a transparent UIA child from consuming a
	// release before the FFB button gets its activation, and gives the user a
	// deterministic press/release interaction instead of a release-only hit test.
	d6PressedKind, d6PressedIndex = kind, idx
	setCapture(mainWnd)
	invalidate(mainWnd)
	return true
}

func d62RefreshLiveTuning(s system.State, reason string) error {
	result, err := system.RefreshNativeGameOutputTuning(s)
	if err != nil {
		d6SetStatus("error", "Live-Tuning fehlgeschlagen", err.Error())
		return err
	}
	if result.Active {
		d6SetStatus("good", "Live übernommen", fmt.Sprintf("%s · Engine %s · FFB-Preset %s · Revision %d", reason, result.EngineProfile, result.FFBPreset, result.Revision))
	} else {
		d6SetStatus("info", "Gespeichert", reason+" · wird beim nächsten Game-FFB-Start wirksam")
	}
	return nil
}

func d6CommitDraft(s system.State, changed d6SliderKind) error {
	if !d6Draft.valid {
		return fmt.Errorf("kein Wheel ausgewählt")
	}
	if changed == d6TestStrength {
		return nil
	}
	if !d62SliderEnabled(changed) {
		return fmt.Errorf("%s ist mit dem aktuellen Adapterpfad nicht verfügbar", d62SliderBadge(changed))
	}
	if changed <= d6Rotation {
		p := d6Draft.profile
		p.Name = "Custom"
		if err := system.SaveNativeEngineProfile(s.DataDir, s.SelectedWheelID, p); err != nil {
			return err
		}
		if changed == d6Rotation && getUISettings().NativeWheelOutput && system.HasActionableSelectedWheel(s) {
			// Range is a real hardware command and therefore keeps the stricter
			// transactional path. It may neutralize an active motor session.
			if err := system.ApplyNativeEngineProfile(s, p.Name); err != nil {
				return err
			}
			d6SetStatus("good", "Lenkwinkel angewendet", fmt.Sprintf("%d° wurden an das Wheel geschrieben", p.RotationDegrees))
			return nil
		}
		if err := system.SetActiveNativeEngineProfile(s.DataDir, s.SelectedWheelID, p.Name); err != nil {
			return err
		}
		if changed == d6Rotation {
			d6SetStatus("warn", "Lenkwinkel nur gespeichert", "Native Output ist aus; der Hardware-Lenkwinkel wird erst bei einer späteren sicheren Anwendung geschrieben")
			return nil
		}
		return d62RefreshLiveTuning(s, "Wheel-Gain geändert")
	}

	a := d6Draft.advanced
	a.Preset = "Custom"
	if err := system.SaveSelectedAdvancedFFBConfig(s, a); err != nil {
		return err
	}
	return d62RefreshLiveTuning(s, "Signalformung geändert")
}

func d6OnMouseUp(x, y int32, s system.State) bool {
	if d6SliderDrag >= 0 {
		idx := d6SliderDrag
		r := d6SliderRects[idx]
		d6SetSliderValue(d6SliderKind(idx), d6SliderValueFromX(d6SliderKind(idx), r, x))
		d6SliderDrag = -1
		releaseCapture()
		if err := d6CommitDraft(s, d6SliderKind(idx)); err != nil {
			queueNotice(err.Error(), "Force Feedback", MB_OK|MB_ICONERROR)
		} else {
			label, _, _, _, display := d6SliderMeta(d6SliderKind(idx))
			setActionFeedback(label + " gespeichert: " + display)
		}
		invalidate(mainWnd)
		return true
	}
	if d6PressedKind == "" {
		return false
	}
	kind, idx := d6PressedKind, d6PressedIndex
	d6PressedKind, d6PressedIndex = "", -1
	releaseCapture()
	if kind == "tab" {
		if hit := wheelSubtabHitTest(x, y); hit == idx {
			return d6HandleClick(x, y, s)
		}
		invalidate(mainWnd)
		return true
	}
	hitKind, hitIdx := d6FFBHitTest(x, y)
	if hitKind == kind && hitIdx == idx {
		return d6HandleClick(x, y, s)
	}
	invalidate(mainWnd)
	return true
}

func d6HandleClick(x, y int32, s system.State) bool {
	if tab := wheelSubtabHitTest(x, y); tab >= 0 {
		setKeyboardFocusID(d6FocusTabLive + tab)
		wheelSubtab = tab
		contentScroll = 0
		if tab == wheelSubtabFFB {
			wheelAdvancedView = false
		}
		labels := map[int]string{
			wheelSubtabLive:        "Live-Ansicht geöffnet.",
			wheelSubtabFFB:         "Force-Feedback-Einstellungen geöffnet.",
			wheelSubtabCalibration: "Kalibrierung geöffnet.",
			wheelSubtabProfiles:    "Profile geöffnet.",
			wheelSubtabDevice:      "Geräteinformationen geöffnet.",
		}
		setActionFeedback(labels[tab])
		invalidate(mainWnd)
		return true
	}
	kind, idx := d6FFBHitTest(x, y)
	if kind == "" {
		return false
	}
	switch kind {
	case "view":
		if idx >= 0 && idx < d62ViewCount {
			d62ActiveView = idx
			contentScroll = 0
			setKeyboardFocusID(d62FocusViewBase + idx)
			setActionFeedback([]string{"FFB Basis geöffnet.", "FFB Effekte geöffnet.", "FFB Signalformung geöffnet.", "FFB Live & Test geöffnet."}[idx])
		}
	case "toggle":
		setKeyboardFocusID(d6FocusToggle)
		d6Draft.advanced.Enabled = !d6Draft.advanced.Enabled
		d6Draft.advanced.Preset = "Custom"
		if err := system.SaveSelectedAdvancedFFBConfig(s, d6Draft.advanced); err != nil {
			queueNotice(err.Error(), "Force Feedback", MB_OK|MB_ICONERROR)
		} else if err := d62RefreshLiveTuning(s, "FFB-Pipeline umgeschaltet"); err != nil {
			queueNotice(err.Error(), "Force Feedback", MB_OK|MB_ICONERROR)
		} else {
			setActionFeedback(map[bool]string{true: "FFB-Pipeline aktiviert.", false: "FFB-Signalformung deaktiviert."}[d6Draft.advanced.Enabled])
		}
	case "native-output":
		setKeyboardFocusID(d6FocusNativeOutput)
		before := getUISettings().NativeWheelOutput
		toggleSetting(4)
		after := getUISettings().NativeWheelOutput
		if before != after {
			if after {
				if _, err := d6ProbeNativeWriter(s, true); err != nil {
					setActionFeedback("Native Output aktiv, HID-Writer aber noch blockiert.")
				} else {
					setActionFeedback("Native Output aktiv · HID-Writer geprüft.")
				}
			} else {
				d6SetStatus("info", "Native Output aus", "Motor-Ausgabe wurde neutralisiert und deaktiviert.")
			}
		}
	case "probe":
		setKeyboardFocusID(d6FocusProbe)
		result, err := d6ProbeNativeWriter(s, false)
		if err != nil {
			setActionFeedback("HID-Writer blockiert – Details im Control Panel.")
		} else if result.ShareMode == "shared-g27-session" {
			setActionFeedback("G27 Unified HID bereit – Input und Output teilen dieselbe C29B-Session.")
		} else if result.ShareMode == "shared-rw" {
			setActionFeedback("HID-Writer bereit (Geteilter HID-Writer).")
		} else {
			setActionFeedback("HID-Writer bereit.")
		}
	case "profile":
		setKeyboardFocusID(d6FocusProfileBase + idx)
		names := []string{"Gentle", "Balanced", "Direct"}
		p, ok := system.ReadNativeEngineProfile(s.DataDir, s.SelectedWheelID, names[idx])
		if !ok {
			return true
		}
		current := system.ReadActiveNativeEngineProfile(s.DataDir, s.SelectedWheelID)
		var err error
		if p.RotationDegrees != current.RotationDegrees && getUISettings().NativeWheelOutput && system.HasActionableSelectedWheel(s) {
			err = system.ApplyNativeEngineProfile(s, p.Name)
		} else {
			err = system.SetActiveNativeEngineProfile(s.DataDir, s.SelectedWheelID, p.Name)
		}
		if err != nil {
			queueNotice(err.Error(), "Wheel-Profil", MB_OK|MB_ICONERROR)
		} else {
			d6Draft.profile = p
			if liveErr := d62RefreshLiveTuning(s, "Wheel-Profil "+p.Name); liveErr != nil {
				queueNotice(liveErr.Error(), "Wheel-Profil", MB_OK|MB_ICONERROR)
			}
			setActionFeedback("Wheel-Profil aktiviert: " + p.Name)
		}
	case "shape":
		setKeyboardFocusID(d6FocusShapeBase + idx)
		presets, _ := system.SelectedAdvancedFFBPresets(s)
		// Builtins are Neutral, Smooth, Responsive, Compensated, Disabled.
		if idx >= 0 && idx < 4 && idx < len(presets) {
			c := presets[idx].Config
			if err := system.SaveSelectedAdvancedFFBConfig(s, c); err != nil {
				queueNotice(err.Error(), "FFB-Signalformung", MB_OK|MB_ICONERROR)
			} else {
				d6Draft.advanced = c
				if liveErr := d62RefreshLiveTuning(s, "FFB-Preset "+presets[idx].Name); liveErr != nil {
					queueNotice(liveErr.Error(), "FFB-Signalformung", MB_OK|MB_ICONERROR)
				}
				setActionFeedback("FFB-Preset aktiviert: " + presets[idx].Name)
			}
		}
	case "test":
		setKeyboardFocusID(d6FocusTestBase + idx)
		if !getUISettings().NativeWheelOutput && idx != 5 {
			d6SetStatus("warn", "Native Output aus", "Aktiviere die Sicherheitsfreigabe oben im Control Panel. Einstellungen bleiben trotzdem editierbar.")
			setActionFeedback("FFB-Test blockiert: Native Output ist aus.")
			invalidate(mainWnd)
			return true
		}
		strength := d6Draft.testStrength
		var err error
		if idx != 5 {
			name := []string{"Constant", "Spring", "Damper", "Friction", "Autocenter"}[idx]
			d6SetStatus("info", name+" wird gestartet", "Vorherige Ausgabe wird neutralisiert, danach werden HID und OutputLease neu geprüft.")
			setActionFeedback(name + "-Test wird vorbereitet …")
			invalidate(mainWnd)
			// Every live-test button is a replace-current-test action. D6.0-D6.2
			// could leave an autocenter/condition lease active and make the next
			// button appear dead with an ownership error. Neutralize first.
			if stopErr := system.NativeOutputEmergencyStop(s); stopErr != nil {
				d6SetStatus("error", "Vorheriger FFB-Test nicht beendet", stopErr.Error())
				setActionFeedback("FFB-Test blockiert: vorherige Ausgabe nicht sicher neutralisiert.")
				invalidate(mainWnd)
				return true
			}
			if _, probeErr := d6ProbeNativeWriter(s, true); probeErr != nil {
				setActionFeedback("FFB-Test nicht gestartet: HID-Writer blockiert.")
				invalidate(mainWnd)
				return true
			}
		}
		switch idx {
		case 0:
			err = system.StartNativeConstantForceTest(s, strength)
		case 1:
			err = system.StartNativeSpringTest(s, strength)
		case 2:
			err = system.StartNativeDamperTest(s, strength)
		case 3:
			err = system.StartNativeFrictionTest(s, strength)
		case 4:
			err = system.NativeOutputTestAutocenter(s, strength, 2)
		case 5:
			err = system.NativeOutputEmergencyStop(s)
		}
		if err != nil {
			d6SetStatus("error", "FFB-Test fehlgeschlagen", err.Error())
			setActionFeedback("FFB-Test fehlgeschlagen – Details im Control Panel.")
		} else if idx == 5 {
			d6SetStatus("good", "Neutralisiert", "Emergency Neutralize wurde über den zentralen Output-Pfad ausgeführt.")
			setActionFeedback("Emergency Neutralize ausgeführt.")
		} else {
			name := []string{"Constant", "Spring", "Damper", "Friction", "Autocenter"}[idx]
			metrics := system.NativeHIDTransportMetricsSnapshot()
			share := d6HIDShareLabel(metrics.LastShareMode)
			d6SetStatus("good", name+" läuft", fmt.Sprintf("%d %% direkte Teststärke · HID %s · max. 3 s · Emergency Stop aktiv", strength, share))
			setActionFeedback("Begrenzter " + name + "-Test gestartet.")
		}
	}
	invalidate(mainWnd)
	return true
}

func d6FocusOrder() []int {
	if currentPage != pageWheel {
		return nil
	}
	out := []int{d6FocusTabLive, d6FocusTabFFB, d6FocusTabCalibration, d6FocusTabProfiles, d6FocusTabDevice}
	if wheelSubtab != wheelSubtabFFB {
		return out
	}
	out = append(out, d6FocusNativeOutput, d6FocusToggle)
	for i := 0; i < d62ViewCount; i++ {
		out = append(out, d62FocusViewBase+i)
	}
	switch d62ActiveView {
	case d62ViewBasis:
		for i := 0; i < 3; i++ {
			out = append(out, d6FocusProfileBase+i)
		}
		out = append(out, d6FocusSliderBase+int(d6Master), d6FocusSliderBase+int(d6Rotation))
	case d62ViewEffects:
		for _, k := range []d6SliderKind{d6Constant, d6Spring, d6Damper, d6Friction} {
			out = append(out, d6FocusSliderBase+int(k))
		}
	case d62ViewSignal:
		for i := 0; i < 4; i++ {
			out = append(out, d6FocusShapeBase+i)
		}
		for _, k := range []d6SliderKind{d6GameConstant, d6Deadband, d6MinimumForce, d6Response, d6LowPass, d6Smoothing, d6OutputLimit} {
			out = append(out, d6FocusSliderBase+int(k))
		}
	case d62ViewLiveTest:
		out = append(out, d6FocusSliderBase+int(d6TestStrength), d6FocusProbe)
		for i := 0; i < 6; i++ {
			out = append(out, d6FocusTestBase+i)
		}
	}
	return out
}

func d6FocusIsSlider(f int) (d6SliderKind, bool) {
	i := f - d6FocusSliderBase
	return d6SliderKind(i), i >= 0 && i < int(d6SliderCount)
}

func d6ActivateFocus(s system.State) bool {
	f := keyboardFocus
	switch {
	case f == d6FocusTabLive:
		wheelSubtab = wheelSubtabLive
		contentScroll = 0
		invalidate(mainWnd)
		return true
	case f >= d6FocusTabLive && f <= d6FocusTabDevice:
		wheelSubtab = f - d6FocusTabLive
		if wheelSubtab != wheelSubtabLive {
			wheelAdvancedView = false
		}
		contentScroll = 0
		invalidate(mainWnd)
		return true
	case f >= d62FocusViewBase && f < d62FocusViewBase+d62ViewCount:
		i := f - d62FocusViewBase
		if wheelSubtab == wheelSubtabFFB && i >= 0 && i < d62ViewCount {
			d62ActiveView = i
			contentScroll = 0
			invalidate(mainWnd)
			return true
		}
	case f == d6FocusNativeOutput:
		if wheelSubtab == wheelSubtabFFB {
			return d6HandleClick((d6NativeOutputRect.Left+d6NativeOutputRect.Right)/2, (d6NativeOutputRect.Top+d6NativeOutputRect.Bottom)/2, s)
		}
	case f == d6FocusToggle:
		if wheelSubtab == wheelSubtabFFB {
			return d6HandleClick((d6ToggleRect.Left+d6ToggleRect.Right)/2, (d6ToggleRect.Top+d6ToggleRect.Bottom)/2, s)
		}
	case f == d6FocusProbe:
		if wheelSubtab == wheelSubtabFFB {
			return d6HandleClick((d6ProbeRect.Left+d6ProbeRect.Right)/2, (d6ProbeRect.Top+d6ProbeRect.Bottom)/2, s)
		}
	case f >= d6FocusProfileBase && f < d6FocusProfileBase+3:
		i := f - d6FocusProfileBase
		r := d6ProfileRects[i]
		return d6HandleClick((r.Left+r.Right)/2, (r.Top+r.Bottom)/2, s)
	case f >= d6FocusShapeBase && f < d6FocusShapeBase+4:
		i := f - d6FocusShapeBase
		r := d6ShapeRects[i]
		return d6HandleClick((r.Left+r.Right)/2, (r.Top+r.Bottom)/2, s)
	case f >= d6FocusTestBase && f < d6FocusTestBase+6:
		i := f - d6FocusTestBase
		r := d6TestRects[i]
		return d6HandleClick((r.Left+r.Right)/2, (r.Top+r.Bottom)/2, s)
	}
	return false
}

func d6AdjustFocusedSlider(delta int, s system.State) bool {
	kind, ok := d6FocusIsSlider(keyboardFocus)
	if !ok || wheelSubtab != wheelSubtabFFB || !d62SliderEnabled(kind) {
		return false
	}
	_, minV, maxV, v, _ := d6SliderMeta(kind)
	step := 1
	if kind == d6LowPass {
		step = 5
	}
	if kind == d6Rotation {
		step = 1
	}
	v += delta * step
	if v < minV {
		v = minV
	}
	if v > maxV {
		v = maxV
	}
	d6SetSliderValue(kind, v)
	if err := d6CommitDraft(s, kind); err != nil {
		queueNotice(err.Error(), "Force Feedback", MB_OK|MB_ICONERROR)
	}
	invalidate(mainWnd)
	return true
}

func d6EnsureFocusVisible(focusID int) {
	if mainWnd == 0 || currentPage != pageWheel || wheelSubtab != wheelSubtabFFB {
		return
	}
	var target RECT
	if k, ok := d6FocusIsSlider(focusID); ok {
		target = d6SliderRects[k]
	} else if focusID >= d62FocusViewBase && focusID < d62FocusViewBase+d62ViewCount {
		target = d62ViewRects[focusID-d62FocusViewBase]
	} else if focusID == d6FocusNativeOutput {
		target = d6NativeOutputRect
	} else if focusID == d6FocusToggle {
		target = d6ToggleRect
	} else if focusID == d6FocusProbe {
		target = d6ProbeRect
	} else if focusID >= d6FocusProfileBase && focusID < d6FocusProfileBase+3 {
		target = d6ProfileRects[focusID-d6FocusProfileBase]
	} else if focusID >= d6FocusShapeBase && focusID < d6FocusShapeBase+4 {
		target = d6ShapeRects[focusID-d6FocusShapeBase]
	} else if focusID >= d6FocusTestBase && focusID < d6FocusTestBase+6 {
		target = d6TestRects[focusID-d6FocusTestBase]
	} else {
		return
	}
	if !rectUsable(target) {
		return
	}
	// Rects are already painted with current scroll. Keep focused item in viewport.
	// Adjust by one viewport delta at a time; next repaint refreshes rects.
	var rc RECT
	pGetClientRect.Call(uintptr(mainWnd), uintptr(unsafe.Pointer(&rc)))
	visibleTop := contentRectFor(rc).Top + 98
	visibleBottom := contentRectFor(rc).Bottom - 16
	if target.Top < visibleTop {
		contentScroll -= visibleTop - target.Top
	} else if target.Bottom > visibleBottom {
		contentScroll += target.Bottom - visibleBottom
	}
	if contentScroll < 0 {
		contentScroll = 0
	}
	if contentScroll > contentScrollMax {
		contentScroll = contentScrollMax
	}
}
