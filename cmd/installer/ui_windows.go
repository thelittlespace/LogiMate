//go:build windows && installer

package main

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type iRECT struct{ Left, Top, Right, Bottom int32 }
type iMSG struct {
	Hwnd, Message, WParam, LParam, Time uintptr
	PtX, PtY                            int32
	Private                             uint32
}
type iPAINTSTRUCT struct {
	Hdc                uintptr
	Erase              int32
	RcPaint            iRECT
	Restore, IncUpdate int32
	RGBReserved        [32]byte
}
type iWNDCLASSEX struct {
	CbSize, Style                            uint32
	LpfnWndProc                              uintptr
	CbClsExtra, CbWndExtra                   int32
	HInstance, HIcon, HCursor, HbrBackground uintptr
	LpszMenuName, LpszClassName              *uint16
	HIconSm                                  uintptr
}
type iLOGFONT struct{}

var (
	gdi32Installer     = syscall.NewLazyDLL("gdi32.dll")
	dwmapiInstaller    = syscall.NewLazyDLL("dwmapi.dll")
	pIRegisterClassExW = user32.NewProc("RegisterClassExW")
	pICreateWindowExW  = user32.NewProc("CreateWindowExW")
	pIDefWindowProcW   = user32.NewProc("DefWindowProcW")
	pIShowWindow       = user32.NewProc("ShowWindow")
	pIUpdateWindow     = user32.NewProc("UpdateWindow")
	pIGetMessageW      = user32.NewProc("GetMessageW")
	pITranslateMessage = user32.NewProc("TranslateMessage")
	pIDispatchMessageW = user32.NewProc("DispatchMessageW")
	pIPostMessageW     = user32.NewProc("PostMessageW")
	pIPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pIDestroyWindow    = user32.NewProc("DestroyWindow")
	pIInvalidateRect   = user32.NewProc("InvalidateRect")
	pIBeginPaint       = user32.NewProc("BeginPaint")
	pIEndPaint         = user32.NewProc("EndPaint")
	pIGetClientRect    = user32.NewProc("GetClientRect")
	pILoadCursorW      = user32.NewProc("LoadCursorW")
	pILoadIconW        = user32.NewProc("LoadIconW")
	pISetCursor        = user32.NewProc("SetCursor")
	pIGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	pICreateSolidBrush      = gdi32Installer.NewProc("CreateSolidBrush")
	pIDeleteObject          = gdi32Installer.NewProc("DeleteObject")
	pIRoundRect             = gdi32Installer.NewProc("RoundRect")
	pICreatePen             = gdi32Installer.NewProc("CreatePen")
	pISelectObject          = gdi32Installer.NewProc("SelectObject")
	pISetTextColor          = gdi32Installer.NewProc("SetTextColor")
	pISetBkMode             = gdi32Installer.NewProc("SetBkMode")
	pIDrawTextW             = user32.NewProc("DrawTextW")
	pICreateFontW           = gdi32Installer.NewProc("CreateFontW")
	pIDwmSetWindowAttribute = dwmapiInstaller.NewProc("DwmSetWindowAttribute")
)

const (
	iWM_DESTROY          = 0x0002
	iWM_ERASEBKGND       = 0x0014
	iWM_PAINT            = 0x000F
	iWM_CLOSE            = 0x0010
	iWM_KEYDOWN          = 0x0100
	iWM_MOUSEMOVE        = 0x0200
	iWM_LBUTTONUP        = 0x0202
	iWM_APP              = 0x8000
	iWM_PROGRESS         = iWM_APP + 1
	iWM_DONE             = iWM_APP + 2
	iWS_OVERLAPPEDWINDOW = 0x00CF0000
	iSW_SHOW             = 5
	iIDC_ARROW           = 32512
	iIDC_HAND            = 32649
	iIDI_APPLICATION     = 32512
	iVK_ESCAPE           = 0x1B
	iDT_LEFT             = 0x0000
	iDT_CENTER           = 0x0001
	iDT_RIGHT            = 0x0002
	iDT_VCENTER          = 0x0004
	iDT_SINGLELINE       = 0x0020
	iDT_WORDBREAK        = 0x0010
	iDT_NOPREFIX         = 0x0800
	iDT_END_ELLIPSIS     = 0x8000
	iTRANSPARENT         = 1
	iPS_SOLID            = 0
)

