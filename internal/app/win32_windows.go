//go:build windows

package app

import (
	"sync"
	"syscall"
	"unsafe"
)

type HWND uintptr
type HINSTANCE uintptr
type HICON uintptr
type HCURSOR uintptr
type HBRUSH uintptr
type HFONT uintptr
type HMENU uintptr

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	Hwnd           HWND
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
	LPrivate       uint32
}
type WNDCLASSEX struct {
	CbSize                      uint32
	Style                       uint32
	LpfnWndProc                 uintptr
	CbClsExtra, CbWndExtra      int32
	HInstance                   HINSTANCE
	HIcon                       HICON
	HCursor                     HCURSOR
	HbrBackground               HBRUSH
	LpszMenuName, LpszClassName *uint16
	HIconSm                     HICON
}
type PAINTSTRUCT struct {
	Hdc                  uintptr
	FErase               int32
	RcPaint              RECT
	FRestore, FIncUpdate int32
	RgbReserved          [32]byte
}
type TRACKMOUSEEVENT struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   HWND
	DwHoverTime uint32
}
type MARGINS struct {
	CxLeftWidth, CxRightWidth, CyTopHeight, CyBottomHeight int32
}

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type RGBQUAD struct {
	Blue, Green, Red, Reserved byte
}

type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]RGBQUAD
}

type HIGHCONTRASTW struct {
	CbSize            uint32
	DwFlags           uint32
	LpszDefaultScheme *uint16
}

type MINMAXINFO struct {
	PtReserved     POINT
	PtMaxSize      POINT
	PtMaxPosition  POINT
	PtMinTrackSize POINT
	PtMaxTrackSize POINT
}

