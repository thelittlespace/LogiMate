//go:build windows

package app

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// modernDialogButton is a LogiMate-owned replacement for the mix of classic
// MessageBox/TaskDialog surfaces that used to appear throughout the app.
// Command-link dialogs use the same Acrylic/Mica card language as "Was ist neu?".
type modernDialogButton struct {
	ID          int
	Title       string
	Description string
	Primary     bool
	Warning     bool
}

type modernDialogSpec struct {
	Parent       HWND
	WindowTitle  string
	Heading      string
	Subtitle     string
	Content      string
	Kind         string // info/warning/error/question
	Buttons      []modernDialogButton
	CommandLinks bool
	DefaultID    int
	CancelID     int
	Width        int32
	Height       int32
	LiveContent  func() string
	RefreshMS    uint32
}

type modernDialogState struct {
	hwnd         HWND
	spec         modernDialogSpec
	result       int
	done         bool
	glass        bool
	hover        int
	focus        int
	scroll       int32
	scrollMax    int32
	mouseTrack   bool
	commandRects []RECT
	footerRects  []RECT
}

const modernDialogRefreshTimer = 0x4D47

var (
	modernDialogClassRegistered bool
	modernDialogStatesMu        sync.Mutex
	modernDialogStates          = map[HWND]*modernDialogState{}
)

func splitDialogChoice(s string) (string, string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.SplitN(s, "\n", 2)
	title := strings.TrimSpace(parts[0])
	desc := ""
	if len(parts) == 2 {
		desc = strings.TrimSpace(parts[1])
	}
	return title, desc
}

func modernChoiceDialog(parent HWND, title, instruction, content string, choices []string, base int32) int {
	if len(choices) == 0 {
		return -1
	}
	buttons := make([]modernDialogButton, 0, len(choices))
	for i, choice := range choices {
		t, d := splitDialogChoice(choice)
		buttons = append(buttons, modernDialogButton{ID: int(base) + i, Title: t, Description: d, Primary: i == 0})
	}
	h := int32(350 + len(choices)*72)
	if h > 740 {
		h = 740
	}
	if h < 520 {
		h = 520
	}
	result, ok := runModernDialog(modernDialogSpec{
		Parent: parent, WindowTitle: title, Heading: instruction,
		Subtitle: "LogiMate · moderner Auswahldialog", Content: content,
		Kind: "info", Buttons: buttons, CommandLinks: true,
		DefaultID: int(base), CancelID: IDCANCEL, Width: 720, Height: h,
	})
	if !ok || result < int(base) || result >= int(base)+len(choices) {
		return -1
	}
	return result - int(base)
}

func modernMessageBox(parent HWND, text, title string, flags uint32) (int, bool) {
	kind := "info"
	switch {
	case flags&MB_ICONERROR != 0:
		kind = "error"
	case flags&MB_ICONWARNING != 0:
		kind = "warning"
	}
	buttons := []modernDialogButton{{ID: IDOK, Title: "OK", Primary: true}}
	defaultID := IDOK
	cancelID := IDOK
	if flags&MB_YESNO != 0 {
		defaultID = IDYES
		if flags&MB_DEFBUTTON2 != 0 {
			defaultID = IDNO
		}
		buttons = []modernDialogButton{
			{ID: IDYES, Title: "Ja", Primary: defaultID == IDYES},
			{ID: IDNO, Title: "Nein", Primary: defaultID == IDNO},
		}
		cancelID = IDNO
		kind = "question"
		if flags&MB_ICONWARNING != 0 {
			kind = "warning"
		}
	}
	heading := strings.TrimSpace(title)
	if heading == "" || strings.EqualFold(heading, "LogiMate") {
		heading = "LogiMate"
	}
	return runModernDialog(modernDialogSpec{
		Parent:      parent,
		WindowTitle: "LogiMate · " + heading,
		Heading:     heading,
		Subtitle:    "LogiMate · " + map[string]string{"info": "Information", "warning": "Bitte prüfen", "error": "Fehler", "question": "Bestätigung"}[kind],
		Content:     text,
		Kind:        kind,
		Buttons:     buttons,
		DefaultID:   defaultID,
		CancelID:    cancelID,
		Width:       680,
		Height:      480,
	})
}

