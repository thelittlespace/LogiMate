//go:build windows

package app

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// D5.8 exposes the custom-painted main-window controls through native BUTTON
// child HWNDs. Windows supplies the standard MSAA/UI Automation providers for
// these controls, while a transparent hit-test subclass lets LogiMate keep its
// existing custom painting and mouse behavior. This avoids inventing a partial
// COM provider for the hand-painted surface and gives Narrator/UIA clients real
// names, roles, focus, enabled and toggle/selection states.
const (
	focusWheelAdvanced = 400

	accessibilityIDNavBase         = 6100
	accessibilityIDAdvanced        = 6110
	accessibilityIDMemoryIntegrity = 6111
	accessibilityIDThemeBase       = 6120
	accessibilityIDSettingBase     = 6140
	accessibilityIDActionBase      = 6170
	accessibilityIDD6TabBase       = 6200
	accessibilityIDD6Toggle        = 6210
	accessibilityIDD6NativeOutput  = 6211
	accessibilityIDD6Probe         = 6212
	accessibilityIDD6SliderBase    = 6220
	accessibilityIDD6ProfileBase   = 6240
	accessibilityIDD6ShapeBase     = 6250
	accessibilityIDD6TestBase      = 6260
	accessibilityIDD62ViewBase     = 6270
	accessibilityIDWheelCalBase    = 6300
	accessibilityIDWheelProfBase   = 6310
	accessibilityIDWheelDevBase    = 6320
	accessibilityIDDiagTabBase     = 6340
	accessibilityIDDiagActionBase  = 6350
)

type accessibilityOverlayKind int

const (
	accessibilityNav accessibilityOverlayKind = iota
	accessibilityAdvanced
	accessibilityMemoryIntegrity
	accessibilityTheme
	accessibilitySetting
	accessibilityAction
	accessibilityD6Tab
	accessibilityD62View
	accessibilityD6Toggle
	accessibilityD6NativeOutput
	accessibilityD6Probe
	accessibilityD6Slider
	accessibilityD6Profile
	accessibilityD6Shape
	accessibilityD6Test
	accessibilityWheelCalibrationAction
	accessibilityWheelProfileAction
	accessibilityWheelDeviceAction
	accessibilityDiagnosticsTab
	accessibilityDiagnosticsAction
)

type accessibilityOverlayControl struct {
	hwnd    HWND
	id      int
	focusID int
	kind    accessibilityOverlayKind
	index   int
	visible bool
	enabled bool
	label   string
	rect    RECT
}

var (
	accessibilityOverlays         []*accessibilityOverlayControl
	accessibilityOldButtonProc    uintptr
	accessibilitySubclassProc     uintptr
	accessibilityOverlayReady     bool
	accessibilityOverlayError     string
	accessibilityStatusHWND       HWND
	accessibilityStatusText       string
	accessibilityStatusRect       RECT
	accessibilityStatusLastUpdate time.Time
)