type OSVERSIONINFOEXW struct {
	DwOSVersionInfoSize uint32
	DwMajorVersion      uint32
	DwMinorVersion      uint32
	DwBuildNumber       uint32
	DwPlatformID        uint32
	SzCSDVersion        [128]uint16
	WServicePackMajor   uint16
	WServicePackMinor   uint16
	WSuiteMask          uint16
	WProductType        byte
	WReserved           byte
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ntdll    = syscall.NewLazyDLL("ntdll.dll")

	pRegisterClassExW              = user32.NewProc("RegisterClassExW")
	pCreateWindowExW               = user32.NewProc("CreateWindowExW")
	pDefWindowProcW                = user32.NewProc("DefWindowProcW")
	pShowWindow                    = user32.NewProc("ShowWindow")
	pUpdateWindow                  = user32.NewProc("UpdateWindow")
	pGetMessageW                   = user32.NewProc("GetMessageW")
	pTranslateMessage              = user32.NewProc("TranslateMessage")
	pDispatchMessageW              = user32.NewProc("DispatchMessageW")
	pPostQuitMessage               = user32.NewProc("PostQuitMessage")
	pPostMessageW                  = user32.NewProc("PostMessageW")
	pSendMessageW                  = user32.NewProc("SendMessageW")
	pDestroyWindow                 = user32.NewProc("DestroyWindow")
	pSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	pMoveWindow                    = user32.NewProc("MoveWindow")
	pInvalidateRect                = user32.NewProc("InvalidateRect")
	pBeginPaint                    = user32.NewProc("BeginPaint")
	pEndPaint                      = user32.NewProc("EndPaint")
	pFillRect                      = user32.NewProc("FillRect")
	pDrawTextW                     = user32.NewProc("DrawTextW")
	pSetTimer                      = user32.NewProc("SetTimer")
	pKillTimer                     = user32.NewProc("KillTimer")
	pMessageBoxW                   = user32.NewProc("MessageBoxW")
	pLoadCursorW                   = user32.NewProc("LoadCursorW")
	pLoadIconW                     = user32.NewProc("LoadIconW")
	pCreateIconFromResourceEx      = user32.NewProc("CreateIconFromResourceEx")
	pDrawIconEx                    = user32.NewProc("DrawIconEx")
	pDestroyIcon                   = user32.NewProc("DestroyIcon")
	pGetClientRect                 = user32.NewProc("GetClientRect")
	pGetWindowRect                 = user32.NewProc("GetWindowRect")
	pEnableWindow                  = user32.NewProc("EnableWindow")
	pSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pOpenClipboard                 = user32.NewProc("OpenClipboard")
	pEmptyClipboard                = user32.NewProc("EmptyClipboard")
	pSetClipboardData              = user32.NewProc("SetClipboardData")
	pCloseClipboard                = user32.NewProc("CloseClipboard")
	pTrackMouseEvent               = user32.NewProc("TrackMouseEvent")
	pSetCursor                     = user32.NewProc("SetCursor")
	pSetCapture                    = user32.NewProc("SetCapture")
	pReleaseCapture                = user32.NewProc("ReleaseCapture")
	pGetKeyState                   = user32.NewProc("GetKeyState")
	pSystemParametersInfoW         = user32.NewProc("SystemParametersInfoW")
	pGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	pGetSysColor                   = user32.NewProc("GetSysColor")
	pGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	pSetWindowLongPtrW             = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcW               = user32.NewProc("CallWindowProcW")
	pSetLayeredWindowAttributes    = user32.NewProc("SetLayeredWindowAttributes")
	pSetWindowTextW                = user32.NewProc("SetWindowTextW")
	pSetFocus                      = user32.NewProc("SetFocus")

	pGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	pGetModuleFileNameW  = kernel32.NewProc("GetModuleFileNameW")
	pGlobalAlloc         = kernel32.NewProc("GlobalAlloc")
	pGlobalLock          = kernel32.NewProc("GlobalLock")
	pGlobalUnlock        = kernel32.NewProc("GlobalUnlock")
	pGlobalFree          = kernel32.NewProc("GlobalFree")
	pOpenProcess         = kernel32.NewProc("OpenProcess")
	pWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	pCloseHandle         = kernel32.NewProc("CloseHandle")

	pCreateSolidBrush   = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject       = gdi32.NewProc("DeleteObject")
	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pDeleteDC           = gdi32.NewProc("DeleteDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pBitBlt             = gdi32.NewProc("BitBlt")
	pSetTextColor       = gdi32.NewProc("SetTextColor")
	pSetBkMode          = gdi32.NewProc("SetBkMode")
	pSaveDC             = gdi32.NewProc("SaveDC")
	pRestoreDC          = gdi32.NewProc("RestoreDC")
	pIntersectClipRect  = gdi32.NewProc("IntersectClipRect")
	pCreateFontW        = gdi32.NewProc("CreateFontW")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pRoundRect          = gdi32.NewProc("RoundRect")
	pEllipse            = gdi32.NewProc("Ellipse")
	pMoveToEx           = gdi32.NewProc("MoveToEx")
	pLineTo             = gdi32.NewProc("LineTo")
	pCreatePen          = gdi32.NewProc("CreatePen")

	pDwmSetWindowAttribute        = dwmapi.NewProc("DwmSetWindowAttribute")
	pDwmExtendFrameIntoClientArea = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	pShellExecuteW                = shell32.NewProc("ShellExecuteW")
	pIsUserAnAdmin                = shell32.NewProc("IsUserAnAdmin")
	pRtlGetVersion                = ntdll.NewProc("RtlGetVersion")
)

const (
	DI_NORMAL                  = 0x0003
	LR_DEFAULTCOLOR            = 0x0000
	WS_OVERLAPPEDWINDOW        = 0x00CF0000
	WS_CLIPCHILDREN            = 0x02000000
	WS_VISIBLE                 = 0x10000000
	WS_CHILD                   = 0x40000000
	WS_TABSTOP                 = 0x00010000
	WS_DISABLED                = 0x08000000
	WS_EX_LAYERED              = 0x00080000
	WS_EX_TRANSPARENT          = 0x00000020
	BS_PUSHBUTTON              = 0x00000000
	BS_AUTOCHECKBOX            = 0x00000003
	BS_AUTORADIOBUTTON         = 0x00000009
	LWA_ALPHA                  = 0x00000002
	GWLP_WNDPROC               = -4
	WM_CREATE                  = 0x0001
	WM_DESTROY                 = 0x0002
	WM_QUERYENDSESSION         = 0x0011
	WM_ENDSESSION              = 0x0016
	WM_POWERBROADCAST          = 0x0218
	PBT_APMSUSPEND             = 0x0004
	PBT_APMSTANDBY             = 0x0005
	PBT_APMRESUMECRITICAL      = 0x0006
	PBT_APMRESUMESUSPEND       = 0x0007
	PBT_APMRESUMEAUTOMATIC     = 0x0012
	WM_PAINT                   = 0x000F
	WM_CLOSE                   = 0x0010
	WM_CANCELMODE              = 0x001F
	WM_ERASEBKGND              = 0x0014
	WM_KEYDOWN                 = 0x0100
	WM_COMMAND                 = 0x0111
	WM_NCHITTEST               = 0x0084
	WM_SETFOCUS                = 0x0007
	WM_GETMINMAXINFO           = 0x0024
	WM_TIMER                   = 0x0113
	WM_SIZE                    = 0x0005
	WM_MOUSEMOVE               = 0x0200
	WM_LBUTTONDOWN             = 0x0201
	WM_LBUTTONUP               = 0x0202
	WM_MOUSEWHEEL              = 0x020A
	WM_CAPTURECHANGED          = 0x0215
	WM_MOUSELEAVE              = 0x02A3
	WM_DWMCOMPOSITIONCHANGED   = 0x031E
	WM_DPICHANGED              = 0x02E0
	WM_SETTINGCHANGE           = 0x001A
	WM_DEVICECHANGE            = 0x0219
	WM_APP                     = 0x8000
	SW_HIDE                    = 0
	SW_SHOW                    = 5
	SW_SHOWNORMAL              = 1
	SW_SHOWNA                  = 8
	DT_LEFT                    = 0x0000
	DT_CENTER                  = 0x0001
	DT_RIGHT                   = 0x0002
	DT_VCENTER                 = 0x0004
	DT_SINGLELINE              = 0x0020
	DT_WORDBREAK               = 0x0010
	DT_CALCRECT                = 0x0400
	DT_NOPREFIX                = 0x0800
	DT_END_ELLIPSIS            = 0x8000
	TRANSPARENT                = 1
	MB_OK                      = 0x00000000
	MB_ICONINFORMATION         = 0x00000040
	MB_ICONWARNING             = 0x00000030
	MB_ICONERROR               = 0x00000010
	MB_YESNO                   = 0x00000004
	MB_DEFBUTTON2              = 0x00000100
	IDOK                       = 1
	IDYES                      = 6
	IDNO                       = 7
	IDCANCEL                   = 2
	BM_SETCHECK                = 0x00F1
	BST_UNCHECKED              = 0
	BST_CHECKED                = 1
	IDC_ARROW                  = 32512
	IDC_HAND                   = 32649
	VK_ESCAPE                  = 0x1B
	VK_TAB                     = 0x09
	VK_RETURN                  = 0x0D
	VK_SPACE                   = 0x20
	VK_SHIFT                   = 0x10
	VK_LEFT                    = 0x25
	VK_UP                      = 0x26
	VK_RIGHT                   = 0x27
	VK_DOWN                    = 0x28
	IDI_APPLICATION            = 32512
	GMEM_MOVEABLE              = 0x0002
	CF_UNICODETEXT             = 13
	PS_SOLID                   = 0
	TME_LEAVE                  = 0x00000002
	SRCCOPY                    = 0x00CC0020
	BI_RGB                     = 0
	DIB_RGB_COLORS             = 0
	SPI_GETHIGHCONTRAST        = 0x0042
	SPI_GETCLIENTAREAANIMATION = 0x1042
	HCF_HIGHCONTRASTON         = 0x00000001
	COLOR_WINDOW               = 5
	COLOR_WINDOWTEXT           = 8
	COLOR_HIGHLIGHT            = 13
	COLOR_HIGHLIGHTTEXT        = 14
	COLOR_BTNFACE              = 15
	COLOR_BTNTEXT              = 18
	SM_CXSCREEN                = 0
	SM_CYSCREEN                = 1
	SM_CMONITORS               = 80
	SYNCHRONIZE                = 0x00100000
	INFINITE                   = 0xFFFFFFFF
)

func rgb(r, g, b byte) uintptr     { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }
func utf16(s string) *uint16       { p, _ := syscall.UTF16PtrFromString(s); return p }
func loWord(v uintptr) uint16      { return uint16(v & 0xffff) }
func hiWord(v uintptr) uint16      { return uint16((v >> 16) & 0xffff) }
func signedLoWord(v uintptr) int32 { return int32(int16(loWord(v))) }
func signedHiWord(v uintptr) int32 { return int32(int16(hiWord(v))) }

func createSolidBrush(c uintptr) HBRUSH { r, _, _ := pCreateSolidBrush.Call(c); return HBRUSH(r) }
func deleteObject(h uintptr) {
	if h != 0 {
		pDeleteObject.Call(h)
	}
}
func postMessage(hwnd HWND, msg uint32, wp, lp uintptr) {
	pPostMessageW.Call(uintptr(hwnd), uintptr(msg), wp, lp)
}
func invalidate(hwnd HWND) { pInvalidateRect.Call(uintptr(hwnd), 0, 0) }
func invalidateArea(hwnd HWND, r RECT) {
	if hwnd == 0 || r.Right <= r.Left || r.Bottom <= r.Top {
		return
	}
	pInvalidateRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)), 0)
}
func messageBox(hwnd HWND, text, title string, flags uint32) int {
	// All app-level prompts now use LogiMate's own Acrylic/Mica dialog surface.
	// This removes the old mix of MessageBox/TaskDialog behavior and, more
	// importantly, avoids TaskDialogIndirect availability mistakes from silently
	// swallowing actions on normal Windows 11 systems.
	if result, ok := modernMessageBox(hwnd, text, title, flags); ok {
		return result
	}
	// Last-resort fallback for environments where the custom window class cannot
	// be registered (for example a severely stripped recovery environment).
	r, _, _ := pMessageBoxW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(utf16(text))), uintptr(unsafe.Pointer(utf16(title))), uintptr(flags))
	return int(r)
}

