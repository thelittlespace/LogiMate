//go:build windows

package app

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"unsafe"

	"github.com/thelittlespace/LogiMate/internal/system"
)

const themePickerHeight int32 = 104

var (
	themeBackground = rgb(18, 20, 24)
	themeSidebar    = rgb(14, 16, 20)
	colBorder       = rgb(58, 64, 74)
	colTrack        = rgb(53, 58, 67)

	// Theme-role colors. Keeping interaction states as roles instead of hardcoded
	// black/white blends makes all three visual modes internally consistent.
	colOnAccent      = rgb(8, 20, 30)
	colPressed       = rgb(31, 35, 42)
	colToggleOff     = rgb(76, 84, 96)
	colToggleKnobOn  = rgb(246, 249, 253)
	colToggleKnobOff = rgb(205, 211, 220)
	colBadge         = rgb(35, 39, 47)
	colInfoChip      = rgb(40, 79, 116)
	colFocusRing     = rgb(124, 193, 255)

	fontSection HFONT
	fontMetric  HFONT
	fontLabel   HFONT

	themeRects   [4]RECT
	hoveredTheme = -1

	liveJoyMu sync.RWMutex
	liveJoy   system.JoyState
)

type themeChoice struct {
	id, label, desc string
}

type themePalette struct {
	background, sidebar                     uintptr
	text, muted, muted2                     uintptr
	accent, accentSoft, onAccent            uintptr
	panel, panel2, hover, selected, pressed uintptr
	warning, warningSurface, good, bad      uintptr
	border, track                           uintptr
	toggleOff, toggleKnobOn, toggleKnobOff  uintptr
	badge, infoChip, focusRing              uintptr
}

var themeChoices = []themeChoice{
	{"dark", "Dunkel", "Tief · kontrastreich"},
	{"gray", "Grau", "Graphit · neutral"},
	{"light", "Hell", "Fluent · klar"},
	{"system", "System", "Folgt Windows"},
}

func normalizeThemeMode(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "gray", "grey", "grau":
		return "gray"
	case "light", "hell":
		return "light"
	case "system", "windows":
		return "system"
	default:
		return "dark"
	}
}

func effectiveThemeMode(mode string) string {
	mode = normalizeThemeMode(mode)
	if mode == "system" {
		if system.WindowsAppsUseLightTheme() {
			return "light"
		}
		return "dark"
	}
	return mode
}

func themeUsesDarkChrome() bool { return effectiveThemeMode(getUISettings().ThemeMode) != "light" }

func materialBackdropForTheme(mode string) (int32, string) {
	switch effectiveThemeMode(mode) {
	case "gray":
		return 4, "Mica Alt" // DWMSBT_TABBEDWINDOW
	case "light":
		return 2, "Mica" // DWMSBT_MAINWINDOW
	default:
		return 3, "Desktop Acrylic" // DWMSBT_TRANSIENTWINDOW
	}
}

func paletteFor(mode string) themePalette {
	switch effectiveThemeMode(mode) {
	case "gray":
		// Deliberately neutral graphite: substantially less blue than Dark, with
		// a desaturated steel accent so Gray reads as its own mode rather than a
		// brighter copy of Dark.
		return themePalette{
			background: rgb(48, 49, 52), sidebar: rgb(39, 40, 43),
			text: rgb(245, 245, 246), muted: rgb(196, 197, 201), muted2: rgb(176, 178, 184),
			accent: rgb(160, 188, 211), accentSoft: rgb(78, 91, 102), onAccent: rgb(24, 28, 31),
			panel: rgb(58, 59, 62), panel2: rgb(68, 69, 73), hover: rgb(79, 80, 85), selected: rgb(83, 91, 98), pressed: rgb(63, 64, 68),
			warning: rgb(255, 205, 120), warningSurface: rgb(116, 90, 54), good: rgb(119, 194, 151), bad: rgb(246, 152, 154),
			border: rgb(96, 98, 104), track: rgb(82, 84, 89),
			toggleOff: rgb(103, 106, 113), toggleKnobOn: rgb(248, 248, 249), toggleKnobOff: rgb(204, 205, 209),
			badge: rgb(67, 68, 72), infoChip: rgb(78, 91, 102), focusRing: rgb(192, 209, 222),
		}
	case "light":
		// Real light palette: white elevated surfaces, neutral cool canvas, dark
		// typography and interaction states designed to become darker on hover /
		// press rather than mixing against black ad-hoc.
		return themePalette{
			background: rgb(245, 247, 250), sidebar: rgb(236, 240, 245),
			text: rgb(31, 35, 42), muted: rgb(82, 91, 103), muted2: rgb(92, 103, 118),
			accent: rgb(0, 103, 192), accentSoft: rgb(216, 235, 249), onAccent: rgb(255, 255, 255),
			panel: rgb(255, 255, 255), panel2: rgb(247, 249, 251), hover: rgb(235, 241, 247), selected: rgb(224, 238, 250), pressed: rgb(218, 227, 236),
			warning: rgb(145, 83, 0), warningSurface: rgb(255, 242, 213), good: rgb(14, 122, 69), bad: rgb(180, 48, 52),
			border: rgb(211, 218, 226), track: rgb(222, 227, 233),
			toggleOff: rgb(174, 184, 196), toggleKnobOn: rgb(255, 255, 255), toggleKnobOff: rgb(247, 249, 252),
			badge: rgb(249, 251, 253), infoChip: rgb(224, 238, 250), focusRing: rgb(0, 103, 192),
		}
	default:
		return themePalette{
			background: rgb(18, 20, 24), sidebar: rgb(14, 16, 20),
			text: rgb(240, 243, 248), muted: rgb(159, 167, 180), muted2: rgb(132, 142, 158),
			accent: rgb(92, 174, 255), accentSoft: rgb(40, 79, 116), onAccent: rgb(8, 20, 30),
			panel: rgb(29, 32, 38), panel2: rgb(35, 39, 47), hover: rgb(43, 48, 57), selected: rgb(37, 68, 96), pressed: rgb(31, 35, 42),
			warning: rgb(255, 190, 92), warningSurface: rgb(107, 73, 31), good: rgb(96, 205, 145), bad: rgb(242, 113, 116),
			border: rgb(58, 64, 74), track: rgb(53, 58, 67),
			toggleOff: rgb(76, 84, 96), toggleKnobOn: rgb(246, 249, 253), toggleKnobOff: rgb(205, 211, 220),
			badge: rgb(35, 39, 47), infoChip: rgb(40, 79, 116), focusRing: rgb(124, 193, 255),
		}
	}
}

func colorChannelLinear(v byte) float64 {
	x := float64(v) / 255.0
	if x <= 0.04045 {
		return x / 12.92
	}
	return math.Pow((x+0.055)/1.055, 2.4)
}

func colorLuminance(c uintptr) float64 {
	r := byte(c & 0xff)
	g := byte((c >> 8) & 0xff)
	b := byte((c >> 16) & 0xff)
	return 0.2126*colorChannelLinear(r) + 0.7152*colorChannelLinear(g) + 0.0722*colorChannelLinear(b)
}