func registerModernDialogClass() bool {
	if modernDialogClassRegistered {
		return true
	}
	hinst, _, _ := pGetModuleHandleW.Call(0)
	cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	icon := brandIcon(64)
	if icon == 0 {
		fallback, _, _ := pLoadIconW.Call(0, IDI_APPLICATION)
		icon = HICON(fallback)
	}
	className := utf16("LogiMateModernDialogWindow")
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:         0x0002 | 0x0001,
		LpfnWndProc:   syscall.NewCallback(modernDialogWndProc),
		HInstance:     HINSTANCE(hinst),
		HIcon:         icon,
		HCursor:       HCURSOR(cursor),
		HbrBackground: 0,
		LpszClassName: className,
		HIconSm:       icon,
	}
	if r, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return false
	}
	modernDialogClassRegistered = true
	return true
}

func dialogInitialFocus(spec modernDialogSpec) int {
	for i, b := range spec.Buttons {
		if b.ID == spec.DefaultID {
			return i
		}
	}
	if len(spec.Buttons) > 0 {
		return 0
	}
	return -1
}

func ensureModernDialogResources() func() {
	if bgBrush != 0 && fontBody != 0 && fontTitle != 0 && fontSection != 0 && fontLabel != 0 {
		return func() {}
	}
	// Admin actions, uninstall and very-early startup failures run before the
	// main window initializes its GDI cache. Build a tiny temporary copy so even
	// those paths use the same LogiMate dialog instead of falling back to a
	// classic MessageBox.
	oldBG, oldGlass := bgBrush, glassBrush
	oldBody, oldTitle, oldSubtitle, oldSmall := fontBody, fontTitle, fontSubtitle, fontSmall
	oldSection, oldLabel := fontSection, fontLabel
	bgBrush = createSolidBrush(themeBackground)
	glassBrush = createSolidBrush(rgb(0, 0, 0))
	fontBody = createFontName(-16, 400, "Segoe UI Variable Text")
	fontTitle = createFontName(-30, 600, "Segoe UI Variable Display")
	fontSubtitle = createFontName(-14, 400, "Segoe UI Variable Text")
	fontSmall = createFontName(-12, 400, "Segoe UI Variable Text")
	fontSection = createFontName(-18, 600, "Segoe UI Variable Text")
	fontLabel = createFontName(-13, 600, "Segoe UI Variable Text")
	return func() {
		for _, h := range []uintptr{uintptr(bgBrush), uintptr(glassBrush), uintptr(fontBody), uintptr(fontTitle), uintptr(fontSubtitle), uintptr(fontSmall), uintptr(fontSection), uintptr(fontLabel)} {
			deleteObject(h)
		}
		bgBrush, glassBrush = oldBG, oldGlass
		fontBody, fontTitle, fontSubtitle, fontSmall = oldBody, oldTitle, oldSubtitle, oldSmall
		fontSection, fontLabel = oldSection, oldLabel
	}
}