func setClipboardText(hwnd HWND, s string) bool {
	if r, _, _ := pOpenClipboard.Call(uintptr(hwnd)); r == 0 {
		return false
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	u, _ := syscall.UTF16FromString(s)
	size := uintptr(len(u) * 2)
	h, _, _ := pGlobalAlloc.Call(GMEM_MOVEABLE, size)
	if h == 0 {
		return false
	}
	owned := true
	defer func() {
		if owned {
			pGlobalFree.Call(h)
		}
	}()
	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return false
	}
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(u))
	copy(dst, u)
	pGlobalUnlock.Call(h)
	r, _, _ := pSetClipboardData.Call(CF_UNICODETEXT, h)
	if r == 0 {
		return false
	}
	// After successful SetClipboardData the system owns the HGLOBAL.
	owned = false
	return true
}

func currentExe() string {
	buf := make([]uint16, 32768)
	n, _, _ := pGetModuleFileNameW.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:n])
}

func shellRunAs(exe, args string) bool {
	r, _, _ := pShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16("runas"))), uintptr(unsafe.Pointer(utf16(exe))), uintptr(unsafe.Pointer(utf16(args))), 0, SW_SHOWNORMAL)
	return r > 32
}

func isAdmin() bool { r, _, _ := pIsUserAnAdmin.Call(); return r != 0 }