type installReportFunc func(progress int, step, detail, command string)

type installerUIState struct {
	mu          sync.Mutex
	hwnd        uintptr
	update      bool
	status      string // ready/running/success/error
	progress    int
	step        string
	detail      string
	logs        []string
	commands    []string
	showDetails bool
	hover       int
	actionRect  iRECT
	detailRect  iRECT
	work        func(installReportFunc) (string, error)
}

var installerState *installerUIState

func iRGB(r, g, b byte) uintptr { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }
func iInside(r iRECT, x, y int32) bool {
	return x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom
}
func iSignedLo(v uintptr) int32 { return int32(int16(uint16(v & 0xffff))) }
func iSignedHi(v uintptr) int32 { return int32(int16(uint16((v >> 16) & 0xffff))) }

func iBrush(c uintptr) uintptr { h, _, _ := pICreateSolidBrush.Call(c); return h }
func iRound(hdc uintptr, r iRECT, rad int32, fill, border uintptr) {
	br := iBrush(fill)
	defer pIDeleteObject.Call(br)
	pen, _, _ := pICreatePen.Call(iPS_SOLID, 1, border)
	defer pIDeleteObject.Call(pen)
	ob, _, _ := pISelectObject.Call(hdc, br)
	op, _, _ := pISelectObject.Call(hdc, pen)
	pIRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(rad), uintptr(rad))
	pISelectObject.Call(hdc, ob)
	pISelectObject.Call(hdc, op)
}
func iText(hdc uintptr, text string, r iRECT, flags uintptr, font uintptr, color uintptr) {
	pISetBkMode.Call(hdc, iTRANSPARENT)
	pISetTextColor.Call(hdc, color)
	old, _, _ := pISelectObject.Call(hdc, font)
	defer pISelectObject.Call(hdc, old)
	pIDrawTextW.Call(hdc, uintptr(unsafe.Pointer(u16(text))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags)
}
func iFont(size int32, weight int32) uintptr {
	h, _, _ := pICreateFontW.Call(uintptr(-size), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16("Segoe UI"))))
	return h
}

var iFontTitle, iFontBody, iFontSmall uintptr