func runModernDialog(spec modernDialogSpec) (int, bool) {
	cleanupTemp := ensureModernDialogResources()
	defer cleanupTemp()
	if !registerModernDialogClass() {
		return 0, false
	}
	if spec.Width <= 0 {
		spec.Width = 680
	}
	if spec.Height <= 0 {
		spec.Height = 480
	}
	if spec.WindowTitle == "" {
		spec.WindowTitle = "LogiMate"
	}
	if spec.CancelID == 0 {
		spec.CancelID = IDCANCEL
	}
	st := &modernDialogState{spec: spec, result: spec.CancelID, hover: -1, focus: dialogInitialFocus(spec)}

	x, y := int32(260), int32(145)
	if spec.Parent != 0 {
		var pr RECT
		if r, _, _ := pGetWindowRect.Call(uintptr(spec.Parent), uintptr(unsafe.Pointer(&pr))); r != 0 {
			x = pr.Left + max32(20, (pr.Right-pr.Left-spec.Width)/2)
			y = pr.Top + max32(20, (pr.Bottom-pr.Top-spec.Height)/2)
		}
	}
	hinst, _, _ := pGetModuleHandleW.Call(0)
	h, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16("LogiMateModernDialogWindow"))),
		uintptr(unsafe.Pointer(utf16(spec.WindowTitle))),
		WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN,
		uintptr(x), uintptr(y), uintptr(spec.Width), uintptr(spec.Height),
		uintptr(spec.Parent), 0, hinst, 0,
	)
	if h == 0 {
		return 0, false
	}
	st.hwnd = HWND(h)
	modernDialogStatesMu.Lock()
	modernDialogStates[st.hwnd] = st
	modernDialogStatesMu.Unlock()

	applyWindowChromeTheme(st.hwnd)
	if spec.Parent != 0 {
		pEnableWindow.Call(uintptr(spec.Parent), 0)
	}
	pShowWindow.Call(h, SW_SHOW)
	pUpdateWindow.Call(h)
	st.glass = applyWindowMaterial(st.hwnd)
	if st.spec.LiveContent != nil {
		if st.spec.RefreshMS == 0 {
			st.spec.RefreshMS = 750
		}
		st.spec.Content = st.spec.LiveContent()
		pSetTimer.Call(uintptr(st.hwnd), modernDialogRefreshTimer, uintptr(st.spec.RefreshMS), 0)
	}
	invalidate(st.hwnd)

	var msg MSG
	for !st.done {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			st.done = true
			if int32(r) == 0 {
				pPostQuitMessage.Call(msg.WParam)
			}
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	if st.hwnd != 0 {
		pDestroyWindow.Call(uintptr(st.hwnd))
	}
	modernDialogStatesMu.Lock()
	delete(modernDialogStates, HWND(h))
	modernDialogStatesMu.Unlock()
	if spec.Parent != 0 {
		pEnableWindow.Call(uintptr(spec.Parent), 1)
		pSetForegroundWindow.Call(uintptr(spec.Parent))
	}
	return st.result, true
}

func lookupModernDialog(hwnd HWND) *modernDialogState {
	modernDialogStatesMu.Lock()
	defer modernDialogStatesMu.Unlock()
	return modernDialogStates[hwnd]
}

func finishModernDialog(st *modernDialogState, id int) {
	if st == nil || st.done {
		return
	}
	st.result = id
	st.done = true
	if st.hwnd != 0 {
		pDestroyWindow.Call(uintptr(st.hwnd))
		st.hwnd = 0
	}
}

func modernDialogWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	st := lookupModernDialog(HWND(hwnd))
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		if st != nil {
			paintModernDialog(st)
		}
		return 0
	case WM_TIMER:
		if st != nil && wParam == modernDialogRefreshTimer && st.spec.LiveContent != nil {
			oldContent := st.spec.Content
			st.spec.Content = st.spec.LiveContent()
			if st.spec.Content != oldContent {
				invalidate(HWND(hwnd))
			}
		}
		return 0
	case WM_MOUSEMOVE:
		if st != nil {
			if !st.mouseTrack {
				trackMouseLeave(HWND(hwnd))
				st.mouseTrack = true
			}
			idx := modernDialogHitTest(st, signedLoWord(lParam), signedHiWord(lParam))
			if idx != st.hover {
				st.hover = idx
				invalidate(HWND(hwnd))
			}
			setCursorHand(idx >= 0)
		}
		return 0
	case WM_MOUSELEAVE:
		if st != nil {
			st.mouseTrack = false
			st.hover = -1
			setCursorHand(false)
			invalidate(HWND(hwnd))
		}
		return 0
	case WM_MOUSEWHEEL:
		if st != nil && st.scrollMax > 0 {
			delta := signedHiWord(wParam)
			st.scroll -= (delta / 120) * 64
			if st.scroll < 0 {
				st.scroll = 0
			}
			if st.scroll > st.scrollMax {
				st.scroll = st.scrollMax
			}
			invalidate(HWND(hwnd))
		}
		return 0
	case WM_LBUTTONUP:
		if st != nil {
			idx := modernDialogHitTest(st, signedLoWord(lParam), signedHiWord(lParam))
			if idx >= 0 && idx < len(st.spec.Buttons) {
				finishModernDialog(st, st.spec.Buttons[idx].ID)
			} else if idx == len(st.spec.Buttons) && st.spec.CommandLinks {
				finishModernDialog(st, st.spec.CancelID)
			}
		}
		return 0
	case WM_KEYDOWN:
		if st == nil {
			break
		}
		switch wParam {
		case VK_ESCAPE:
			finishModernDialog(st, st.spec.CancelID)
			return 0
		case VK_UP, VK_LEFT:
			if len(st.spec.Buttons) > 0 {
				st.focus--
				if st.focus < 0 {
					st.focus = len(st.spec.Buttons) - 1
				}
				ensureDialogFocusVisible(st)
				invalidate(HWND(hwnd))
			}
			return 0
		case VK_DOWN, VK_RIGHT, VK_TAB:
			if len(st.spec.Buttons) > 0 {
				st.focus++
				if st.focus >= len(st.spec.Buttons) {
					st.focus = 0
				}
				ensureDialogFocusVisible(st)
				invalidate(HWND(hwnd))
			}
			return 0
		case VK_RETURN, VK_SPACE:
			if st.focus >= 0 && st.focus < len(st.spec.Buttons) {
				finishModernDialog(st, st.spec.Buttons[st.focus].ID)
			}
			return 0
		}
	case WM_CLOSE:
		if st != nil {
			finishModernDialog(st, st.spec.CancelID)
		}
		return 0
	case WM_DWMCOMPOSITIONCHANGED:
		if st != nil {
			st.glass = applyWindowMaterial(HWND(hwnd))
			invalidate(HWND(hwnd))
		}
		return 0
	case WM_SETTINGCHANGE:
		applyWindowChromeTheme(HWND(hwnd))
		if st != nil && !safeUI && getUISettings().Acrylic {
			st.glass = applyWindowMaterial(HWND(hwnd))
		}
		invalidate(HWND(hwnd))
		return 0
	case WM_DESTROY:
		if st != nil {
			pKillTimer.Call(hwnd, modernDialogRefreshTimer)
			st.done = true
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func ensureDialogFocusVisible(st *modernDialogState) {
	if st == nil || !st.spec.CommandLinks || st.focus < 0 || st.focus >= len(st.commandRects) {
		return
	}
	var rc RECT
	pGetClientRect.Call(uintptr(st.hwnd), uintptr(unsafe.Pointer(&rc)))
	viewportTop := int32(190)
	viewportBottom := rc.Bottom - 96
	r := st.commandRects[st.focus]
	if r.Top < viewportTop {
		st.scroll -= viewportTop - r.Top
	} else if r.Bottom > viewportBottom {
		st.scroll += r.Bottom - viewportBottom
	}
	if st.scroll < 0 {
		st.scroll = 0
	}
	if st.scroll > st.scrollMax {
		st.scroll = st.scrollMax
	}
}

func modernDialogHitTest(st *modernDialogState, x, y int32) int {
	if st == nil {
		return -1
	}
	for i, r := range st.commandRects {
		if r.Right > r.Left && x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom {
			return i
		}
	}
	for i, r := range st.footerRects {
		if r.Right > r.Left && x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom {
			return i
		}
	}
	if st.spec.CommandLinks {
		var rc RECT
		pGetClientRect.Call(uintptr(st.hwnd), uintptr(unsafe.Pointer(&rc)))
		cancel := RECT{rc.Right - 180, rc.Bottom - 70, rc.Right - 30, rc.Bottom - 24}
		if x >= cancel.Left && x <= cancel.Right && y >= cancel.Top && y <= cancel.Bottom {
			return len(st.spec.Buttons)
		}
	}
	return -1
}

func dialogAccent(kind string) uintptr {
	switch kind {
	case "warning":
		return colWarning
	case "error":
		return colBad
	default:
		return colAccent
	}
}

func dialogGlyph(kind string) string {
	switch kind {
	case "warning":
		return "!"
	case "error":
		return "×"
	case "question":
		return "?"
	default:
		return "i"
	}
}

func paintModernDialog(st *modernDialogState) {
	if st == nil || st.hwnd == 0 {
		return
	}
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(uintptr(st.hwnd), uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(uintptr(st.hwnd), uintptr(unsafe.Pointer(&ps)))

	var rc RECT
	pGetClientRect.Call(uintptr(st.hwnd), uintptr(unsafe.Pointer(&rc)))
	clear := bgBrush
	if st.glass {
		clear = materialClearBrush()
	}
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(clear))
	pSetBkMode.Call(hdc, TRANSPARENT)

	accent := dialogAccent(st.spec.Kind)
	logo := RECT{30, 23, 78, 71}
	drawBrandIcon(hdc, logo)
	heading := RECT{92, 24, rc.Right - 86, 70}
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontTitle))
	drawText(hdc, st.spec.Heading, &heading, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	glyph := RECT{rc.Right - 70, 31, rc.Right - 34, 67}
	glyphFill := blendColor(accent, colPanel2, 72)
	fillRoundRect(hdc, glyph, 12, glyphFill)
	pSetTextColor.Call(hdc, readableTextColor(accent, glyphFill))
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSection))
	drawText(hdc, dialogGlyph(st.spec.Kind), &glyph, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)

	sub := RECT{94, 71, rc.Right - 34, 104}
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSubtitle))
	drawText(hdc, st.spec.Subtitle, &sub, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	card := RECT{30, 116, rc.Right - 30, rc.Bottom - 94}
	drawRoundRect(hdc, card, 18, colPanel, colBorder)
	rail := RECT{card.Left + 18, card.Top + 20, card.Left + 22, card.Bottom - 20}
	fillRoundRect(hdc, rail, 2, accent)

	contentLeft := card.Left + 40
	contentRight := card.Right - 28
	contentTop := card.Top + 24
	measure := RECT{contentLeft, 0, contentRight, 2000}
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
	measureText(hdc, st.spec.Content, &measure, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
	contentH := measure.Bottom
	if contentH < 28 {
		contentH = 28
	}
	pSetTextColor.Call(hdc, colText)
	cr := RECT{contentLeft, contentTop, contentRight, contentTop + contentH + 4}
	if !st.spec.CommandLinks {
		visibleH := card.Bottom - 18 - contentTop
		st.scrollMax = contentH - visibleH
		if st.scrollMax < 0 {
			st.scrollMax = 0
		}
		if st.scroll > st.scrollMax {
			st.scroll = st.scrollMax
		}
		cr.Top -= st.scroll
		cr.Bottom -= st.scroll
		saved, _, _ := pSaveDC.Call(hdc)
		pIntersectClipRect.Call(hdc, uintptr(card.Left+28), uintptr(contentTop), uintptr(card.Right-16), uintptr(card.Bottom-18))
		drawText(hdc, st.spec.Content, &cr, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
		if saved != 0 {
			pRestoreDC.Call(hdc, saved)
		}
		if st.scrollMax > 0 {
			track := RECT{card.Right - 12, contentTop, card.Right - 8, card.Bottom - 18}
			fillRoundRect(hdc, track, 2, colTrack)
			trackH := track.Bottom - track.Top
			thumbH := max32(34, trackH*visibleH/max32(contentH, 1))
			travel := trackH - thumbH
			y := track.Top
			if st.scrollMax > 0 {
				y += travel * st.scroll / st.scrollMax
			}
			fillRoundRect(hdc, RECT{track.Left, y, track.Right, y + thumbH}, 2, accent)
		}
	} else {
		drawText(hdc, st.spec.Content, &cr, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
	}
	pSelectObject.Call(hdc, old)

	st.commandRects = make([]RECT, len(st.spec.Buttons))
	st.footerRects = nil
	if st.spec.CommandLinks {
		listTop := cr.Bottom + 16
		viewportBottom := card.Bottom - 18
		cardH := int32(72)
		gap := int32(10)
		total := int32(len(st.spec.Buttons)) * (cardH + gap)
		visible := viewportBottom - listTop
		st.scrollMax = total - visible
		if st.scrollMax < 0 {
			st.scrollMax = 0
		}
		if st.scroll > st.scrollMax {
			st.scroll = st.scrollMax
		}
		saved, _, _ := pSaveDC.Call(hdc)
		pIntersectClipRect.Call(hdc, uintptr(card.Left+28), uintptr(listTop), uintptr(card.Right-16), uintptr(viewportBottom))
		for i, b := range st.spec.Buttons {
			top := listTop + int32(i)*(cardH+gap) - st.scroll
			r := RECT{contentLeft, top, contentRight, top + cardH}
			st.commandRects[i] = r
			fill, border := colPanel2, colBorder
			if st.hover == i || st.focus == i {
				fill, border = colHover, accent
			}
			if b.Warning {
				border = colWarning
			}
			drawRoundRect(hdc, r, 14, fill, border)
			chip := RECT{r.Left + 12, r.Top + 14, r.Left + 50, r.Bottom - 14}
			chipFill := blendColor(accent, colPanel2, 72)
			fillRoundRect(hdc, chip, 10, chipFill)
			pSetTextColor.Call(hdc, readableTextColor(accent, chipFill))
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
			drawText(hdc, fmt.Sprintf("%d", i+1), &chip, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
			pSelectObject.Call(hdc, old)
			pSetTextColor.Call(hdc, colText)
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
			tr := RECT{r.Left + 62, r.Top + 8, r.Right - 12, r.Top + 31}
			drawText(hdc, b.Title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
			if b.Description != "" {
				dr := RECT{r.Left + 62, r.Top + 32, r.Right - 12, r.Bottom - 6}
				drawFittedParagraph(hdc, b.Description, dr, colMuted, fontSmall, fontSmall)
			}
		}
		if saved != 0 {
			pRestoreDC.Call(hdc, saved)
		}
		if st.scrollMax > 0 {
			track := RECT{card.Right - 12, listTop, card.Right - 8, viewportBottom}
			fillRoundRect(hdc, track, 2, colTrack)
			trackH := track.Bottom - track.Top
			thumbH := max32(34, trackH*visible/max32(total, 1))
			travel := trackH - thumbH
			y := track.Top
			if st.scrollMax > 0 {
				y += travel * st.scroll / st.scrollMax
			}
			fillRoundRect(hdc, RECT{track.Left, y, track.Right, y + thumbH}, 2, accent)
		}
		cancel := RECT{rc.Right - 180, rc.Bottom - 70, rc.Right - 30, rc.Bottom - 24}
		fill := colPanel2
		if st.hover == len(st.spec.Buttons) {
			fill = colHover
		}
		drawRoundRect(hdc, cancel, 14, fill, colBorder)
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
		drawText(hdc, "Abbrechen", &cancel, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
		return
	}

	gap := int32(10)
	bw := int32(160)
	totalW := int32(len(st.spec.Buttons))*bw + int32(maxInt(len(st.spec.Buttons)-1, 0))*gap
	left := rc.Right - 30 - totalW
	if left < 30 {
		left = 30
	}
	st.footerRects = make([]RECT, len(st.spec.Buttons))
	for i, b := range st.spec.Buttons {
		r := RECT{left + int32(i)*(bw+gap), rc.Bottom - 70, left + int32(i)*(bw+gap) + bw, rc.Bottom - 24}
		st.footerRects[i] = r
		fill, border, text := colPanel2, colBorder, colText
		if b.Primary {
			fill, border, text = accent, accent, colOnAccent
		}
		if b.Warning {
			border = colWarning
		}
		if st.hover == i || st.focus == i {
			if b.Primary {
				fill = blendColor(fill, colText, 8)
			} else {
				fill = colHover
			}
			border = accent
		}
		drawRoundRect(hdc, r, 14, fill, border)
		pSetTextColor.Call(hdc, text)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
		drawText(hdc, b.Title, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