func ensureAccessibilityOverlays(parent HWND) {
	if parent == 0 || accessibilityOverlayReady || accessibilityOverlayError != "" {
		return
	}
	accessibilitySubclassProc = syscall.NewCallback(accessibilityOverlayWndProc)

	create := func(kind accessibilityOverlayKind, index, id, focusID int, label string, style uintptr) bool {
		h, _, err := pCreateWindowExW.Call(
			WS_EX_LAYERED|WS_EX_TRANSPARENT,
			uintptr(unsafe.Pointer(utf16("BUTTON"))),
			uintptr(unsafe.Pointer(utf16(label))),
			WS_CHILD|WS_VISIBLE|WS_TABSTOP|style,
			0, 0, 1, 1,
			uintptr(parent), uintptr(id), 0, 0,
		)
		if h == 0 {
			accessibilityOverlayError = fmt.Sprintf("UIA-Overlay %d konnte nicht erstellt werden: %v", id, err)
			return false
		}
		// Fully transparent visually, but still a real, visible HWND with the
		// native Button/CheckBox/RadioButton accessibility provider.
		pSetLayeredWindowAttributes.Call(h, 0, 0, LWA_ALPHA)
		old, _, _ := pSetWindowLongPtrW.Call(h, ^uintptr(3), accessibilitySubclassProc) // GWLP_WNDPROC == -4
		if old == 0 {
			pDestroyWindow.Call(h)
			accessibilityOverlayError = fmt.Sprintf("UIA-Overlay %d konnte nicht unterklassen werden", id)
			return false
		}
		if accessibilityOldButtonProc == 0 {
			accessibilityOldButtonProc = old
		}
		accessibilityOverlays = append(accessibilityOverlays, &accessibilityOverlayControl{
			hwnd: HWND(h), id: id, focusID: focusID, kind: kind, index: index,
			visible: true, enabled: true, label: label,
		})
		return true
	}

	for i, n := range navItems {
		if !create(accessibilityNav, i, accessibilityIDNavBase+i, i, n.title, BS_PUSHBUTTON) {
			return
		}
	}
	if !create(accessibilityAdvanced, 0, accessibilityIDAdvanced, focusWheelAdvanced, "Erweiterte Lenkradansicht", BS_AUTOCHECKBOX) {
		return
	}
	if !create(accessibilityMemoryIntegrity, 0, accessibilityIDMemoryIntegrity, focusMemoryIntegrity, "Speicherintegrität in Windows-Sicherheit öffnen", BS_PUSHBUTTON) {
		return
	}
	for i, label := range []string{"Live", "Force Feedback", "Kalibrierung", "Profile", "Gerät"} {
		if !create(accessibilityD6Tab, i, accessibilityIDD6TabBase+i, d6FocusTabLive+i, "Lenkrad-Reiter "+label, BS_PUSHBUTTON) {
			return
		}
	}
	for i, label := range []string{"Basis", "Effekte", "Signalformung", "Live & Test"} {
		if !create(accessibilityD62View, i, accessibilityIDD62ViewBase+i, d62FocusViewBase+i, "Force-Feedback-Bereich "+label, BS_PUSHBUTTON) {
			return
		}
	}
	if !create(accessibilityD6Toggle, 0, accessibilityIDD6Toggle, d6FocusToggle, "FFB Pipeline", BS_AUTOCHECKBOX) {
		return
	}
	if !create(accessibilityD6NativeOutput, 0, accessibilityIDD6NativeOutput, d6FocusNativeOutput, "Native Wheel Output", BS_AUTOCHECKBOX) {
		return
	}
	if !create(accessibilityD6Probe, 0, accessibilityIDD6Probe, d6FocusProbe, "HID Writer prüfen", BS_PUSHBUTTON) {
		return
	}
	for i := 0; i < int(d6SliderCount); i++ {
		if !create(accessibilityD6Slider, i, accessibilityIDD6SliderBase+i, d6FocusSliderBase+i, fmt.Sprintf("Force Feedback Regler %d", i+1), BS_PUSHBUTTON) {
			return
		}
	}
	for i, label := range []string{"Sanft", "Ausgewogen", "Direkt"} {
		if !create(accessibilityD6Profile, i, accessibilityIDD6ProfileBase+i, d6FocusProfileBase+i, "Wheel-Profil "+label, BS_PUSHBUTTON) {
			return
		}
	}
	for i, label := range []string{"Neutral", "Smooth", "Responsive", "Compensated"} {
		if !create(accessibilityD6Shape, i, accessibilityIDD6ShapeBase+i, d6FocusShapeBase+i, "FFB-Preset "+label, BS_PUSHBUTTON) {
			return
		}
	}
	for i, label := range []string{"Constant", "Spring", "Damper", "Friction", "Autocenter", "Emergency Stop"} {
		if !create(accessibilityD6Test, i, accessibilityIDD6TestBase+i, d6FocusTestBase+i, "FFB-Test "+label, BS_PUSHBUTTON) {
			return
		}
	}
	for i, label := range []string{"Lenkung kalibrieren", "Pedale kalibrieren", "Taste lernen", "H-Shifter lernen"} {
		if !create(accessibilityWheelCalibrationAction, i, accessibilityIDWheelCalBase+i, focusWheelCalibrationActionBase+i, label, BS_PUSHBUTTON) {
			return
		}
	}
	for i, label := range []string{"Wheel-Profile verwalten", "Game-Profile verwalten", "Force Feedback öffnen", "Profile als JSON exportieren"} {
		if !create(accessibilityWheelProfileAction, i, accessibilityIDWheelProfBase+i, focusWheelProfileActionBase+i, label, BS_PUSHBUTTON) {
			return
		}
	}
	for i, label := range []string{"Geräte verwalten", "Rohreport kopieren", "Erweiterte Ansicht", "Windows Controller"} {
		if !create(accessibilityWheelDeviceAction, i, accessibilityIDWheelDevBase+i, focusWheelDeviceActionBase+i, label, BS_PUSHBUTTON) {
			return
		}
	}
	for i, label := range []string{"Übersicht", "Probleme", "Tests", "Protokoll", "Export", "Release"} {
		if !create(accessibilityDiagnosticsTab, i, accessibilityIDDiagTabBase+i, focusDiagnosticsTabBase+i, "Diagnose-Reiter "+label, BS_PUSHBUTTON) {
			return
		}
	}
	for i := 0; i < 4; i++ {
		if !create(accessibilityDiagnosticsAction, i, accessibilityIDDiagActionBase+i, focusDiagnosticsActionBase+i, fmt.Sprintf("Diagnose-Aktion %d", i+1), BS_PUSHBUTTON) {
			return
		}
	}
	for i, t := range themeChoices {
		if !create(accessibilityTheme, i, accessibilityIDThemeBase+i, focusThemeBase+i, "Farbmodus "+t.label, BS_AUTORADIOBUTTON) {
			return
		}
	}
	for i, d := range settingDefs {
		if !create(accessibilitySetting, i, accessibilityIDSettingBase+i, focusSettingBase+i, d.title, BS_AUTOCHECKBOX) {
			return
		}
	}
	for i := range actionButtons {
		if !create(accessibilityAction, i, accessibilityIDActionBase+i, focusActionBase+i, fmt.Sprintf("Aktion %d", i+1), BS_PUSHBUTTON) {
			return
		}
	}
	// A transparent native STATIC exposes the current page summary/live text to
	// screen readers. It is disabled and not in the tab order, so it cannot
	// intercept interaction with the painted dashboard underneath.
	status, _, err := pCreateWindowExW.Call(
		WS_EX_LAYERED|WS_EX_TRANSPARENT,
		uintptr(unsafe.Pointer(utf16("STATIC"))),
		uintptr(unsafe.Pointer(utf16("LogiMate Status"))),
		WS_CHILD|WS_VISIBLE|WS_DISABLED,
		0, 0, 1, 1, uintptr(parent), 6190, 0, 0,
	)
	if status == 0 {
		accessibilityOverlayError = fmt.Sprintf("UIA-Statusfläche konnte nicht erstellt werden: %v", err)
		return
	}
	accessibilityStatusHWND = HWND(status)
	pSetLayeredWindowAttributes.Call(status, 0, 0, LWA_ALPHA)
	accessibilityOverlayReady = true
	accessibilityOverlayError = ""
	logStartup("UI Automation overlays ready: %d native controls", len(accessibilityOverlays))
}

func accessibilityOverlayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_NCHITTEST:
		// Keep all mouse behavior on the custom-painted parent. UIA clients and
		// keyboard focus can still address this native child directly.
		return ^uintptr(0) // HTTRANSPARENT == -1
	case WM_SETFOCUS:
		if c := accessibilityOverlayByHWND(HWND(hwnd)); c != nil {
			keyboardFocus = c.focusID
			ensureKeyboardFocusVisible(c.focusID)
			invalidate(mainWnd)
		}
	case WM_KEYDOWN:
		if onKeyDown(uint32(wParam)) {
			if uint32(wParam) == VK_ESCAPE && mainWnd != 0 {
				pSetFocus.Call(uintptr(mainWnd))
			}
			return 0
		}
	}
	if accessibilityOldButtonProc != 0 {
		r, _, _ := pCallWindowProcW.Call(accessibilityOldButtonProc, hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	return 0
}

func accessibilityOverlayByHWND(hwnd HWND) *accessibilityOverlayControl {
	for _, c := range accessibilityOverlays {
		if c.hwnd == hwnd {
			return c
		}
	}
	return nil
}

func accessibilityOverlayByFocusID(focusID int) *accessibilityOverlayControl {
	for _, c := range accessibilityOverlays {
		if c.focusID == focusID {
			return c
		}
	}
	return nil
}

func setKeyboardFocusID(focusID int) {
	keyboardFocus = focusID
	ensureKeyboardFocusVisible(focusID)
	if c := accessibilityOverlayByFocusID(focusID); c != nil && c.visible && c.enabled && c.hwnd != 0 {
		pSetFocus.Call(uintptr(c.hwnd))
	}
	invalidate(mainWnd)
}

func ensureKeyboardFocusVisible(focusID int) {
	if mainWnd == 0 {
		return
	}
	if currentPage == pageWheel {
		d6EnsureFocusVisible(focusID)
		return
	}
	if currentPage != pageSettings {
		return
	}
	if focusID >= focusThemeBase && focusID < focusThemeBase+len(themeChoices) {
		if contentScroll != 0 {
			contentScroll = 0
		}
		return
	}
	if focusID < focusSettingBase || focusID >= focusSettingBase+len(settingDefs) {
		return
	}
	var rc RECT
	pGetClientRect.Call(uintptr(mainWnd), uintptr(unsafe.Pointer(&rc)))
	content := contentRectFor(rc)
	rows, _, _, totalHeight := settingsLayout(content)
	idx := focusID - focusSettingBase
	if idx < 0 || idx >= len(rows) {
		return
	}
	visibleTop := content.Top + 52
	visibleBottom := content.Bottom - 16
	visibleHeight := visibleBottom - visibleTop
	maxScroll := totalHeight - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	r := offsetRect(rows[idx], -contentScroll)
	if r.Top < visibleTop {
		contentScroll -= visibleTop - r.Top
	} else if r.Bottom > visibleBottom {
		contentScroll += r.Bottom - visibleBottom
	}
	if contentScroll < 0 {
		contentScroll = 0
	}
	if contentScroll > maxScroll {
		contentScroll = maxScroll
	}
}

func syncAccessibilityOverlays() {
	if mainWnd == 0 || !accessibilityOverlayReady {
		return
	}
	var client RECT
	pGetClientRect.Call(uintptr(mainWnd), uintptr(unsafe.Pointer(&client)))
	prefs := getUISettings()
	settingValues := settingsValues(prefs)

	for _, c := range accessibilityOverlays {
		visible, enabled, checked := false, true, false
		label := c.label
		r := RECT{}
		switch c.kind {
		case accessibilityNav:
			visible = c.index >= 0 && c.index < len(navItems)
			if visible {
				r = navItemRect(c.index, client)
				label = navItems[c.index].title
			}
		case accessibilityAdvanced:
			visible = currentPage == pageWheel && rectUsable(wheelAdvancedToggleRect)
			r = wheelAdvancedToggleRect
			checked = wheelAdvancedView
			label = map[bool]string{true: "Erweiterte Lenkradansicht aktiv", false: "Erweiterte Lenkradansicht"}[wheelAdvancedView]
		case accessibilityMemoryIntegrity:
			visible = pageShowsStatusBand() && rectUsable(memoryIntegrityRect)
			r = memoryIntegrityRect
			label = "Speicherintegrität in Windows-Sicherheit öffnen"
		case accessibilityD6Tab:
			visible = currentPage == pageWheel && c.index >= 0 && c.index < len(wheelSubtabRects) && rectUsable(wheelSubtabRects[c.index])
			if visible {
				r = wheelSubtabRects[c.index]
				labels := []string{"Live", "Force Feedback", "Kalibrierung", "Profile", "Gerät"}
				label = "Lenkrad-Reiter " + labels[c.index]
			}
		case accessibilityD62View:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabFFB && c.index >= 0 && c.index < len(d62ViewRects) && rectUsable(d62ViewRects[c.index])
			if visible {
				r = d62ViewRects[c.index]
				labels := []string{"Basis", "Effekte", "Signalformung", "Live & Test"}
				label = "Force-Feedback-Bereich " + labels[c.index]
			}
		case accessibilityD6Toggle:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabFFB && rectUsable(d6ToggleRect)
			r = d6ToggleRect
			checked = d6Draft.valid && d6Draft.advanced.Enabled
			label = map[bool]string{true: "FFB Pipeline aktiv", false: "FFB Pipeline deaktiviert"}[checked]
		case accessibilityD6NativeOutput:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabFFB && rectUsable(d6NativeOutputRect)
			r = d6NativeOutputRect
			checked = prefs.NativeWheelOutput
			label = map[bool]string{true: "Native Wheel Output aktiv", false: "Native Wheel Output deaktiviert"}[checked]
		case accessibilityD6Probe:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabFFB && rectUsable(d6ProbeRect)
			r = d6ProbeRect
			label = "HID Writer prüfen, ohne Force-Feedback-Report"
		case accessibilityD6Slider:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabFFB && c.index >= 0 && c.index < int(d6SliderCount) && rectUsable(d6SliderRects[c.index])
			if visible {
				r = d6SliderRects[c.index]
				k := d6SliderKind(c.index)
				enabled = d62SliderEnabled(k)
				name, _, _, _, value := d6SliderMeta(k)
				if enabled {
					label = name + " " + value + ". Pfeiltasten ändern. " + d62SliderBadge(k)
				} else {
					label = name + " " + value + ". Nicht verfügbar: " + d62SliderBadge(k)
				}
			}
		case accessibilityD6Profile:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabFFB && c.index >= 0 && c.index < len(d6ProfileRects) && rectUsable(d6ProfileRects[c.index])
			if visible {
				r = d6ProfileRects[c.index]
			}
		case accessibilityD6Shape:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabFFB && c.index >= 0 && c.index < len(d6ShapeRects) && rectUsable(d6ShapeRects[c.index])
			if visible {
				r = d6ShapeRects[c.index]
			}
		case accessibilityD6Test:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabFFB && c.index >= 0 && c.index < len(d6TestRects) && rectUsable(d6TestRects[c.index])
			if visible {
				r = d6TestRects[c.index]
				enabled = prefs.NativeWheelOutput || c.index == 5
			}
		case accessibilityWheelCalibrationAction:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabCalibration && !wheelCalibrationWizard.Active && c.index >= 0 && c.index < len(wheelCalibrationActionRects) && rectUsable(wheelCalibrationActionRects[c.index])
			if visible {
				r = wheelCalibrationActionRects[c.index]
			}
		case accessibilityWheelProfileAction:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabProfiles && c.index >= 0 && c.index < len(wheelProfileActionRects) && rectUsable(wheelProfileActionRects[c.index])
			if visible {
				r = wheelProfileActionRects[c.index]
			}
		case accessibilityWheelDeviceAction:
			visible = currentPage == pageWheel && wheelSubtab == wheelSubtabDevice && c.index >= 0 && c.index < len(wheelDeviceActionRects) && rectUsable(wheelDeviceActionRects[c.index])
			if visible {
				r = wheelDeviceActionRects[c.index]
			}
		case accessibilityDiagnosticsTab:
			visible = currentPage == pageDiagnostics && c.index >= 0 && c.index < diagnosticsTabCount && rectUsable(diagnosticsTabRects[c.index])
			if visible {
				r = diagnosticsTabRects[c.index]
			}
		case accessibilityDiagnosticsAction:
			visible = currentPage == pageDiagnostics && c.index >= 0 && c.index < len(diagnosticsActionRects) && rectUsable(diagnosticsActionRects[c.index])
			if visible {
				r = diagnosticsActionRects[c.index]
			}
		case accessibilityTheme:
			visible = currentPage == pageSettings && c.index >= 0 && c.index < len(themeChoices) && rectUsable(themeRects[c.index])
			if c.index >= 0 && c.index < len(themeChoices) {
				r = themeRects[c.index]
				label = "Farbmodus " + themeChoices[c.index].label
				checked = normalizeThemeMode(prefs.ThemeMode) == themeChoices[c.index].id
			}
		case accessibilitySetting:
			visible = currentPage == pageSettings && c.index >= 0 && c.index < len(settingRects) && rectUsable(settingRects[c.index])
			if c.index >= 0 && c.index < len(settingDefs) {
				label = settingDefs[c.index].title
				if c.index < len(settingRects) {
					r = settingRects[c.index]
				}
				if c.index < len(settingValues) {
					checked = settingValues[c.index]
				}
			}
		case accessibilityAction:
			if c.index >= 0 && c.index < len(actionButtons) {
				b := actionButtons[c.index]
				visible = b.visible && rectUsable(actionRects[c.index])
				enabled = b.enabled
				r = actionRects[c.index]
				if strings.TrimSpace(b.label) != "" {
					label = b.label
				}
			}
		}
		syncAccessibilityOverlay(c, visible, enabled, checked, r, label)
	}
	syncAccessibilityStatusSurface(client)
}

func syncAccessibilityStatusSurface(client RECT) {
	if accessibilityStatusHWND == 0 {
		return
	}
	content := contentRectFor(client)
	if !rectUsable(content) {
		pShowWindow.Call(uintptr(accessibilityStatusHWND), SW_HIDE)
		return
	}
	bodyMu.RLock()
	body := pageBody
	bodyMu.RUnlock()
	text := strings.TrimSpace(pageTitle() + "\r\n" + pageSubtitle() + "\r\n" + body)
	if text == "" {
		text = "LogiMate · " + pageTitle()
	}
	if text != accessibilityStatusText {
		// The wheel input timer can change this text four times per second. A
		// fully transparent native STATIC still generates accessibility/window
		// traffic on every SetWindowText. Throttle live-status publication while
		// keeping navigation/settings pages immediate.
		now := time.Now()
		if currentPage != pageWheel || accessibilityStatusLastUpdate.IsZero() || now.Sub(accessibilityStatusLastUpdate) >= time.Second {
			accessibilityStatusText = text
			accessibilityStatusLastUpdate = now
			pSetWindowTextW.Call(uintptr(accessibilityStatusHWND), uintptr(unsafe.Pointer(utf16(text))))
		}
	}
	w, h := content.Right-content.Left, content.Bottom-content.Top
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	nextRect := RECT{content.Left, content.Top, content.Left + w, content.Top + h}
	if nextRect != accessibilityStatusRect {
		accessibilityStatusRect = nextRect
		pMoveWindow.Call(uintptr(accessibilityStatusHWND), uintptr(content.Left), uintptr(content.Top), uintptr(w), uintptr(h), 0)
	}
	pShowWindow.Call(uintptr(accessibilityStatusHWND), SW_SHOWNA)
}

func rectUsable(r RECT) bool { return r.Right > r.Left && r.Bottom > r.Top }

func syncAccessibilityOverlay(c *accessibilityOverlayControl, visible, enabled, checked bool, r RECT, label string) {
	if c == nil || c.hwnd == 0 {
		return
	}
	if label != c.label {
		c.label = label
		pSetWindowTextW.Call(uintptr(c.hwnd), uintptr(unsafe.Pointer(utf16(label))))
	}
	if enabled != c.enabled {
		c.enabled = enabled
		v := uintptr(0)
		if enabled {
			v = 1
		}
		pEnableWindow.Call(uintptr(c.hwnd), v)
	}
	if c.kind == accessibilityAdvanced || c.kind == accessibilityTheme || c.kind == accessibilitySetting || c.kind == accessibilityD6Toggle || c.kind == accessibilityD6NativeOutput {
		state := uintptr(BST_UNCHECKED)
		if checked {
			state = BST_CHECKED
		}
		pSendMessageW.Call(uintptr(c.hwnd), BM_SETCHECK, state, 0)
	}
	if visible {
		w, h := r.Right-r.Left, r.Bottom-r.Top
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		nextRect := RECT{r.Left, r.Top, r.Left + w, r.Top + h}
		if nextRect != c.rect {
			c.rect = nextRect
			pMoveWindow.Call(uintptr(c.hwnd), uintptr(r.Left), uintptr(r.Top), uintptr(w), uintptr(h), 0)
		}
		if !c.visible {
			pShowWindow.Call(uintptr(c.hwnd), SW_SHOWNA)
		}
	} else if c.visible {
		pShowWindow.Call(uintptr(c.hwnd), SW_HIDE)
	}
	c.visible = visible
}

func handleAccessibilityCommand(id int) bool {
	switch {
	case id >= accessibilityIDNavBase && id < accessibilityIDNavBase+len(navItems):
		i := id - accessibilityIDNavBase
		setKeyboardFocusID(i)
		setPage(navItems[i].page)
		return true
	case id == accessibilityIDAdvanced:
		if currentPage != pageWheel {
			return true
		}
		setKeyboardFocusID(focusWheelAdvanced)
		wheelAdvancedView = !wheelAdvancedView
		contentScroll = 0
		setActionFeedback(map[bool]string{true: "Erweiterte Lenkradansicht aktiv.", false: "Standardansicht aktiv."}[wheelAdvancedView])
		invalidate(mainWnd)
		return true
	case id == accessibilityIDMemoryIntegrity:
		setKeyboardFocusID(focusMemoryIntegrity)
		if pageShowsStatusBand() && rectUsable(memoryIntegrityRect) {
			activateMemoryIntegritySettings()
		}
		return true
	case id >= accessibilityIDD6TabBase && id < accessibilityIDD6TabBase+5:
		i := id - accessibilityIDD6TabBase
		setKeyboardFocusID(d6FocusTabLive + i)
		wheelSubtab = i
		if i == wheelSubtabFFB {
			wheelAdvancedView = false
		}
		contentScroll = 0
		invalidate(mainWnd)
		return true
	case id >= accessibilityIDD62ViewBase && id < accessibilityIDD62ViewBase+d62ViewCount:
		i := id - accessibilityIDD62ViewBase
		if currentPage == pageWheel && wheelSubtab == wheelSubtabFFB {
			d62ActiveView = i
			contentScroll = 0
			setKeyboardFocusID(d62FocusViewBase + i)
			invalidate(mainWnd)
		}
		return true
	case id == accessibilityIDD6Toggle:
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(d6FocusToggle)
		return d6ActivateFocus(s)
	case id == accessibilityIDD6NativeOutput:
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(d6FocusNativeOutput)
		return d6ActivateFocus(s)
	case id == accessibilityIDD6Probe:
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(d6FocusProbe)
		return d6ActivateFocus(s)
	case id >= accessibilityIDD6SliderBase && id < accessibilityIDD6SliderBase+int(d6SliderCount):
		i := id - accessibilityIDD6SliderBase
		setKeyboardFocusID(d6FocusSliderBase + i)
		return true
	case id >= accessibilityIDD6ProfileBase && id < accessibilityIDD6ProfileBase+3:
		i := id - accessibilityIDD6ProfileBase
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(d6FocusProfileBase + i)
		return d6ActivateFocus(s)
	case id >= accessibilityIDD6ShapeBase && id < accessibilityIDD6ShapeBase+4:
		i := id - accessibilityIDD6ShapeBase
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(d6FocusShapeBase + i)
		return d6ActivateFocus(s)
	case id >= accessibilityIDD6TestBase && id < accessibilityIDD6TestBase+6:
		i := id - accessibilityIDD6TestBase
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(d6FocusTestBase + i)
		return d6ActivateFocus(s)
	case id >= accessibilityIDWheelCalBase && id < accessibilityIDWheelCalBase+len(wheelCalibrationActionRects):
		i := id - accessibilityIDWheelCalBase
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(focusWheelCalibrationActionBase + i)
		return wheelReworkActivateFocus(s, focusWheelCalibrationActionBase+i)
	case id >= accessibilityIDWheelProfBase && id < accessibilityIDWheelProfBase+len(wheelProfileActionRects):
		i := id - accessibilityIDWheelProfBase
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(focusWheelProfileActionBase + i)
		return wheelReworkActivateFocus(s, focusWheelProfileActionBase+i)
	case id >= accessibilityIDWheelDevBase && id < accessibilityIDWheelDevBase+len(wheelDeviceActionRects):
		i := id - accessibilityIDWheelDevBase
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(focusWheelDeviceActionBase + i)
		return wheelReworkActivateFocus(s, focusWheelDeviceActionBase+i)
	case id >= accessibilityIDDiagTabBase && id < accessibilityIDDiagTabBase+diagnosticsTabCount:
		i := id - accessibilityIDDiagTabBase
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(focusDiagnosticsTabBase + i)
		return diagnosticsActivateFocus(s, focusDiagnosticsTabBase+i)
	case id >= accessibilityIDDiagActionBase && id < accessibilityIDDiagActionBase+len(diagnosticsActionRects):
		i := id - accessibilityIDDiagActionBase
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		setKeyboardFocusID(focusDiagnosticsActionBase + i)
		return diagnosticsActivateFocus(s, focusDiagnosticsActionBase+i)
	case id >= accessibilityIDThemeBase && id < accessibilityIDThemeBase+len(themeChoices):
		i := id - accessibilityIDThemeBase
		setKeyboardFocusID(focusThemeBase + i)
		setThemeMode(themeChoices[i].id)
		return true
	case id >= accessibilityIDSettingBase && id < accessibilityIDSettingBase+len(settingDefs):
		i := id - accessibilityIDSettingBase
		setKeyboardFocusID(focusSettingBase + i)
		toggleSetting(i)
		return true
	case id >= accessibilityIDActionBase && id < accessibilityIDActionBase+len(actionButtons):
		i := id - accessibilityIDActionBase
		setKeyboardFocusID(focusActionBase + i)
		if actionButtons[i].visible && actionButtons[i].enabled {
			action(currentPage, i)
		}
		return true
	}
	return false
}

func accessibilityRuntimeSummary() string {
	expected := len(navItems) + 2 + len(themeChoices) + len(settingDefs) + len(actionButtons) + 5 + d62ViewCount + int(d6SliderCount) + 3 + 4 + 6 + 4 + 4 + 4 + diagnosticsTabCount + 4 + 3
	if accessibilityOverlayError != "" {
		return fmt.Sprintf("UI Automation: FEHLER · %s", accessibilityOverlayError)
	}
	if !accessibilityOverlayReady {
		return fmt.Sprintf("UI Automation: wartet · native Controls 0/%d", expected)
	}
	visible := 0
	for _, c := range accessibilityOverlays {
		if c.visible {
			visible++
		}
	}
	status := "ohne Statusfläche"
	if accessibilityStatusHWND != 0 {
		status = "Live-Statusfläche aktiv"
	}
	return fmt.Sprintf("UI Automation: native HWND-Provider bereit · %d/%d Controls · %d aktuell sichtbar · %s · Tastaturfokus=%d", len(accessibilityOverlays), expected, visible, status, keyboardFocus)
}

func accessibilityOverlayCoverageOK() bool {
	expected := len(navItems) + 2 + len(themeChoices) + len(settingDefs) + len(actionButtons) + 5 + d62ViewCount + int(d6SliderCount) + 3 + 4 + 6 + 4 + 4 + 4 + diagnosticsTabCount + 4 + 3
	if !accessibilityOverlayReady || accessibilityOverlayError != "" || accessibilityStatusHWND == 0 || len(accessibilityOverlays) != expected {
		return false
	}
	seen := make(map[int]bool, expected)
	for _, c := range accessibilityOverlays {
		if c == nil || c.hwnd == 0 || seen[c.focusID] {
			return false
		}
		seen[c.focusID] = true
	}
	return true
}