func setCapture(hwnd HWND) {
	if hwnd != 0 {
		pSetCapture.Call(uintptr(hwnd))
	}
}

func releaseCapture() {
	pReleaseCapture.Call()
}

func trackMouseLeave(hwnd HWND) {
	t := TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), DwFlags: TME_LEAVE, HwndTrack: hwnd}
	pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&t)))
}

func setCursorHand(hand bool) {
	id := uintptr(IDC_ARROW)
	if hand {
		id = IDC_HAND
	}
	c, _, _ := pLoadCursorW.Call(0, id)
	pSetCursor.Call(c)
}

var (
	windowsBuildOnce sync.Once
	windowsBuild     uint32
)

func windowsBuildNumber() uint32 {
	windowsBuildOnce.Do(func() {
		var v OSVERSIONINFOEXW
		v.DwOSVersionInfoSize = uint32(unsafe.Sizeof(v))
		r, _, _ := pRtlGetVersion.Call(uintptr(unsafe.Pointer(&v)))
		if r == 0 {
			windowsBuild = v.DwBuildNumber
		}
	})
	return windowsBuild
}

func isWindowsBuildAtLeast(build uint32) bool {
	b := windowsBuildNumber()
	return b != 0 && b >= build
}

func highContrastEnabled() bool {
	hc := HIGHCONTRASTW{CbSize: uint32(unsafe.Sizeof(HIGHCONTRASTW{}))}
	r, _, _ := pSystemParametersInfoW.Call(SPI_GETHIGHCONTRAST, uintptr(hc.CbSize), uintptr(unsafe.Pointer(&hc)), 0)
	return r != 0 && hc.DwFlags&HCF_HIGHCONTRASTON != 0
}

func clientAnimationsEnabled() bool {
	var enabled int32
	r, _, _ := pSystemParametersInfoW.Call(SPI_GETCLIENTAREAANIMATION, 0, uintptr(unsafe.Pointer(&enabled)), 0)
	if r == 0 {
		return true
	}
	return enabled != 0
}

func systemColor(index uintptr) uintptr { r, _, _ := pGetSysColor.Call(index); return r }

func windowDPI(hwnd HWND) uint32 {
	if hwnd == 0 || pGetDpiForWindow.Find() != nil {
		return 96
	}
	r, _, _ := pGetDpiForWindow.Call(uintptr(hwnd))
	if r == 0 {
		return 96
	}
	return uint32(r)
}

func monitorCount() int {
	r, _, _ := pGetSystemMetrics.Call(SM_CMONITORS)
	if r == 0 {
		return 1
	}
	return int(r)
}

func waitForProcessExit(pid uint32) {
	if pid == 0 {
		return
	}
	h, _, _ := pOpenProcess.Call(SYNCHRONIZE, 0, uintptr(pid))
	if h == 0 {
		return
	}
	defer pCloseHandle.Call(h)
	pWaitForSingleObject.Call(h, INFINITE)
}