func colorContrastRatio(a, b uintptr) float64 {
	la, lb := colorLuminance(a), colorLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// readableTextColor keeps decorative/project accents usable in all themes.
// Several cards intentionally use fixed brand colors (purple, pink, orange).
// Those are attractive on Dark but can become almost invisible on Light/Gray.
// The border/stripe keeps the original hue; text is blended toward the theme's
// primary text color until it reaches WCAG AA normal-text contrast.
func readableTextColor(fg, bg uintptr) uintptr {
	if highContrastEnabled() {
		return colText
	}
	if colorContrastRatio(fg, bg) >= 4.5 {
		return fg
	}
	for pct := 12; pct <= 100; pct += 8 {
		candidate := blendColor(fg, colText, pct)
		if colorContrastRatio(candidate, bg) >= 4.5 {
			return candidate
		}
	}
	return colText
}

func readableAccentOnPanel(accent uintptr) uintptr {
	return readableTextColor(accent, colPanel)
}

func readableAccentOnPanel2(accent uintptr) uintptr {
	return readableTextColor(accent, colPanel2)
}

func setThemeColors(mode string) {
	if highContrastEnabled() {
		bg, text := systemColor(COLOR_WINDOW), systemColor(COLOR_WINDOWTEXT)
		accent, onAccent := systemColor(COLOR_HIGHLIGHT), systemColor(COLOR_HIGHLIGHTTEXT)
		themeBackground, themeSidebar = bg, bg
		colText, colMuted, colMuted2 = text, text, text
		colAccent, colAccentSoft, colOnAccent = accent, accent, onAccent
		colPanel, colPanel2, colHover, colSelected, colPressed = bg, bg, systemColor(COLOR_BTNFACE), accent, systemColor(COLOR_BTNFACE)
		colWarning, colWarningSurface, colGood, colBad = accent, systemColor(COLOR_BTNFACE), accent, accent
		colBorder, colTrack = text, text
		colToggleOff, colToggleKnobOn, colToggleKnobOff = systemColor(COLOR_BTNFACE), onAccent, text
		colBadge, colInfoChip, colFocusRing = bg, accent, accent
		return
	}
	p := paletteFor(mode)
	themeBackground, themeSidebar = p.background, p.sidebar
	colText, colMuted, colMuted2 = p.text, p.muted, p.muted2
	colAccent, colAccentSoft, colOnAccent = p.accent, p.accentSoft, p.onAccent
	colPanel, colPanel2, colHover, colSelected, colPressed = p.panel, p.panel2, p.hover, p.selected, p.pressed
	colWarning, colWarningSurface, colGood, colBad = p.warning, p.warningSurface, p.good, p.bad
	colBorder, colTrack = p.border, p.track
	colToggleOff, colToggleKnobOn, colToggleKnobOff = p.toggleOff, p.toggleKnobOn, p.toggleKnobOff
	colBadge, colInfoChip, colFocusRing = p.badge, p.infoChip, p.focusRing
}

func rebuildThemeResources() {
	setThemeColors(getUISettings().ThemeMode)
	for _, b := range roundBrushes {
		deleteObject(uintptr(b))
	}
	for _, p := range roundPens {
		deleteObject(p)
	}
	roundBrushes = map[uintptr]HBRUSH{}
	roundPens = map[uintptr]uintptr{}
	deleteObject(uintptr(bgBrush))
	deleteObject(uintptr(sidebarBrush))
	bgBrush = createSolidBrush(themeBackground)
	sidebarBrush = createSolidBrush(themeSidebar)

	// Theme transitions are treated as a material transition too. Resetting the
	// old backdrop first avoids Light cards being painted over a stale Dark
	// Acrylic composition (and vice versa) while DWM catches up.
	if mainWnd != 0 {
		resetWindowMaterial(mainWnd)
		glassActive = false
		materialStarted = false
		applyWindowChromeTheme(mainWnd)
		if !safeUI && getUISettings().Acrylic {
			startWindowMaterialAsync(mainWnd)
		}
		invalidate(mainWnd)
	}
	if changelogWnd != 0 {
		resetWindowMaterial(changelogWnd)
		changelogGlassActive = false
		applyWindowChromeTheme(changelogWnd)
		if !safeUI && getUISettings().Acrylic {
			changelogGlassActive = applyWindowMaterial(changelogWnd)
		}
		invalidate(changelogWnd)
	}
	if setupWnd != 0 {
		resetWindowMaterial(setupWnd)
		setupGlassActive = false
		applyWindowChromeTheme(setupWnd)
		if !safeUI && getUISettings().Acrylic {
			setupGlassActive = applyWindowMaterial(setupWnd)
		}
		invalidate(setupWnd)
	}
}

func setThemeMode(mode string) {
	p := getUISettings()
	mode = normalizeThemeMode(mode)
	if p.ThemeMode == mode {
		return
	}
	p.ThemeMode = mode
	if err := replaceUISettings(p); err != nil {
		queueNotice(err.Error(), "Darstellung", MB_OK|MB_ICONERROR)
		return
	}
	rebuildThemeResources()
}

func themeHitTest(x, y int32) int {
	if currentPage != pageSettings || mainWnd == 0 {
		return -1
	}
	var rc RECT
	pGetClientRect.Call(uintptr(mainWnd), uintptr(unsafe.Pointer(&rc)))
	content := contentRectFor(rc)
	if y < content.Top+52 || y > content.Bottom-16 {
		return -1
	}
	for i, r := range themeRects {
		if r.Right > r.Left && x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom {
			return i
		}
	}
	return -1
}

func paintThemePicker(hdc uintptr, content RECT) {
	prefs := getUISettings()
	top := content.Top + 55 - contentScroll
	label := RECT{content.Left + 22, top, content.Right - 22, top + 22}
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSection))
	drawText(hdc, "Farbmodus", &label, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	gap := int32(10)
	left := content.Left + 20
	right := content.Right - 20
	w := (right - left - gap*int32(len(themeChoices)-1)) / int32(len(themeChoices))
	cardTop := top + 30
	for i, t := range themeChoices {
		r := RECT{left + int32(i)*(w+gap), cardTop, left + int32(i)*(w+gap) + w, cardTop + 62}
		themeRects[i] = r
		selected := normalizeThemeMode(prefs.ThemeMode) == t.id
		fill := colPanel2
		border := colBorder
		if hoveredTheme == i {
			fill = colHover
		}
		if selected {
			border = colAccent
			fill = blendColor(colSelected, colPanel2, 28)
		}
		if keyboardFocus == focusThemeBase+i {
			border = colFocusRing
		}
		drawRoundRect(hdc, r, 14, fill, border)
		preview := paletteFor(t.id)
		swatch := RECT{r.Left + 12, r.Top + 13, r.Left + 40, r.Top + 41}
		drawRoundRect(hdc, swatch, 8, preview.background, preview.border)
		// A miniature sidebar/surface/accent trio makes the modes visually
		// distinguishable before selection instead of reducing them to one dot.
		fillRoundRect(hdc, RECT{swatch.Left + 3, swatch.Top + 3, swatch.Left + 10, swatch.Bottom - 3}, 3, preview.sidebar)
		fillRoundRect(hdc, RECT{swatch.Left + 12, swatch.Top + 5, swatch.Right - 3, swatch.Bottom - 8}, 4, preview.panel)
		fillRoundRect(hdc, RECT{swatch.Left + 12, swatch.Bottom - 6, swatch.Right - 3, swatch.Bottom - 3}, 2, preview.accent)
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
		tr := RECT{r.Left + 50, r.Top + 8, r.Right - 8, r.Top + 32}
		drawText(hdc, t.label, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		dr := RECT{r.Left + 50, r.Top + 31, r.Right - 8, r.Bottom - 7}
		drawText(hdc, t.desc, &dr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func setLiveJoy(j system.JoyState) { liveJoyMu.Lock(); liveJoy = j; liveJoyMu.Unlock() }
func getLiveJoy() system.JoyState  { liveJoyMu.RLock(); defer liveJoyMu.RUnlock(); return liveJoy }

func axisNorm(v, min, max uint32) float64 {
	if max <= min {
		return .5
	}
	if v <= min {
		return 0
	}
	if v >= max {
		return 1
	}
	return float64(v-min) / float64(max-min)
}

func ellipse(hdc uintptr, r RECT, fill, border uintptr) {
	brush, ok := roundBrushes[fill]
	if !ok || brush == 0 {
		brush = createSolidBrush(fill)
		roundBrushes[fill] = brush
	}
	pen, ok := roundPens[border]
	if !ok || pen == 0 {
		pen, _, _ = pCreatePen.Call(PS_SOLID, 1, border)
		roundPens[border] = pen
	}
	ob, _, _ := pSelectObject.Call(hdc, uintptr(brush))
	op, _, _ := pSelectObject.Call(hdc, pen)
	pEllipse.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom))
	pSelectObject.Call(hdc, ob)
	pSelectObject.Call(hdc, op)
}

func line(hdc uintptr, x1, y1, x2, y2 int32, color uintptr, width int32) {
	key := color ^ uintptr(width)<<32
	pen, ok := roundPens[key]
	if !ok || pen == 0 {
		pen, _, _ = pCreatePen.Call(PS_SOLID, uintptr(width), color)
		roundPens[key] = pen
	}
	old, _, _ := pSelectObject.Call(hdc, pen)
	pMoveToEx.Call(hdc, uintptr(x1), uintptr(y1), 0)
	pLineTo.Call(hdc, uintptr(x2), uintptr(y2))
	pSelectObject.Call(hdc, old)
}

func wheelAttentionCopy(s system.State) (title, detail, icon string, accent uintptr, visible bool) {
	accent = colWarning
	if s.DeviceDetectionError != "" {
		return "Hardware-Erkennung fehlgeschlagen",
			"Windows konnte die Logitech-Geräte nicht zuverlässig auflisten. Die Live-Anzeigen bleiben sichtbar; öffne „Geräte verwalten“ und starte „Neu erkennen“. Den technischen Fehler findest du zusätzlich unter Diagnose.",
			"!", colBad, true
	}
	if len(s.Wheels) > 1 {
		if w, ok := system.SelectedWheel(s); ok {
			return "Mehrere Lenkräder erkannt",
				fmt.Sprintf("%d unterstützte Geräte sind verbunden. Die Live-Anzeigen verwenden aktuell „%s“. Über „Geräte verwalten“ kannst du das Ziel jederzeit wechseln. Treiberwechsel bleiben bei mehreren gleichzeitig angeschlossenen Rädern absichtlich gesperrt.", len(s.Wheels), system.WheelDeviceLabel(w)),
				"↔", colWarning, true
		}
		return "Mehrere Lenkräder erkannt · Auswahl erforderlich",
			fmt.Sprintf("%d unterstützte Geräte sind verbunden. Die Instrumente bleiben sichtbar, zeigen aber erst nach einer bewussten Auswahl Live-Daten. Öffne „Geräte verwalten“ und wähle das gewünschte Lenkrad.", len(s.Wheels)),
			"↔", colWarning, true
	}
	if len(s.Wheels) == 0 {
		return "Kein unterstütztes Lenkrad erkannt",
			"Die Hardwareanzeigen bleiben sichtbar, damit die Seite stabil aufgebaut bleibt. Prüfe USB, Stromversorgung und Kabel und starte danach unter „Geräte verwalten“ eine neue Erkennung.",
			"◉", colWarning, true
	}
	if _, ok := system.SelectedWheel(s); !ok {
		if s.SelectedWheelID != "" {
			return "Ausgewähltes Lenkrad nicht verbunden",
				"LogiMate wechselt nicht automatisch auf ein anderes physisches Rad. Die Instrumente bleiben sichtbar; verbinde das bisherige Gerät erneut oder wähle unter „Geräte verwalten“ bewusst ein anderes Ziel.",
				"↺", colWarning, true
		}
		return "Lenkrad auswählen",
			"Ein unterstütztes Gerät wurde erkannt, aber noch kein Ziel für die Live-Diagnose gewählt. Die Instrumente bleiben sichtbar; öffne „Geräte verwalten“ und wähle das gewünschte Rad.",
			"?", colAccent, true
	}
	if system.IsCompatibilityModel(s.WheelModel) {
		return "C294 erkannt · Modellbestätigung erforderlich",
			"Windows erkennt das Lenkrad im gemeinsamen Logitech-Kompatibilitätsmodus. Live-Diagnose bleibt möglich, aber modellabhängige HID-Kommandos und Treiberwechsel sind gesperrt, bis G25, G27 oder Driving Force GT einmalig für dieses physische Rad bestätigt wurde.",
			"?", colWarning, true
	}
	return "", "", "", colAccent, false
}

func paintWheelAttentionPanel(hdc uintptr, r RECT, s system.State) bool {
	title, detail, icon, accent, visible := wheelAttentionCopy(s)
	if !visible {
		return false
	}
	drawRoundRect(hdc, r, 16, blendColor(accent, colPanel2, 82), blendColor(accent, colBorder, 45))
	chip := RECT{r.Left + 16, r.Top + 15, r.Left + 60, r.Top + 59}
	chipFill := blendColor(accent, colPanel2, 70)
	fillRoundRect(hdc, chip, 14, chipFill)
	pSetTextColor.Call(hdc, readableTextColor(accent, chipFill))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSection))
	drawText(hdc, icon, &chip, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)

	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	tr := RECT{chip.Right + 14, r.Top + 11, r.Right - 18, r.Top + 34}
	drawText(hdc, title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	dr := RECT{chip.Right + 14, r.Top + 34, r.Right - 18, r.Bottom - 10}
	drawFittedParagraph(hdc, detail, dr, colMuted, fontSmall, fontSmall)
	return true
}

func paintWheelDashboard(hdc uintptr, content RECT, s system.State) {
	paintWheelSubtabs(hdc, content)
	switch wheelSubtab {
	case wheelSubtabFFB:
		paintWheelFFBControlPanel(hdc, content, s)
		return
	case wheelSubtabCalibration:
		paintWheelCalibrationPanel(hdc, content, s)
		return
	case wheelSubtabProfiles:
		paintWheelProfilesPanel(hdc, content, s)
		return
	case wheelSubtabDevice:
		paintWheelDevicePanel(hdc, content, s)
		return
	}
	d6ResetRects()
	j := getLiveJoy()
	visibleTop := content.Top + 98
	visibleBottom := content.Bottom - 16
	visibleHeight := visibleBottom - visibleTop
	attentionH := int32(0)
	if _, _, _, _, visible := wheelAttentionCopy(s); visible {
		attentionH = 100
	}
	advancedH := int32(0)
	if wheelAdvancedView {
		advancedH = wheelAdvancedViewHeight(s, j)
	}
	virtualHeight := int32(538) + attentionH + advancedH
	if j.NativeControls {
		virtualHeight = 786 + attentionH + advancedH
	}
	contentScrollMax = virtualHeight - visibleHeight
	if contentScrollMax < 0 {
		contentScrollMax = 0
	}
	if contentScroll > contentScrollMax {
		contentScroll = contentScrollMax
	}
	y0 := content.Top + 101 - contentScroll
	left, right := content.Left+20, content.Right-20
	width := right - left
	gap := int32(12)

	saved, _, _ := pSaveDC.Call(hdc)
	pIntersectClipRect.Call(hdc, uintptr(content.Left+16), uintptr(visibleTop), uintptr(content.Right-14), uintptr(visibleBottom))

	if attentionH > 0 {
		attention := RECT{left, y0, right, y0 + attentionH - gap}
		paintWheelAttentionPanel(hdc, attention, s)
		y0 += attentionH
	}

	steerW := width * 38 / 100
	if steerW < 245 {
		steerW = 245
	}
	steer := RECT{left, y0, left + steerW, y0 + 166}
	info := RECT{steer.Right + gap, y0, right, y0 + 166}
	drawRoundRect(hdc, steer, 16, colPanel2, colBorder)
	drawRoundRect(hdc, info, 16, colPanel2, colBorder)

	// Steering instrument.
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	rr := RECT{steer.Left + 16, steer.Top + 10, steer.Right - 16, steer.Top + 32}
	drawText(hdc, "Lenkung", &rr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pct := .5
	if j.Found {
		pct = axisNorm(j.X, j.XMin, j.XMax)
	}
	angle := (pct - .5) * 900
	if j.SteeringCalibrated {
		angle = j.SteeringDegrees
	}
	size := int32(92)
	cx := steer.Left + 62
	cy := steer.Top + 94
	dial := RECT{cx - size/2, cy - size/2, cx + size/2, cy + size/2}
	ellipse(hdc, dial, blendColor(colPanel, colPanel2, 35), colBorder)
	ellipse(hdc, RECT{cx - 11, cy - 11, cx + 11, cy + 11}, colAccent, colAccent)
	rad := ((pct-.5)*280 - 90) * math.Pi / 180
	ex := cx + int32(math.Cos(rad)*36)
	ey := cy + int32(math.Sin(rad)*36)
	line(hdc, cx, cy, ex, ey, colAccent, 3)
	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontMetric))
	mr := RECT{steer.Left + 124, steer.Top + 52, steer.Right - 14, steer.Top + 92}
	drawText(hdc, fmt.Sprintf("%+.0f°", angle), &mr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	vr := RECT{steer.Left + 126, steer.Top + 92, steer.Right - 12, steer.Top + 122}
	steerDetail := fmt.Sprintf("X  ·  %.1f %%", pct*100)
	if j.SteeringCalibrated {
		steerDetail += fmt.Sprintf("  ·  %d° kalibriert", j.SteeringRangeDegrees)
	}
	drawText(hdc, steerDetail, &vr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	// Device / mapping card.
	status := "Nicht zugeordnet"
	if j.Found {
		status = "Live verbunden"
	}
	pSetTextColor.Call(hdc, map[bool]uintptr{true: colGood, false: colBad}[j.Found])
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	sr := RECT{info.Left + 16, info.Top + 12, info.Right - 16, info.Top + 34}
	drawText(hdc, "●  "+status, &sr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSection))
	nr := RECT{info.Left + 16, info.Top + 38, info.Right - 16, info.Top + 67}
	name := j.Name
	if name == "" {
		if selected, ok := system.SelectedWheel(s); ok {
			name = system.WheelDeviceLabel(selected)
		} else {
			name = compactWheelName(s.WheelModel)
		}
	}
	drawText(hdc, name, &nr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	details := j.Selection
	if details == "" {
		details = j.Error
	}
	if details == "" {
		details = "PnP und WinMM werden unabhängig geprüft"
	}
	if j.ReportRateHz > 0 {
		details += fmt.Sprintf(" · %.1f Hz", j.ReportRateHz)
	}
	if j.LastReportAge > 0 {
		details += fmt.Sprintf(" · letzter Report %.2fs", j.LastReportAge.Seconds())
	}
	if strings.TrimSpace(j.LastInputError) != "" {
		details += " · letzter Fehler: " + j.LastInputError
	}
	dr := RECT{info.Left + 16, info.Top + 72, info.Right - 16, info.Top + 112}
	drawFittedParagraph(hdc, details, dr, colMuted, fontSmall, fontSmall)
	chipGap := int32(6)
	chipW := (info.Right - info.Left - 32 - chipGap*3) / 4
	chips := [4]RECT{}
	for i := 0; i < 4; i++ {
		l := info.Left + 16 + int32(i)*(chipW+chipGap)
		chips[i] = RECT{l, info.Bottom - 40, l + chipW, info.Bottom - 12}
		fillRoundRect(hdc, chips[i], 10, colPanel)
	}
	pSetTextColor.Call(hdc, colMuted)
	drawText(hdc, fmt.Sprintf("%d Achsen", j.NumAxes), &chips[0], DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	drawText(hdc, fmt.Sprintf("%d Tasten", j.NumButtons), &chips[1], DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	rateText := "— Hz"
	if j.ReportRateHz > 0 {
		rateText = fmt.Sprintf("%.0f Hz", j.ReportRateHz)
	}
	drawText(hdc, rateText, &chips[2], DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	drawText(hdc, fmt.Sprintf("↻ %d", j.Reconnects), &chips[3], DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	// Axis instrument card.
	axes := RECT{left, steer.Bottom + gap, right, steer.Bottom + gap + 120}
	drawRoundRect(hdc, axes, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	ar := RECT{axes.Left + 16, axes.Top + 10, axes.Right - 16, axes.Top + 32}
	drawText(hdc, "Live-Achsen", &ar, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	type ax struct {
		name, role  string
		v, min, max uint32
	}
	sharedHID := strings.Contains(strings.ToLower(j.Selection), "direct hid") || strings.Contains(strings.ToLower(j.Selection), "shared hid")
	axisLabel := func(axis string) string {
		if sharedHID {
			switch axis {
			case "Y":
				return "Gas"
			case "Z":
				return "Bremse"
			case "R":
				return "Kupplung"
			}
		}
		switch axis {
		case s.Pedals.Gas:
			return "Gas · " + axis
		case s.Pedals.Brake:
			return "Bremse · " + axis
		case s.Pedals.Clutch:
			return "Kupplung · " + axis
		default:
			return axis
		}
	}
	axv := []ax{{axisLabel("Y"), "gas", j.Y, j.YMin, j.YMax}, {axisLabel("Z"), "brake", j.Z, j.ZMin, j.ZMax}}
	if !(sharedHID && system.IsDFGTModel(s.WheelModel)) {
		axv = append(axv, ax{axisLabel("R"), "clutch", j.R, j.RMin, j.RMax})
	}
	if !sharedHID {
		axv = append(axv, ax{axisLabel("U"), "", j.U, j.UMin, j.UMax}, ax{axisLabel("V"), "", j.V, j.VMin, j.VMax})
	}
	innerL := axes.Left + 18
	innerR := axes.Right - 18
	ag := int32(12)
	aw := (innerR - innerL - ag*int32(len(axv)-1)) / int32(len(axv))
	for i, a := range axv {
		r := RECT{innerL + int32(i)*(aw+ag), axes.Top + 40, innerL + int32(i)*(aw+ag) + aw, axes.Bottom - 12}
		p := 0.0
		if j.Found {
			p = axisNorm(a.v, a.min, a.max)
			if a.role != "" {
				if cp, ok := system.PedalPercent(j, s.Pedals, a.role); ok {
					p = cp
				}
			}
		}
		track := RECT{r.Left, r.Top + 22, r.Right, r.Top + 38}
		fillRoundRect(hdc, track, 8, colPanel)
		fw := int32(float64(track.Right-track.Left) * p)
		if fw < 3 && p > 0 {
			fw = 3
		}
		fillRoundRect(hdc, RECT{track.Left, track.Top, track.Left + fw, track.Bottom}, 8, colAccent)
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
		lr := RECT{r.Left, r.Top, r.Right - 36, r.Top + 20}
		drawText(hdc, a.name, &lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colMuted)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		pr := RECT{r.Right - 35, r.Top, r.Right, r.Top + 20}
		drawText(hdc, fmt.Sprintf("%3.0f%%", p*100), &pr, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
	}

	// Controls. Native classic HID with a learned/known semantic map gets the wheel + H-shifter view; generic
	// WinMM keeps the neutral button/POV matrix because its button numbering can
	// differ between driver generations.
	inputTop := axes.Bottom + gap
	normalBottom := inputTop
	if j.NativeControls {
		wheelControls := RECT{left, inputTop, right, inputTop + 132}
		paintNativeWheelControls(hdc, wheelControls, j)
		shifter := RECT{left, wheelControls.Bottom + gap, right, wheelControls.Bottom + gap + 205}
		paintNativeHShifter(hdc, shifter, j)
		normalBottom = shifter.Bottom
	} else {
		input := RECT{left, inputTop, right, inputTop + 154}
		paintGenericButtonsAndPOV(hdc, input, j)
		normalBottom = input.Bottom
	}

	if wheelAdvancedView {
		paintWheelAdvancedData(hdc, RECT{left, normalBottom + gap, right, normalBottom + gap + advancedH}, s, j)
	}

	if saved != 0 {
		pRestoreDC.Call(hdc, saved)
	}
	paintScrollBar(hdc, content, visibleHeight, virtualHeight)
}

func controlDot(hdc uintptr, r RECT, label string, active bool) {
	fill, border, text := colPanel, colBorder, colMuted
	if active {
		fill, border, text = colAccent, colAccent, colOnAccent
	}
	ellipse(hdc, r, fill, border)
	pSetTextColor.Call(hdc, text)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
	drawText(hdc, label, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}

func paintNativeWheelControls(hdc uintptr, r RECT, j system.JoyState) {
	drawRoundRect(hdc, r, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 32}
	drawText(hdc, "Lenkrad-Tasten", &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	centerY := r.Top + 80
	// Six red wheel buttons in two groups; numbers match the semantic monitor
	// rather than a driver-dependent DirectInput index.
	for i := 0; i < 6; i++ {
		col := i
		x := r.Left + 96 + int32(col)*48
		controlDot(hdc, RECT{x - 15, centerY - 15, x + 15, centerY + 15}, fmt.Sprintf("W%d", i+1), j.WheelButtons[i])
	}
	// Paddles get wide chips so left/right are obvious at a glance.
	for i, p := range []struct {
		label  string
		active bool
	}{{"Wippe L", j.PaddleLeft}, {"Wippe R", j.PaddleRight}} {
		x := r.Right - 220 + int32(i)*102
		cr := RECT{x, centerY - 18, x + 92, centerY + 18}
		fill, border, text := colPanel, colBorder, colMuted
		if p.active {
			fill, border, text = colAccent, colAccent, colOnAccent
		}
		drawRoundRect(hdc, cr, 12, fill, border)
		pSetTextColor.Call(hdc, text)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
		drawText(hdc, p.label, &cr, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func gearLabel(g int) string {
	if g == -1 {
		return "R"
	}
	if g == 0 {
		return "N"
	}
	return fmt.Sprintf("%d", g)
}

func paintNativeHShifter(hdc uintptr, r RECT, j system.JoyState) {
	drawRoundRect(hdc, r, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 32}
	drawText(hdc, "H-Shifter · Gang, D-Pad & 8 Tasten", &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	// H gate.
	gate := RECT{r.Left + 22, r.Top + 44, r.Left + 295, r.Bottom - 18}
	drawRoundRect(hdc, gate, 14, colPanel, colBorder)
	gx1, gx2, gx3 := gate.Left+54, gate.Left+136, gate.Left+218
	gyTop, gyBottom := gate.Top+36, gate.Bottom-36
	line(hdc, gx1, gyTop, gx1, gyBottom, colMuted2, 2)
	line(hdc, gx2, gyTop, gx2, gyBottom, colMuted2, 2)
	line(hdc, gx3, gyTop, gx3, gyBottom, colMuted2, 2)
	line(hdc, gx1, (gyTop+gyBottom)/2, gx3, (gyTop+gyBottom)/2, colMuted2, 2)
	positions := []struct {
		gear int
		x, y int32
	}{
		{1, gx1, gyTop}, {2, gx1, gyBottom}, {3, gx2, gyTop}, {4, gx2, gyBottom}, {5, gx3, gyTop}, {6, gx3, gyBottom}, {-1, gate.Right - 24, gyBottom},
	}
	for _, gp := range positions {
		active := j.Gear == gp.gear
		controlDot(hdc, RECT{gp.x - 16, gp.y - 16, gp.x + 16, gp.y + 16}, gearLabel(gp.gear), active)
	}
	if j.Gear == 0 {
		n := RECT{gx2 - 18, (gyTop+gyBottom)/2 - 13, gx2 + 18, (gyTop+gyBottom)/2 + 13}
		drawRoundRect(hdc, n, 10, colAccent, colAccent)
		pSetTextColor.Call(hdc, colOnAccent)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		drawText(hdc, "N", &n, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
	}

	// Eight shifter buttons.
	buttonsLeft := gate.Right + 24
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		x := buttonsLeft + int32(col)*44
		y := r.Top + 62 + int32(row)*44
		controlDot(hdc, RECT{x - 14, y - 14, x + 14, y + 14}, fmt.Sprintf("S%d", i+1), j.ShifterButtons[i])
	}

	// D-pad compass.
	cx, cy := r.Right-75, r.Bottom-58
	controlDot(hdc, RECT{cx - 13, cy - 45, cx + 13, cy - 19}, "N", j.DPad == 1 || j.DPad == 2 || j.DPad == 8)
	controlDot(hdc, RECT{cx + 20, cy - 13, cx + 46, cy + 13}, "E", j.DPad == 2 || j.DPad == 3 || j.DPad == 4)
	controlDot(hdc, RECT{cx - 13, cy + 19, cx + 13, cy + 45}, "S", j.DPad == 4 || j.DPad == 5 || j.DPad == 6)
	controlDot(hdc, RECT{cx - 46, cy - 13, cx - 20, cy + 13}, "W", j.DPad == 6 || j.DPad == 7 || j.DPad == 8)
	centerFill := colPanel
	if j.DPad == 0 {
		centerFill = colSelected
	}
	ellipse(hdc, RECT{cx - 9, cy - 9, cx + 9, cy + 9}, centerFill, colBorder)
}

func paintGenericButtonsAndPOV(hdc uintptr, input RECT, j system.JoyState) {
	drawRoundRect(hdc, input, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	br := RECT{input.Left + 16, input.Top + 10, input.Right - 16, input.Top + 32}
	drawText(hdc, "Tasten & POV", &br, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	maxButtons := int(j.NumButtons)
	if maxButtons <= 0 {
		maxButtons = 16
	}
	if maxButtons > 24 {
		maxButtons = 24
	}
	gridLeft, gridTop := input.Left+18, input.Top+42
	cell, cg := int32(30), int32(7)
	cols := 8
	for i := 0; i < maxButtons; i++ {
		row, col := i/cols, i%cols
		x := gridLeft + int32(col)*(cell+cg)
		y := gridTop + int32(row)*(cell+cg)
		controlDot(hdc, RECT{x, y, x + cell, y + cell}, fmt.Sprintf("%d", i+1), j.Buttons&(1<<uint(i)) != 0)
	}
	pcx, pcy := input.Right-72, input.Top+91
	pradius := int32(41)
	ellipse(hdc, RECT{pcx - pradius, pcy - pradius, pcx + pradius, pcy + pradius}, colPanel, colBorder)
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	for txt, rr := range map[string]RECT{"N": {pcx - 10, pcy - pradius - 2, pcx + 10, pcy - pradius + 18}, "S": {pcx - 10, pcy + pradius - 18, pcx + 10, pcy + pradius + 2}, "W": {pcx - pradius - 2, pcy - 10, pcx - pradius + 20, pcy + 10}, "E": {pcx + pradius - 20, pcy - 10, pcx + pradius + 2, pcy + 10}} {
		drawText(hdc, txt, &rr, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	}
	pSelectObject.Call(hdc, old)
	if j.Found && j.POV != 0xFFFF && j.POV != 0xFFFFFFFF {
		rad := (float64(j.POV)/100 - 90) * math.Pi / 180
		px, py := pcx+int32(math.Cos(rad)*26), pcy+int32(math.Sin(rad)*26)
		ellipse(hdc, RECT{px - 7, py - 7, px + 7, py + 7}, colAccent, colAccent)
	} else {
		ellipse(hdc, RECT{pcx - 5, pcy - 5, pcx + 5, pcy + 5}, colMuted2, colMuted2)
	}
}

func splitRichBlocks(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	raw := strings.Split(body, "\n\n")
	out := make([]string, 0, len(raw))
	for _, b := range raw {
		if x := strings.TrimSpace(b); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func richIcon(title string) string {
	t := strings.ToLower(title)
	switch {
	case strings.Contains(t, "windows"):
		return "▣"
	case strings.Contains(t, "lenkrad") || strings.Contains(t, "wheel"):
		return "◉"
	case strings.Contains(t, "sicher") || strings.Contains(t, "backup"):
		return "✓"
	case strings.Contains(t, "treiber") || strings.Contains(t, "modus") || strings.Contains(t, "betrieb"):
		return "↔"
	case strings.Contains(t, "empfohlen") || strings.Contains(t, "nächster"):
		return "→"
	case strings.Contains(t, "openg27") || strings.Contains(t, "engine"):
		return "⚡"
	case strings.Contains(t, "markus"):
		return "☺"
	case strings.Contains(t, "logimate"):
		return "◉"
	case strings.Contains(t, "indicana") || strings.Contains(t, "strompilot"):
		return "◇"
	case strings.Contains(t, "green-itea") || strings.Contains(t, "cyclist"):
		return "✦"
	case strings.Contains(t, "sharing is caring"):
		return "♥"
	case strings.Contains(t, "open source") || strings.Contains(t, "credits"):
		return "↗"
	default:
		return "•"
	}
}

func paintRichBody(hdc uintptr, content RECT, body string) {
	blocks := splitRichBlocks(body)
	visibleTop := content.Top + 52
	visibleBottom := content.Bottom - 16
	visibleHeight := visibleBottom - visibleTop
	x1, x2 := content.Left+20, content.Right-20
	y := content.Top + 55 - contentScroll
	gap := int32(10)
	heights := make([]int32, len(blocks))
	// Pre-measure virtual size.
	total := int32(0)
	for i, b := range blocks {
		lines := strings.Split(b, "\n")
		rest := ""
		if len(lines) > 1 {
			rest = strings.Join(lines[1:], "\n")
		}
		h := int32(70)
		if rest != "" {
			mr := RECT{x1 + 52, 0, x2 - 16, 20000}
			old, _, _ := pSelectObject.Call(hdc, uintptr(fontBody))
			measureText(hdc, rest, &mr, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
			pSelectObject.Call(hdc, old)
			h = max32(76, mr.Bottom+54)
		}
		heights[i] = h
		total += h
		if i < len(blocks)-1 {
			total += gap
		}
	}
	contentScrollMax = total - visibleHeight
	if contentScrollMax < 0 {
		contentScrollMax = 0
	}
	if contentScroll > contentScrollMax {
		contentScroll = contentScrollMax
		y = content.Top + 55 - contentScroll
	}
	saved, _, _ := pSaveDC.Call(hdc)
	pIntersectClipRect.Call(hdc, uintptr(content.Left+16), uintptr(visibleTop), uintptr(content.Right-14), uintptr(visibleBottom))
	for i, b := range blocks {
		h := heights[i]
		r := RECT{x1, y, x2, y + h}
		y += h + gap
		if r.Bottom < visibleTop || r.Top > visibleBottom {
			continue
		}
		lines := strings.Split(b, "\n")
		title := strings.TrimSpace(lines[0])
		rest := ""
		if len(lines) > 1 {
			rest = strings.TrimSpace(strings.Join(lines[1:], "\n"))
		}
		border := colBorder
		lowerTitle := strings.ToLower(title)
		if strings.Contains(lowerTitle, "empfohlen") || strings.Contains(lowerTitle, "sharing is caring") || strings.Contains(lowerTitle, "markus kleine") {
			border = colAccent
		}
		drawRoundRect(hdc, r, 15, colPanel2, border)
		chip := RECT{r.Left + 14, r.Top + 14, r.Left + 46, r.Top + 46}
		fillRoundRect(hdc, chip, 10, colInfoChip)
		pSetTextColor.Call(hdc, readableTextColor(colAccent, colInfoChip))
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
		drawText(hdc, richIcon(title), &chip, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSection))
		tr := RECT{r.Left + 58, r.Top + 10, r.Right - 14, r.Top + 38}
		drawText(hdc, title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		if rest != "" {
			pSetTextColor.Call(hdc, colMuted)
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
			br := RECT{r.Left + 58, r.Top + 40, r.Right - 16, r.Bottom - 12}
			drawText(hdc, rest, &br, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
			pSelectObject.Call(hdc, old)
		}
	}
	if saved != 0 {
		pRestoreDC.Call(hdc, saved)
	}
	paintScrollBar(hdc, content, visibleHeight, total)
}

func paintRawInputPanel(hdc uintptr, r RECT, j system.JoyState) {
	panelFill := blendColor(colAccent, colPanel2, 84)
	drawRoundRect(hdc, r, 16, panelFill, blendColor(colAccent, colBorder, 48))
	pSetTextColor.Call(hdc, readableTextColor(colAccent, panelFill))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 31}
	drawText(hdc, "ROHDATENMODUS · LIVE", &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	rate := "—"
	if j.ReportRateHz > 0 {
		rate = fmt.Sprintf("%.1f Hz", j.ReportRateHz)
	}
	age := "—"
	if j.LastReportAge > 0 {
		age = fmt.Sprintf("%.2f s", j.LastReportAge.Seconds())
	}
	status := fmt.Sprintf("%s · letzter Report %s · Reconnects %d", rate, age, j.Reconnects)
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	sr := RECT{r.Left + 16, r.Top + 32, r.Right - 16, r.Top + 52}
	drawText(hdc, status, &sr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

	raw := system.RawReportHex(j)
	if raw == "" {
		raw = "Kein HID-Rohreport · aktueller Pfad liefert WinMM/normalisierte Werte"
	}
	if len(raw) > 112 {
		raw = raw[:112] + "…"
	}
	rr := RECT{r.Left + 16, r.Top + 56, r.Right - 16, r.Top + 80}
	drawText(hdc, raw, &rr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	errText := strings.TrimSpace(j.LastInputError)
	if errText == "" {
		errText = "Letzter HID/WinMM-Fehler: keiner"
	} else {
		errText = "Letzter HID/WinMM-Fehler: " + errText
	}
	er := RECT{r.Left + 16, r.Top + 82, r.Right - 16, r.Bottom - 8}
	drawText(hdc, errText, &er, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}

// D5.6: the wheel page owns its complete raw-data view. This deliberately
// stays inside the existing page instead of opening another diagnostic window,
// so the user can compare semantic controls with the underlying HID/WinMM data
// in one scrolling surface.
type advancedDataRow struct {
	label string
	value string
}

func wheelAdvancedViewHeight(s system.State, j system.JoyState) int32 {
	rawRows := (len(j.RawReport) + 3) / 4
	if rawRows < 1 {
		rawRows = 1
	}
	rawH := int32(102 + rawRows*54)
	hidCount := len(s.HIDCandidates)
	if hidCount < 1 {
		hidCount = 1
	}
	hidH := int32(72 + hidCount*96)
	// Banner + identity + runtime + axes + buttons + semantic controls + raw
	// report + HID interfaces + native output + calibration + physical certification + spacing.
	return 62 + 364 + 12 + 382 + 12 + 226 + 12 + 226 + 12 + 184 + 12 + rawH + 12 + hidH + 12 + 306 + 12 + 250 + 12 + 350 + 18
}

func advancedPOVText(v uint32) string {
	if v == 0xFFFF || v == 0xFFFFFFFF {
		return "Neutral"
	}
	return fmt.Sprintf("%d (%.1f°)", v, float64(v)/100.0)
}

func advancedBool(v bool) string {
	if v {
		return "Ja"
	}
	return "Nein"
}

func advancedValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "—"
	}
	return v
}

func paintAdvancedCard(hdc uintptr, r RECT, title, subtitle string, rows []advancedDataRow) {
	drawRoundRect(hdc, r, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 31}
	drawText(hdc, title, &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	if strings.TrimSpace(subtitle) != "" {
		pSetTextColor.Call(hdc, colMuted)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		sr := RECT{r.Left + 16, r.Top + 31, r.Right - 16, r.Top + 51}
		drawText(hdc, subtitle, &sr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
	if len(rows) == 0 {
		return
	}
	top := r.Top + 58
	available := r.Bottom - top - 10
	rowH := available / int32(len(rows))
	if rowH > 25 {
		rowH = 25
	}
	if rowH < 18 {
		rowH = 18
	}
	labelW := int32(190)
	for i, row := range rows {
		y := top + int32(i)*rowH
		if y+rowH > r.Bottom-5 {
			break
		}
		if i > 0 {
			line(hdc, r.Left+16, y, r.Right-16, y, blendColor(colBorder, colPanel2, 45), 1)
		}
		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		lr := RECT{r.Left + 16, y + 1, r.Left + 16 + labelW, y + rowH - 1}
		drawText(hdc, row.label, &lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSetTextColor.Call(hdc, colText)
		vr := RECT{lr.Right + 10, y + 1, r.Right - 16, y + rowH - 1}
		drawText(hdc, advancedValue(row.value), &vr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func paintAdvancedAxisTable(hdc uintptr, r RECT, j system.JoyState, s system.State) {
	drawRoundRect(hdc, r, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 32}
	drawText(hdc, "Achsen · Rohwerte, Bereiche & Normalisierung", &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	type axisRow struct {
		name        string
		v, min, max uint32
		role        string
	}
	rows := []axisRow{
		{"X / Lenkung", j.X, j.XMin, j.XMax, "steering"},
		{"Y", j.Y, j.YMin, j.YMax, "gas"},
		{"Z", j.Z, j.ZMin, j.ZMax, "brake"},
		{"R", j.R, j.RMin, j.RMax, "clutch"},
		{"U", j.U, j.UMin, j.UMax, ""},
		{"V", j.V, j.VMin, j.VMax, ""},
	}
	cols := []struct {
		title string
		w     int32
	}{{"Achse", 118}, {"Roh", 92}, {"Min", 92}, {"Max", 92}, {"Norm", 90}, {"Semantik / Kalibrierung", 0}}
	x := r.Left + 16
	yHead := r.Top + 43
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	for _, c := range cols {
		w := c.w
		if w == 0 {
			w = r.Right - 16 - x
		}
		cr := RECT{x, yHead, x + w, yHead + 22}
		drawText(hdc, c.title, &cr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		x += w
	}
	rowTop := yHead + 24
	rowH := int32(25)
	for i, a := range rows {
		y := rowTop + int32(i)*rowH
		if y+rowH > r.Bottom-7 {
			break
		}
		if i%2 == 0 {
			fillRoundRect(hdc, RECT{r.Left + 12, y, r.Right - 12, y + rowH - 1}, 6, blendColor(colPanel, colPanel2, 45))
		}
		norm := axisNorm(a.v, a.min, a.max) * 100
		semantic := "—"
		switch a.role {
		case "steering":
			if j.SteeringCalibrated {
				semantic = fmt.Sprintf("%+.2f° · Bereich %d°", j.SteeringDegrees, j.SteeringRangeDegrees)
			} else {
				semantic = "Lenkung · nicht kalibriert"
			}
		case "gas", "brake", "clutch":
			if p, ok := system.PedalPercent(j, s.Pedals, a.role); ok {
				roleName := map[string]string{"gas": "Gas", "brake": "Bremse", "clutch": "Kupplung"}[a.role]
				semantic = fmt.Sprintf("%s · %.2f %%", roleName, p*100)
			}
		}
		vals := []string{a.name, fmt.Sprintf("%d", a.v), fmt.Sprintf("%d", a.min), fmt.Sprintf("%d", a.max), fmt.Sprintf("%.2f %%", norm), semantic}
		x = r.Left + 16
		for ci, c := range cols {
			w := c.w
			if w == 0 {
				w = r.Right - 16 - x
			}
			pSetTextColor.Call(hdc, map[bool]uintptr{true: colText, false: colMuted}[ci == 0 || ci == len(cols)-1])
			cr := RECT{x, y, x + w - 4, y + rowH}
			drawText(hdc, vals[ci], &cr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			x += w
		}
	}
	pSelectObject.Call(hdc, old)
}

func paintAdvancedButtons(hdc uintptr, r RECT, j system.JoyState) {
	drawRoundRect(hdc, r, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 31}
	drawText(hdc, "Buttons · komplette Bitmaske", &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	meta := fmt.Sprintf("HEX 0x%08X   ·   BIN %032b   ·   POV %s   ·   D-Pad %d", j.Buttons, j.Buttons, advancedPOVText(j.POV), j.DPad)
	mr := RECT{r.Left + 16, r.Top + 33, r.Right - 16, r.Top + 55}
	drawText(hdc, meta, &mr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	cols := 8
	gap := int32(7)
	cellW := (r.Right - r.Left - 32 - gap*int32(cols-1)) / int32(cols)
	cellH := int32(31)
	startY := r.Top + 64
	for i := 0; i < 32; i++ {
		row, col := i/cols, i%cols
		x := r.Left + 16 + int32(col)*(cellW+gap)
		y := startY + int32(row)*(cellH+6)
		active := j.Buttons&(1<<uint(i)) != 0
		fill, border, text := colPanel, colBorder, colMuted
		if active {
			fill, border, text = colAccent, colAccent, colOnAccent
		}
		cr := RECT{x, y, x + cellW, y + cellH}
		drawRoundRect(hdc, cr, 9, fill, border)
		pSetTextColor.Call(hdc, text)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		drawText(hdc, fmt.Sprintf("B%02d  %d", i+1, map[bool]int{true: 1, false: 0}[active]), &cr, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
	}
}

func paintAdvancedSemanticControls(hdc uintptr, r RECT, j system.JoyState) {
	drawRoundRect(hdc, r, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 31}
	drawText(hdc, "Semantische Eingaben · LogiMate Mapping", &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	items := []struct {
		label  string
		active bool
	}{{"Wippe L", j.PaddleLeft}, {"Wippe R", j.PaddleRight}}
	for i, v := range j.WheelButtons {
		items = append(items, struct {
			label  string
			active bool
		}{fmt.Sprintf("Wheel %d", i+1), v})
	}
	for i, v := range j.ShifterButtons {
		items = append(items, struct {
			label  string
			active bool
		}{fmt.Sprintf("Shifter %d", i+1), v})
	}
	cols := 8
	gap := int32(7)
	cellW := (r.Right - r.Left - 32 - gap*int32(cols-1)) / int32(cols)
	cellH := int32(31)
	startY := r.Top + 45
	for i, it := range items {
		row, col := i/cols, i%cols
		x := r.Left + 16 + int32(col)*(cellW+gap)
		y := startY + int32(row)*(cellH+6)
		fill, border, text := colPanel, colBorder, colMuted
		if it.active {
			fill, border, text = colAccent, colAccent, colOnAccent
		}
		cr := RECT{x, y, x + cellW, y + cellH}
		drawRoundRect(hdc, cr, 9, fill, border)
		pSetTextColor.Call(hdc, text)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		drawText(hdc, it.label, &cr, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	status := fmt.Sprintf("Gang: %s   ·   D-Pad: %d   ·   NativeControls: %s", gearLabel(j.Gear), j.DPad, advancedBool(j.NativeControls))
	sr := RECT{r.Left + 16, r.Bottom - 29, r.Right - 16, r.Bottom - 8}
	drawText(hdc, status, &sr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}

func paintAdvancedRawReport(hdc uintptr, r RECT, j system.JoyState) {
	panelFill := blendColor(colAccent, colPanel2, 86)
	drawRoundRect(hdc, r, 16, panelFill, blendColor(colAccent, colBorder, 42))
	pSetTextColor.Call(hdc, readableTextColor(colAccent, panelFill))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 31}
	drawText(hdc, "HID-Rohreport · Byte für Byte", &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	hexText := system.RawReportHex(j)
	if hexText == "" {
		hexText = "Kein Direct-HID-Report vorhanden · WinMM liefert keine Rohbytes"
	}
	hr := RECT{r.Left + 16, r.Top + 34, r.Right - 16, r.Top + 57}
	drawText(hdc, "HEX: "+hexText, &hr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	if len(j.RawReport) == 0 {
		return
	}
	cols := 4
	gap := int32(8)
	cellW := (r.Right - r.Left - 32 - gap*int32(cols-1)) / int32(cols)
	cellH := int32(45)
	startY := r.Top + 65
	for i, v := range j.RawReport {
		row, col := i/cols, i%cols
		x := r.Left + 16 + int32(col)*(cellW+gap)
		y := startY + int32(row)*(cellH+8)
		cr := RECT{x, y, x + cellW, y + cellH}
		drawRoundRect(hdc, cr, 9, colPanel, colBorder)
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		line1 := RECT{cr.Left + 8, cr.Top + 4, cr.Right - 8, cr.Top + 21}
		drawText(hdc, fmt.Sprintf("[%02d]   0x%02X   %3d", i, v, v), &line1, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSetTextColor.Call(hdc, colMuted2)
		line2 := RECT{cr.Left + 8, cr.Top + 22, cr.Right - 8, cr.Bottom - 3}
		drawText(hdc, fmt.Sprintf("BIN %08b", v), &line2, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
	}
}

func paintAdvancedHIDInterfaces(hdc uintptr, r RECT, s system.State) {
	drawRoundRect(hdc, r, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 31}
	drawText(hdc, "Direkte HID-Schnittstellen · Geräteerkennung", &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	sub := "Capability-Ranking: höchste Output-Report-Länge zuerst · Daten stammen aus dem Hintergrundscan, nicht aus dem Paint-Pfad."
	sr := RECT{r.Left + 16, r.Top + 32, r.Right - 16, r.Top + 53}
	drawText(hdc, sub, &sr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	if len(s.HIDCandidates) == 0 {
		empty := RECT{r.Left + 16, r.Top + 63, r.Right - 16, r.Bottom - 12}
		drawFittedParagraph(hdc, "Keine unterstützte Logitech-HID-Schnittstelle im letzten Erkennungssnapshot.", empty, colWarning, fontSmall, fontSmall)
		return
	}
	y := r.Top + 61
	for i, c := range s.HIDCandidates {
		cr := RECT{r.Left + 14, y, r.Right - 14, y + 86}
		fillRoundRect(hdc, cr, 11, colPanel)
		product := advancedValue(c.Product)
		pid := fmt.Sprintf("%04X", c.ProductID)
		if c.ProductID == 0 {
			pid = "????"
		}
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		top := RECT{cr.Left + 10, cr.Top + 5, cr.Right - 10, cr.Top + 25}
		drawText(hdc, fmt.Sprintf("%d. VID %04X / PID %s · %s · Probe=%s", i+1, c.VendorID, pid, product, advancedBool(c.ProbeOK)), &top, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSetTextColor.Call(hdc, colMuted2)
		mid := RECT{cr.Left + 10, cr.Top + 25, cr.Right - 10, cr.Top + 45}
		drawText(hdc, fmt.Sprintf("In=%d · Out=%d · Feature=%d · Usage=%04X/%04X · Version=%04X · Serial=%s", c.InputReportLength, c.OutputReportLength, c.FeatureReportLength, c.UsagePage, c.Usage, c.VersionNumber, advancedValue(c.Serial)), &mid, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		path := RECT{cr.Left + 10, cr.Top + 47, cr.Right - 10, cr.Bottom - 5}
		drawFittedParagraph(hdc, advancedValue(c.Path), path, colMuted2, fontSmall, fontSmall)
		pSelectObject.Call(hdc, old)
		y += 96
	}
}

func calibrationSummary(c system.AxisCalibration) string {
	return fmt.Sprintf("Min=%d Max=%d Inv=%s Deadzone=%.1f%% Kurve=%s", c.Min, c.Max, advancedBool(c.Inverted), c.Deadzone*100, advancedValue(c.Curve))
}

func paintWheelAdvancedData(hdc uintptr, r RECT, s system.State, j system.JoyState) {
	gap := int32(12)
	y := r.Top
	banner := RECT{r.Left, y, r.Right, y + 50}
	bannerFill := blendColor(colAccent, colPanel2, 86)
	drawRoundRect(hdc, banner, 15, bannerFill, blendColor(colAccent, colBorder, 45))
	pSetTextColor.Call(hdc, readableTextColor(colAccent, bannerFill))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	bt := RECT{banner.Left + 16, banner.Top + 7, banner.Right - 16, banner.Top + 27}
	drawText(hdc, "ERWEITERTE ANSICHT · LIVE-ROHDATEN", &bt, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSetTextColor.Call(hdc, colMuted)
	old2, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
	bs := RECT{banner.Left + 16, banner.Top + 27, banner.Right - 16, banner.Bottom - 5}
	drawText(hdc, "Alle Werte aktualisieren sich live. Lesen verändert weder Treiber noch Force Feedback.", &bs, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old2)
	pSelectObject.Call(hdc, old)
	y = banner.Bottom + gap

	w, haveWheel := system.SelectedWheel(s)
	deviceRows := []advancedDataRow{
		{"LogiMate Wheel-ID", s.SelectedWheelID},
		{"Session / Container", "—"},
		{"Name", compactWheelName(s.WheelModel)},
		{"Modell / Modus", fmt.Sprintf("%s  /  %s", s.WheelModel, s.ActiveMode)},
		{"Erkennungs-Evidence", s.DetectionEvidence},
		{"Auswahlstatus", s.SelectionStatus},
		{"Instance-ID", "—"},
		{"Interface-IDs", "—"},
		{"Alias-IDs", "—"},
		{"Service / INF", "—"},
		{"PnP / Modell bestätigt", "Nein / Nein"},
		{"Persistent Identity", "Nein"},
		{"Hardware-Fingerprint", "—"},
	}
	if haveWheel {
		deviceRows[1].value = w.SessionID
		deviceRows[2].value = w.Name
		deviceRows[6].value = w.InstanceID
		deviceRows[7].value = strings.Join(w.InterfaceIDs, " | ")
		deviceRows[8].value = strings.Join(w.AliasIDs, " | ")
		deviceRows[9].value = advancedValue(strings.TrimSpace(w.Service + " / " + w.INF))
		deviceRows[10].value = advancedBool(w.PnPVerified) + " / " + advancedBool(w.ModelConfirmed)
		deviceRows[11].value = advancedBool(w.PersistentIdentity)
		deviceRows[12].value = w.HardwareFingerprint
	}
	card := RECT{r.Left, y, r.Right, y + 364}
	paintAdvancedCard(hdc, card, "Gerät & Windows-Identität", "SetupAPI/PnP + HID-Evidence des ausgewählten physischen Wheels", deviceRows)
	y = card.Bottom + gap

	sampleAt := "—"
	if !j.SampleAt.IsZero() {
		sampleAt = j.SampleAt.Format("15:04:05.000")
	}
	runtimeRows := []advancedDataRow{
		{"Found / SampleValid", advancedBool(j.Found) + " / " + advancedBool(j.SampleValid)},
		{"Gerätename", j.Name},
		{"WinMM Joystick-ID", fmt.Sprintf("%d", j.ID)},
		{"Input-Quelle", j.InputSource},
		{"Layout-ID", j.LayoutID},
		{"Zuordnung", j.Selection},
		{"Connection State", j.ConnectionState},
		{"Connected Controller", strings.Join(j.Connected, " | ")},
		{"Wheel-ID / Session", advancedValue(j.WheelID) + " / " + advancedValue(j.SessionID)},
		{"Sample Generation", fmt.Sprintf("%d", j.SampleGeneration)},
		{"Sample-Zeit", sampleAt},
		{"Report-Rate / Alter", fmt.Sprintf("%.2f Hz / %.3f s", j.ReportRateHz, j.LastReportAge.Seconds())},
		{"Reconnects", fmt.Sprintf("%d", j.Reconnects)},
		{"Achsen / Tasten", fmt.Sprintf("%d / %d", j.NumAxes, j.NumButtons)},
		{"Aktueller Fehler", advancedValue(j.Error)},
		{"Letzter Input-Fehler", advancedValue(j.LastInputError)},
	}
	card = RECT{r.Left, y, r.Right, y + 382}
	paintAdvancedCard(hdc, card, "Input Engine · Live Sample", "Direct HID ist bevorzugt; WinMM bleibt Fallback", runtimeRows)
	y = card.Bottom + gap

	card = RECT{r.Left, y, r.Right, y + 226}
	paintAdvancedAxisTable(hdc, card, j, s)
	y = card.Bottom + gap

	card = RECT{r.Left, y, r.Right, y + 226}
	paintAdvancedButtons(hdc, card, j)
	y = card.Bottom + gap

	card = RECT{r.Left, y, r.Right, y + 184}
	paintAdvancedSemanticControls(hdc, card, j)
	y = card.Bottom + gap

	rawRows := (len(j.RawReport) + 3) / 4
	if rawRows < 1 {
		rawRows = 1
	}
	rawH := int32(102 + rawRows*54)
	card = RECT{r.Left, y, r.Right, y + rawH}
	paintAdvancedRawReport(hdc, card, j)
	y = card.Bottom + gap

	hidCount := len(s.HIDCandidates)
	if hidCount < 1 {
		hidCount = 1
	}
	hidH := int32(72 + hidCount*96)
	card = RECT{r.Left, y, r.Right, y + hidH}
	paintAdvancedHIDInterfaces(hdc, card, s)
	y = card.Bottom + gap

	out := system.NativeOutputSnapshot()
	ffb := system.NativeFFBSnapshot()
	lease := system.NativeOutputLeaseSnapshot()
	phase, detail, running := system.NativeActivationStatus()
	outputRows := []advancedDataRow{
		{"Native Output", fmt.Sprintf("aktiv=%s · Wheel=%s · Modell=%s", advancedBool(out.Active), advancedValue(out.WheelID), advancedValue(out.Model))},
		{"Letzter Output-Befehl", advancedValue(out.LastCommand)},
		{"Output Writes / E-Stops", fmt.Sprintf("%d / %d · Watchdog %d", out.CommandCount, out.EmergencyStops, out.WatchdogStops)},
		{"Output-Fehler", advancedValue(out.LastError)},
		{"FFB State", fmt.Sprintf("aktiv=%s stopping=%s faulted=%s · Effekt=%s", advancedBool(ffb.Active), advancedBool(ffb.Stopping), advancedBool(ffb.Faulted), advancedValue(ffb.Effect))},
		{"FFB Requested / Applied", fmt.Sprintf("%d / %d · Frames=%d · Clips=%d · Slew=%d", ffb.Requested, ffb.Applied, ffb.Frames, ffb.ClipEvents, ffb.SlewLimited)},
		{"FFB Conditions", fmt.Sprintf("Spring=%d Damper=%d Friction=%d · Profil=%s", ffb.SpringApplied, ffb.DamperApplied, ffb.FrictionApplied, advancedValue(ffb.ProfileName))},
		{"Output Lease", fmt.Sprintf("aktiv=%s · Gen=%d · Zweck=%s · Motor=%s", advancedBool(lease.Active), lease.Generation, advancedValue(lease.Purpose), advancedBool(lease.Motor))},
		{"Lease Wheel / Session", advancedValue(lease.WheelID) + " / " + advancedValue(lease.SessionID)},
		{"Native-Aktivierung", fmt.Sprintf("Phase=%s · läuft=%s · %s", advancedValue(phase), advancedBool(running), advancedValue(detail))},
	}
	card = RECT{r.Left, y, r.Right, y + 306}
	paintAdvancedCard(hdc, card, "Native Engine · Output / FFB / Lease", "Nur Diagnosewerte; diese Ansicht sendet selbst keine Motorbefehle", outputRows)
	y = card.Bottom + gap

	ped := s.Pedals
	calRows := []advancedDataRow{
		{"Steering", fmt.Sprintf("kalibriert=%s · %+.2f° · Bereich %d°", advancedBool(j.SteeringCalibrated), j.SteeringDegrees, j.SteeringRangeDegrees)},
		{"Pedal Input / Layout", advancedValue(ped.InputSource) + " / " + advancedValue(ped.LayoutID)},
		{"Gas-Achse", advancedValue(ped.Gas) + " · " + calibrationSummary(ped.GasCalibration)},
		{"Bremse-Achse", advancedValue(ped.Brake) + " · " + calibrationSummary(ped.BrakeCalibration)},
		{"Kupplung-Achse", advancedValue(ped.Clutch) + " · " + calibrationSummary(ped.ClutchCalibration)},
		{"Raw Input / HID Fehler", advancedValue(s.RawInputError)},
		{"Device Detection Fehler", advancedValue(s.DeviceDetectionError)},
	}
	card = RECT{r.Left, y, r.Right, y + 250}
	paintAdvancedCard(hdc, card, "Kalibrierung & Erkennungsfehler", "Persistente Einstellungen sind an das physische Wheel gebunden", calRows)
	y = card.Bottom + gap

	card = RECT{r.Left, y, r.Right, y + 350}
	paintHardwareCertificationProgress(hdc, card, s.Certification)
}

func paintHardwareCertificationProgress(hdc uintptr, r RECT, p system.CertificationProgress) {
	drawRoundRect(hdc, r, 16, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
	t := RECT{r.Left + 16, r.Top + 10, r.Right - 16, r.Top + 31}
	drawText(hdc, "Hardware Certification · echte Wheel-Evidenz", &t, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	summary := "Kein zertifizierbares Wheel ausgewählt."
	if p.Required > 0 {
		summary = fmt.Sprintf("%d/%d PASS · %d offen · %d FAIL · Stable bleibt bis zur vollständigen physischen Matrix gesperrt.", p.Passed, p.Required, p.Pending, p.Failed)
		if p.Complete {
			summary = fmt.Sprintf("%d/%d PASS · lokale Modell-Evidenz vollständig. Stable benötigt zusätzlich globale Release-Gates.", p.Passed, p.Required)
		}
	}
	if strings.TrimSpace(p.Error) != "" {
		summary += " · Fehler: " + p.Error
	}
	sr := RECT{r.Left + 16, r.Top + 31, r.Right - 16, r.Top + 54}
	drawText(hdc, summary, &sr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	if len(p.Items) == 0 {
		return
	}
	cols := 3
	gap := int32(8)
	innerL, innerR := r.Left+16, r.Right-16
	cellW := (innerR - innerL - int32(cols-1)*gap) / int32(cols)
	cellH := int32(39)
	startY := r.Top + 64
	for i, it := range p.Items {
		row, col := i/cols, i%cols
		x := innerL + int32(col)*(cellW+gap)
		y := startY + int32(row)*(cellH+7)
		cr := RECT{x, y, x + cellW, y + cellH}
		fill, border, text := colPanel, colBorder, colMuted
		prefix := "○"
		if it.Passed {
			fill, border, prefix = blendColor(colGood, colPanel, 78), colGood, "✓"
			text = readableTextColor(colGood, fill)
		} else if it.Failed {
			fill, border, prefix = blendColor(colBad, colPanel, 78), colBad, "✕"
			text = readableTextColor(colBad, fill)
		}
		drawRoundRect(hdc, cr, 9, fill, border)
		pSetTextColor.Call(hdc, text)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		label := prefix + "  " + it.Label
		drawText(hdc, label, &cr, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}
