//go:build windows

package app

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const currentReleaseNotes = `LogiMate 0.0.1-alpha · Build 017 — Publication Candidate

• Responsive/DPI behavior hardened for small and high-DPI displays.
• Suspend/Resume and stale HID sessions now fail safe and reconnect cleanly.
• G27 real-hardware FFB/LED evidence is tracked without overstating certification.
• Accessibility keyboard/UIA coverage now reaches all five wheel tabs and diagnostics.
• GitHub CI, CodeQL, Dependabot and release-candidate provenance are prepared.

Die sichtbare Produktversion bleibt 0.0.1-alpha. Vor einer öffentlichen Veröffentlichung folgt noch der vollständige Post-Build-017-Audit.`

var (
	changelogWnd             HWND
	changelogClassRegistered bool
	changelogText            string
	changelogScroll          int32
	changelogScrollMax       int32
	changelogGlassActive     bool
)

func showWhatsNewWindow(text string) bool {
	if strings.TrimSpace(text) == "" {
		text = currentReleaseNotes
	}
	changelogText = text
	changelogScroll = 0
	if changelogWnd != 0 {
		pSetForegroundWindow.Call(uintptr(changelogWnd))
		invalidate(changelogWnd)
		return true
	}

	hinst, _, _ := pGetModuleHandleW.Call(0)
	className := utf16("LogiMateWhatsNewWindow")
	if !changelogClassRegistered {
		cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
		icon := brandIcon(64)
		if icon == 0 {
			fallback, _, _ := pLoadIconW.Call(0, IDI_APPLICATION)
			icon = HICON(fallback)
		}
		wc := WNDCLASSEX{
			CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
			Style:         0x0002 | 0x0001,
			LpfnWndProc:   syscall.NewCallback(changelogWndProc),
			HInstance:     HINSTANCE(hinst),
			HIcon:         icon,
			HCursor:       HCURSOR(cursor),
			HbrBackground: 0,
			LpszClassName: className,
			HIconSm:       icon,
		}
		if r, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			// It is safe to retry only once. A class from another copy of the
			// process cannot exist inside this process, so treat failure as a
			// non-fatal inability to show the custom window.
			return false
		}
		changelogClassRegistered = true
	}

	title := fmt.Sprintf("LogiMate %s – Was ist neu?", displayVersion(Version))
	h, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16(title))),
		WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN,
		230, 130, 780, 640,
		uintptr(mainWnd), 0, hinst, 0,
	)
	if h == 0 {
		return false
	}
	changelogWnd = HWND(h)
	applyWindowChromeTheme(changelogWnd)
	pShowWindow.Call(h, SW_SHOW)
	pUpdateWindow.Call(h)
	// This window is created only after the main message loop is stable, so
	// applying the same Windows material synchronously cannot block startup.
	changelogGlassActive = applyWindowMaterial(changelogWnd)
	invalidate(changelogWnd)
	return true
}

func changelogWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		paintChangelog(HWND(hwnd))
		return 0
	case WM_MOUSEWHEEL:
		delta := signedHiWord(wParam)
		if delta != 0 {
			changelogScroll -= (delta / 120) * 60
			if changelogScroll < 0 {
				changelogScroll = 0
			}
			if changelogScroll > changelogScrollMax {
				changelogScroll = changelogScrollMax
			}
			invalidate(HWND(hwnd))
		}
		return 0
	case WM_KEYDOWN:
		if wParam == VK_ESCAPE {
			pDestroyWindow.Call(hwnd)
			return 0
		}
	case WM_DESTROY:
		changelogWnd = 0
		changelogGlassActive = false
		if updateInstallPromptPending && mainWnd != 0 {
			updateInstallPromptPending = false
			postMessage(mainWnd, msgUpdateDownloaded, 0, 0)
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func paintChangelog(hwnd HWND) {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	var rc RECT
	pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	clear := bgBrush
	if changelogGlassActive {
		clear = glassBrush
	}
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(clear))
	pSetBkMode.Call(hdc, TRANSPARENT)

	logo := RECT{30, 24, 78, 72}
	drawBrandIcon(hdc, logo)
	header := RECT{92, 28, rc.Right - 180, 74}
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontTitle))
	drawText(hdc, "Was ist neu?", &header, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	badge := RECT{rc.Right - 158, 34, rc.Right - 34, 66}
	drawRoundRect(hdc, badge, 16, colSelected, colAccent)
	pSetTextColor.Call(hdc, readableTextColor(colAccent, colSelected))
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	drawText(hdc, displayVersion(Version), &badge, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	sub := RECT{94, 72, rc.Right - 36, 104}
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSubtitle))
	drawText(hdc, "LogiMate "+displayVersion(Version)+" · dein Logitech-Freund entwickelt sich weiter", &sub, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	card := RECT{30, 116, rc.Right - 30, rc.Bottom - 30}
	drawRoundRect(hdc, card, 18, colPanel, colBorder)

	textRect := RECT{card.Left + 24, card.Top + 22 - changelogScroll, card.Right - 28, card.Bottom - 22}
	measure := RECT{textRect.Left, 0, textRect.Right, 20000}
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
	measureText(hdc, changelogText, &measure, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
	visible := card.Bottom - card.Top - 44
	changelogScrollMax = measure.Bottom - visible
	if changelogScrollMax < 0 {
		changelogScrollMax = 0
	}
	if changelogScroll > changelogScrollMax {
		changelogScroll = changelogScrollMax
	}
	textRect.Top = card.Top + 22 - changelogScroll
	textRect.Bottom = textRect.Top + measure.Bottom + 8
	pSetTextColor.Call(hdc, colText)
	saved, _, _ := pSaveDC.Call(hdc)
	pIntersectClipRect.Call(hdc, uintptr(card.Left+18), uintptr(card.Top+16), uintptr(card.Right-18), uintptr(card.Bottom-16))
	drawText(hdc, changelogText, &textRect, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
	if saved != 0 {
		pRestoreDC.Call(hdc, saved)
	}
	pSelectObject.Call(hdc, old)

	if changelogScrollMax > 0 {
		track := RECT{card.Right - 11, card.Top + 22, card.Right - 7, card.Bottom - 22}
		fillRoundRect(hdc, track, 2, colTrack)
		trackH := track.Bottom - track.Top
		thumbH := max32(34, trackH*visible/max32(measure.Bottom, 1))
		travel := trackH - thumbH
		y := track.Top
		if changelogScrollMax > 0 {
			y += travel * changelogScroll / changelogScrollMax
		}
		fillRoundRect(hdc, RECT{track.Left, y, track.Right, y + thumbH}, 2, colAccentSoft)
	}
}

func displayVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// releaseIdentity is intentionally separate from the visible product version.
// During the 0.0.1-alpha stabilization line the SemVer shown to users stays
// frozen while BuildID advances. This identity is used for update/changelog
// bookkeeping only and must not leak into Windows DisplayVersion.
func releaseIdentity() string {
	v := displayVersion(Version)
	b := strings.TrimSpace(BuildID)
	if strings.EqualFold(v, "0.0.1-alpha") && b != "" && !strings.EqualFold(b, "dev") {
		b = strings.TrimLeft(b, "0")
		if b == "" {
			b = "0"
		}
		return v + "." + b
	}
	return v
}

func buildLabel() string {
	b := strings.TrimSpace(BuildID)
	if b == "" || strings.EqualFold(b, "dev") {
		return "Build dev"
	}
	return "Build " + b
}