func runInstallerWindow(update bool, work func(installReportFunc) (string, error)) {
	hinst, _, _ := pIGetModuleHandleW.Call(0)
	cursor, _, _ := pILoadCursorW.Call(0, iIDC_ARROW)
	icon, _, _ := pILoadIconW.Call(0, iIDI_APPLICATION)
	class := u16("LogiMateInstallerWindow")
	wc := iWNDCLASSEX{CbSize: uint32(unsafe.Sizeof(iWNDCLASSEX{})), Style: 3, LpfnWndProc: syscall.NewCallback(installerWndProc), HInstance: hinst, HIcon: icon, HCursor: cursor, LpszClassName: class, HIconSm: icon}
	pIRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	iFontTitle = iFont(28, 600)
	iFontBody = iFont(17, 400)
	iFontSmall = iFont(14, 400)
	defer func() {
		pIDeleteObject.Call(iFontTitle)
		pIDeleteObject.Call(iFontBody)
		pIDeleteObject.Call(iFontSmall)
	}()

	st := &installerUIState{update: update, status: "ready", progress: 0, work: work, hover: -1}
	if update {
		st.status = "running"
		st.step = "Update wird vorbereitet"
		st.detail = "Die bestehende Installation wird sicher aktualisiert."
	}
	installerState = st
	title := "LogiMate Setup"
	if update {
		title = "LogiMate Update"
	}
	h, _, _ := pICreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(u16(title))), iWS_OVERLAPPEDWINDOW, 280, 155, 780, 650, 0, 0, hinst, 0)
	if h == 0 {
		return
	}
	st.hwnd = h
	// Windows 11 Mica/Acrylic-like system backdrop; harmlessly ignored on old builds.
	backdrop := int32(2)
	pIDwmSetWindowAttribute.Call(h, 38, uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
	pIShowWindow.Call(h, iSW_SHOW)
	pIUpdateWindow.Call(h)
	if update {
		go st.startWork()
	}
	var msg iMSG
	for {
		r, _, _ := pIGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pITranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pIDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	installerState = nil
}

func (st *installerUIState) report(progress int, step, detail, command string) {
	st.mu.Lock()
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	st.progress = progress
	st.step = step
	st.detail = detail
	if strings.TrimSpace(step) != "" {
		st.logs = append(st.logs, fmt.Sprintf("[%3d%%] %s — %s", progress, step, detail))
	}
	if strings.TrimSpace(command) != "" {
		st.commands = append(st.commands, "> "+command)
	}
	h := st.hwnd
	st.mu.Unlock()
	if h != 0 {
		pIPostMessageW.Call(h, iWM_PROGRESS, 0, 0)
	}
}
func (st *installerUIState) startWork() {
	st.mu.Lock()
	if st.status == "running" && len(st.logs) > 0 {
		st.mu.Unlock()
		return
	}
	st.status = "running"
	st.mu.Unlock()
	msg, err := st.work(st.report)
	st.mu.Lock()
	if err != nil {
		st.status = "error"
		st.step = "Installation fehlgeschlagen"
		st.detail = err.Error()
		st.logs = append(st.logs, "FEHLER: "+err.Error())
	} else {
		st.status = "success"
		st.progress = 100
		st.step = "Abgeschlossen"
		st.detail = msg
		st.logs = append(st.logs, "OK: "+msg)
	}
	h := st.hwnd
	st.mu.Unlock()
	if h != 0 {
		pIPostMessageW.Call(h, iWM_DONE, 0, 0)
	}
}

func installerWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	st := installerState
	switch msg {
	case iWM_ERASEBKGND:
		return 1
	case iWM_PAINT:
		if st != nil {
			paintInstaller(st)
		}
		return 0
	case iWM_MOUSEMOVE:
		if st != nil {
			x, y := iSignedLo(lParam), iSignedHi(lParam)
			hover := -1
			st.mu.Lock()
			ar, dr := st.actionRect, st.detailRect
			st.mu.Unlock()
			if iInside(ar, x, y) {
				hover = 0
			} else if iInside(dr, x, y) {
				hover = 1
			}
			st.mu.Lock()
			changed := st.hover != hover
			st.hover = hover
			st.mu.Unlock()
			cur := uintptr(iIDC_ARROW)
			if hover >= 0 {
				cur = iIDC_HAND
			}
			c, _, _ := pILoadCursorW.Call(0, cur)
			pISetCursor.Call(c)
			if changed {
				pIInvalidateRect.Call(hwnd, 0, 0)
			}
		}
		return 0
	case iWM_LBUTTONUP:
		if st != nil {
			x, y := iSignedLo(lParam), iSignedHi(lParam)
			st.mu.Lock()
			ar, dr, status := st.actionRect, st.detailRect, st.status
			st.mu.Unlock()
			if iInside(dr, x, y) {
				st.mu.Lock()
				st.showDetails = !st.showDetails
				st.mu.Unlock()
				pIInvalidateRect.Call(hwnd, 0, 0)
				return 0
			}
			if iInside(ar, x, y) {
				if status == "ready" {
					st.mu.Lock()
					st.status = "running"
					st.step = "Installation wird vorbereitet"
					st.detail = "LogiMate richtet die Anwendung sicher ein."
					st.mu.Unlock()
					pIInvalidateRect.Call(hwnd, 0, 0)
					go st.startWork()
				} else if status == "success" || status == "error" {
					pIDestroyWindow.Call(hwnd)
				}
			}
		}
		return 0
	case iWM_KEYDOWN:
		if wParam == iVK_ESCAPE && st != nil {
			st.mu.Lock()
			running := st.status == "running"
			st.mu.Unlock()
			if !running {
				pIDestroyWindow.Call(hwnd)
			}
		}
		return 0
	case iWM_CLOSE:
		if st != nil {
			st.mu.Lock()
			running := st.status == "running"
			st.mu.Unlock()
			if running {
				return 0
			}
		}
		pIDestroyWindow.Call(hwnd)
		return 0
	case iWM_PROGRESS, iWM_DONE:
		pIInvalidateRect.Call(hwnd, 0, 0)
		return 0
	case iWM_DESTROY:
		pIPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pIDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func paintInstaller(st *installerUIState) {
	var ps iPAINTSTRUCT
	hdc, _, _ := pIBeginPaint.Call(st.hwnd, uintptr(unsafe.Pointer(&ps)))
	defer pIEndPaint.Call(st.hwnd, uintptr(unsafe.Pointer(&ps)))
	var rc iRECT
	pIGetClientRect.Call(st.hwnd, uintptr(unsafe.Pointer(&rc)))
	bg := iRGB(24, 27, 32)
	panel := iRGB(34, 38, 45)
	panel2 := iRGB(42, 47, 55)
	border := iRGB(64, 70, 80)
	text := iRGB(238, 241, 245)
	muted := iRGB(165, 174, 186)
	accent := iRGB(79, 145, 255)
	bad := iRGB(222, 82, 82)
	good := iRGB(77, 185, 122)
	// Fill background with a giant solid surface to stay dependency-light.
	iRound(hdc, iRECT{0, 0, rc.Right + 4, rc.Bottom + 4}, 0, bg, bg)
	st.mu.Lock()
	status, progress, step, detail, show, hover := st.status, st.progress, st.step, st.detail, st.showDetails, st.hover
	logs := append([]string(nil), st.logs...)
	commands := append([]string(nil), st.commands...)
	st.mu.Unlock()

	heading := "LogiMate installieren"
	subtitle := "Moderner Windows-Installer · sicherer Rollback-Pfad"
	if st.update {
		heading = "LogiMate aktualisieren"
		subtitle = "Update · vorhandene Version bleibt bis zur Aktivierung als Rückfallebene erhalten"
	}
	iText(hdc, heading, iRECT{34, 28, rc.Right - 34, 70}, iDT_LEFT|iDT_VCENTER|iDT_SINGLELINE|iDT_NOPREFIX, iFontTitle, text)
	iText(hdc, subtitle, iRECT{36, 72, rc.Right - 34, 103}, iDT_LEFT|iDT_VCENTER|iDT_SINGLELINE|iDT_NOPREFIX|iDT_END_ELLIPSIS, iFontSmall, muted)
	card := iRECT{30, 118, rc.Right - 30, rc.Bottom - 95}
	iRound(hdc, card, 18, panel, border)
	ac := accent
	if status == "error" {
		ac = bad
	} else if status == "success" {
		ac = good
	}
	iRound(hdc, iRECT{card.Left + 20, card.Top + 22, card.Left + 25, card.Bottom - 22}, 3, ac, ac)
	if status == "ready" {
		step = "Bereit zur Installation"
		detail = "LogiMate wird nach Program Files kopiert. Desktop-/Startmenü-Verknüpfungen und der Deinstallations-Eintrag werden eingerichtet."
	}
	iText(hdc, step, iRECT{card.Left + 44, card.Top + 26, card.Right - 34, card.Top + 58}, iDT_LEFT|iDT_VCENTER|iDT_SINGLELINE|iDT_NOPREFIX|iDT_END_ELLIPSIS, iFontBody, text)
	iText(hdc, detail, iRECT{card.Left + 44, card.Top + 62, card.Right - 34, card.Top + 116}, iDT_LEFT|iDT_WORDBREAK|iDT_NOPREFIX, iFontSmall, muted)
	bar := iRECT{card.Left + 44, card.Top + 132, card.Right - 44, card.Top + 150}
	iRound(hdc, bar, 9, panel2, panel2)
	fillR := bar.Left + int32(float64(bar.Right-bar.Left)*float64(progress)/100)
	if fillR > bar.Left {
		iRound(hdc, iRECT{bar.Left, bar.Top, fillR, bar.Bottom}, 9, ac, ac)
	}
	iText(hdc, fmt.Sprintf("%d %%", progress), iRECT{bar.Left, bar.Bottom + 6, bar.Right, bar.Bottom + 32}, iDT_RIGHT|iDT_VCENTER|iDT_SINGLELINE|iDT_NOPREFIX, iFontSmall, muted)

	detailRect := iRECT{card.Left + 44, card.Top + 190, card.Left + 290, card.Top + 232}
	iRound(hdc, detailRect, 12, func() uintptr {
		if show {
			return panel2
		}
		return panel
	}(), func() uintptr {
		if show {
			return accent
		}
		return border
	}())
	label := "Befehlsansicht anzeigen"
	if show {
		label = "Befehlsansicht ausblenden"
	}
	iText(hdc, label, detailRect, iDT_CENTER|iDT_VCENTER|iDT_SINGLELINE|iDT_NOPREFIX, iFontSmall, func() uintptr {
		if show {
			return accent
		}
		return text
	}())
	if show {
		logRect := iRECT{card.Left + 44, card.Top + 248, card.Right - 44, card.Bottom - 24}
		iRound(hdc, logRect, 12, panel2, border)
		lines := append([]string{}, logs...)
		lines = append(lines, commands...)
		if len(lines) > 12 {
			lines = lines[len(lines)-12:]
		}
		tx := strings.Join(lines, "\r\n")
		if tx == "" {
			tx = "Noch keine Befehle ausgeführt."
		}
		iText(hdc, tx, iRECT{logRect.Left + 14, logRect.Top + 12, logRect.Right - 14, logRect.Bottom - 12}, iDT_LEFT|iDT_WORDBREAK|iDT_NOPREFIX|iDT_END_ELLIPSIS, iFontSmall, muted)
	} else {
		y := card.Top + 258
		start := 0
		if len(logs) > 5 {
			start = len(logs) - 5
		}
		for _, line := range logs[start:] {
			iText(hdc, line, iRECT{card.Left + 46, y, card.Right - 44, y + 30}, iDT_LEFT|iDT_VCENTER|iDT_SINGLELINE|iDT_NOPREFIX|iDT_END_ELLIPSIS, iFontSmall, muted)
			y += 32
		}
	}
	action := iRECT{rc.Right - 240, rc.Bottom - 70, rc.Right - 30, rc.Bottom - 24}
	actionText := "Installieren"
	actionFill := accent
	actionColor := iRGB(255, 255, 255)
	if st.update && status == "ready" {
		actionText = "Aktualisieren"
	}
	if status == "running" {
		actionText = "Installation läuft …"
		actionFill = panel2
		actionColor = muted
	}
	if status == "success" {
		actionText = "Schließen"
		actionFill = good
	}
	if status == "error" {
		actionText = "Schließen"
		actionFill = bad
	}
	if hover == 0 && status != "running" {
		actionFill = func() uintptr {
			if status == "success" {
				return iRGB(67, 165, 110)
			}
			if status == "error" {
				return iRGB(202, 72, 72)
			}
			return iRGB(67, 130, 235)
		}()
	}
	iRound(hdc, action, 14, actionFill, actionFill)
	iText(hdc, actionText, action, iDT_CENTER|iDT_VCENTER|iDT_SINGLELINE|iDT_NOPREFIX, iFontBody, actionColor)
	st.mu.Lock()
	st.actionRect = action
	st.detailRect = detailRect
	st.mu.Unlock()
}
