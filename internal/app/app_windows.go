//go:build windows

package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/thelittlespace/LogiMate/internal/system"
)

const (
	pageOverview = iota
	pageWheel
	pageSystem
	pageDiagnostics
	pageSettings
	pageAbout
)

const (
	idAction1 = 2001 + iota
	idAction2
	idAction3
	idAction4
)

const (
	timerJoy              = 1
	timerRefresh          = 2
	timerAnim             = 3
	timerMaterial         = 4
	timerStartupStable    = 5
	timerSmoke            = 6
	timerFeedback         = 7
	timerDeviceRefresh    = 8
	msgStateReady         = WM_APP + 1
	msgOperationText      = WM_APP + 2
	msgRefreshRequest     = WM_APP + 4
	msgNotice             = WM_APP + 5
	msgUpdateFound        = WM_APP + 6
	msgUpdateDownloaded   = WM_APP + 7
	msgSetupMigrationDone = WM_APP + 8
)

const (
	sidebarCollapsed int32 = 82
	sidebarExpanded  int32 = 228
	navTop           int32 = 112
	navHeight        int32 = 48
	navGap           int32 = 8
)

type actionButton struct {
	label   string
	icon    string
	visible bool
	enabled bool
	primary bool
	warning bool
}

type uiNotice struct {
	text  string
	title string
	flags uint32
}

type navItem struct {
	page  int
	icon  string
	title string
}

var Version = "dev"
var BuildID = "dev"

var navItems = []navItem{
	{pageOverview, "\uE80F", "Startseite"},
	{pageWheel, "\uE7FC", "Lenkrad"},
	{pageSystem, "\uE90F", "System"},
	{pageDiagnostics, "\uE9D9", "Diagnose"},
	{pageAbout, "\uE77B", "Über mich"},
	{pageSettings, "\uE713", "Einstellungen"},
}

var (
	mainWnd     HWND
	currentPage = pageOverview

	actionButtons           [4]actionButton
	actionRects             [4]RECT
	hoveredAction           = -1
	pressedAction           = -1
	wheelAdvancedToggleRect RECT
	wheelAdvancedHovered    bool
	memoryIntegrityRect     RECT
	memoryIntegrityHovered  bool

	stateMu  sync.RWMutex
	appState system.State
	bodyMu   sync.RWMutex
	pageBody string

	busy bool

	noticeMu sync.Mutex
	notices  []uiNotice

	sidebarWidth           int32  = sidebarExpanded
	sidebarTarget          int32  = sidebarExpanded
	hoveredNav                    = -1
	hoveredSetting                = -1
	currentDPI             uint32 = 96
	settingRects                  = make([]RECT, 0)
	mouseTracked           bool
	pageAnim               float64 = 1
	animRunning            bool
	contentScroll          int32
	contentScrollMax       int32
	glassActive            bool
	materialStarted        bool
	safeUI                 bool
	smokeTest              bool
	startupMarker          string
	startupLog             string
	startupLogMu           sync.Mutex
	startupStableReached   bool
	startupActionsDone     bool
	runtimeRecoveryChecked bool
	manualRefreshPending   bool
	deviceRefreshPending   bool
	actionFeedback         string

	bgBrush        HBRUSH
	sidebarBrush   HBRUSH
	glassBrush     HBRUSH
	backDC         uintptr
	backBitmap     uintptr
	backOldBitmap  uintptr
	backBits       uintptr
	backW          int32
	backH          int32
	rendererStatus = "GDI (opaque)"
	rendererDetail string
	roundBrushes   = map[uintptr]HBRUSH{}
	roundPens      = map[uintptr]uintptr{}
	fontBody       HFONT
	fontTitle      HFONT
	fontSubtitle   HFONT
	fontSmall      HFONT
	fontBrand      HFONT
	fontIcon       HFONT
)

func appStateSnapshot() system.State {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return appState
}

var (
	colText           = rgb(240, 243, 248)
	colMuted          = rgb(159, 167, 180)
	colMuted2         = rgb(132, 142, 158)
	colAccent         = rgb(92, 174, 255)
	colAccentSoft     = rgb(40, 79, 116)
	colPanel          = rgb(29, 32, 38)
	colPanel2         = rgb(35, 39, 47)
	colHover          = rgb(43, 48, 57)
	colSelected       = rgb(37, 68, 96)
	colWarning        = rgb(255, 190, 92)
	colWarningSurface = rgb(107, 73, 31)
	colGood           = rgb(96, 205, 145)
	colBad            = rgb(242, 113, 116)
)

func Run() (exitCode int) {
	// Native Win32 windows, callbacks and the message pump are thread-affine.
	// Keep the entire UI lifecycle on one OS thread; without this Go may move
	// the goroutine between threads and cause sporadic startup freezes.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if handled, code := runUpdateHelperIfRequested(); handled {
		return code
	}

	releaseInstance, err := system.AcquireApplicationInstanceLock()
	if err != nil {
		messageBox(0, "LogiMate läuft bereits oder die globale Instanzsperre konnte nicht sicher übernommen werden.\r\n\r\n"+err.Error(), "LogiMate", MB_OK|MB_ICONWARNING)
		return 4
	}
	defer releaseInstance()

	dataDir, _ := system.GetDataDir()
	if err := system.EnsureDataSchema(dataDir); err != nil {
		messageBox(0, "LogiMate konnte die Datenmigration nicht sicher vorbereiten:\r\n\r\n"+err.Error(), "LogiMate · Datenmigration", MB_OK|MB_ICONERROR)
		return 98
	}
	if notes, err := system.RecoverInterruptedMigrations(dataDir); err != nil {
		detail := strings.Join(notes, "\r\n")
		if detail != "" {
			detail += "\r\n\r\n"
		}
		messageBox(0, "LogiMate hat eine unvollständige frühere Modusmigration erkannt. Neue Moduswechsel bleiben gesperrt, bis der Zustand eindeutig geklärt ist.\r\n\r\n"+detail+err.Error(), "LogiMate · Migrations-Recovery", MB_OK|MB_ICONWARNING)
	}
	uiPrefs = loadUISettings(dataDir)
	if uiSettingsLoadWarning != "" {
		messageBox(0, "Die UI-Einstellungen konnten nicht vollständig im normalen Schreibmodus geladen werden. LogiMate bewahrt die vorhandene Datei und überschreibt sie nicht automatisch.\r\n\r\n"+uiSettingsLoadWarning+"\r\n\r\nÜber 'Einstellungen zurücksetzen' kannst du bewusst eine neue saubere Datei erzeugen.", "LogiMate · Einstellungen geschützt", MB_OK|MB_ICONWARNING)
	}
	if uiPrefs.SidebarAutoExpand {
		sidebarWidth, sidebarTarget = sidebarExpanded, sidebarExpanded
	} else {
		sidebarWidth, sidebarTarget = sidebarCollapsed, sidebarCollapsed
	}
	setThemeColors(uiPrefs.ThemeMode)
	initUpdateStartupFlags()
	startupLog = filepath.Join(dataDir, "startup.log")
	rotateLogFile(startupLog, 1<<20, 3)
	startupMarker = filepath.Join(dataDir, "startup.pending")

	// If the previous process never reached the stable-start timer, prefer a
	// conservative opaque UI on the next launch. This avoids repeatedly hanging
	// on a problematic DWM / graphics-driver path.
	if _, err := os.Stat(startupMarker); err == nil {
		safeUI = true
	}
	for _, a := range os.Args[1:] {
		switch a {
		case "--safe-ui":
			safeUI = true
		case "--smoke-test":
			safeUI = true
			smokeTest = true
		}
	}
	_ = os.WriteFile(startupMarker, []byte(time.Now().Format(time.RFC3339)), 0644)
	logStartup("Run entered; safeUI=%v smokeTest=%v acrylic=%v animations=%v sidebarAutoExpand=%v autoRefresh=%v", safeUI, smokeTest, uiPrefs.Acrylic, uiPrefs.Animations, uiPrefs.SidebarAutoExpand, uiPrefs.AutoRefresh)

	defer func() {
		if r := recover(); r != nil {
			logStartup("PANIC: %v", r)
			messageBox(0, fmt.Sprintf("LogiMate konnte nicht vollständig starten.\r\n\r\n%v\r\n\r\nStartprotokoll:\r\n%s\r\n\r\nStarte bei Bedarf mit --safe-ui.", r, startupLog), "LogiMate Startfehler", MB_OK|MB_ICONERROR)
			exitCode = 99
		}
	}()

	// Prefer Per-Monitor V2 so Windows never bitmap-scales our GDI text when
	// moving between mixed-DPI displays. Older Windows versions fall back to
	// classic system DPI awareness.
	dpiV2 := false
	if pSetProcessDpiAwarenessContext.Find() == nil {
		ctx := ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == (HANDLE)-4
		if r, _, _ := pSetProcessDpiAwarenessContext.Call(ctx); r != 0 {
			dpiV2 = true
		}
	}
	if !dpiV2 {
		pSetProcessDPIAware.Call()
	}
	logStartup("DPI awareness requested; perMonitorV2=%v", dpiV2)

	hinst, _, _ := pGetModuleHandleW.Call(0)
	className := utf16("LogiMateWindow")
	cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	icon := brandIcon(64)
	if icon == 0 {
		fallback, _, _ := pLoadIconW.Call(0, IDI_APPLICATION)
		icon = HICON(fallback)
	}
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:         0x0002 | 0x0001,
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     HINSTANCE(hinst),
		HIcon:         icon,
		HCursor:       HCURSOR(cursor),
		HbrBackground: 0,
		LpszClassName: className,
		HIconSm:       icon,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		logStartup("RegisterClassExW failed: %v", err)
		return 2
	}
	logStartup("Window class registered")

	// GDI resources are initialized explicitly during Run rather than at package
	// initialization time. This keeps DLL/GDI work out of the pre-main path.
	bgBrush = createSolidBrush(themeBackground)
	sidebarBrush = createSolidBrush(themeSidebar)
	// Black is the documented glass-sheet clear color when the DWM frame is extended.
	// Clearing every frame prevents old antialiased glyphs from accumulating on Acrylic.
	glassBrush = createSolidBrush(rgb(0, 0, 0))
	fontBody = createFontName(-16, 400, "Segoe UI Variable Text")
	fontTitle = createFontName(-32, 600, "Segoe UI Variable Display")
	fontSubtitle = createFontName(-14, 400, "Segoe UI Variable Text")
	fontSmall = createFontName(-12, 400, "Segoe UI Variable Text")
	fontBrand = createFontName(-21, 600, "Segoe UI Variable Display")
	fontIcon = createFontName(-22, 400, "Segoe MDL2 Assets")
	fontSection = createFontName(-18, 600, "Segoe UI Variable Text")
	fontMetric = createFontName(-28, 600, "Segoe UI Variable Display")
	fontLabel = createFontName(-13, 600, "Segoe UI Variable Text")
	logStartup("GDI resources initialized")

	logStartup("CreateWindowExW begin")
	hwnd, _, err := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16("LogiMate"))),
		// D5.8 uses transparent native child HWNDs as UI Automation proxies for
		// the custom-painted surface. Do not use WS_CLIPCHILDREN here: the parent
		// must continue painting behind those fully transparent accessibility
		// controls.
		WS_OVERLAPPEDWINDOW,
		70, 45, 1380, 900,
		0, 0, hinst, 0,
	)
	if hwnd == 0 {
		logStartup("CreateWindowExW failed: %v", err)
		return 3
	}
	mainWnd = HWND(hwnd)
	currentDPI = windowDPI(mainWnd)
	if currentDPI == 0 {
		currentDPI = 96
	}
	rebuildFontsForDPI()
	applyWindowChromeTheme(mainWnd)
	logStartup("CreateWindowExW complete")

	// Show immediately. Acrylic and other cosmetic effects are intentionally
	// delayed until the message loop is already alive.
	pShowWindow.Call(hwnd, SW_SHOW)
	pUpdateWindow.Call(hwnd)
	logStartup("Window shown")

	pSetTimer.Call(hwnd, timerMaterial, 450, 0)
	pSetTimer.Call(hwnd, timerStartupStable, 1800, 0)
	if smokeTest {
		pSetTimer.Call(hwnd, timerSmoke, 2400, 0)
	}

	refreshAsync()
	logStartup("Async state refresh scheduled; entering message loop")

	var msg MSG
	for {
		r, _, err := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) == -1 {
			logStartup("GetMessageW failed: %v", err)
			break
		}
		if int32(r) == 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	if startupMarker != "" {
		_ = os.Remove(startupMarker)
	}
	cleanup()
	logStartup("Message loop ended")
	return int(msg.WParam)
}

func logStartup(format string, args ...any) {
	if startupLog == "" {
		return
	}
	startupLogMu.Lock()
	defer startupLogMu.Unlock()
	line := fmt.Sprintf("[%s] %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, args...))
	f, err := os.OpenFile(startupLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		_, _ = f.WriteString(line)
		_ = f.Close()
	}
}

func createFontName(height int32, weight int32, name string) HFONT {
	r, _, _ := pCreateFontW.Call(
		uintptr(height), 0, 0, 0, uintptr(weight),
		0, 0, 0, 1, 0, 0, 4, 0, // ANTIALIASED_QUALITY: crisp on translucent Acrylic; avoids ClearType ghosting
		uintptr(unsafe.Pointer(utf16(name))),
	)
	return HFONT(r)
}

func rebuildFontsForDPI() {
	dpi := int32(currentDPI)
	if dpi <= 0 {
		dpi = 96
	}
	scale := func(v int32) int32 { return v * dpi / 96 }
	for _, f := range []HFONT{fontBody, fontTitle, fontSubtitle, fontSmall, fontBrand, fontIcon, fontSection, fontMetric, fontLabel} {
		if f != 0 {
			deleteObject(uintptr(f))
		}
	}
	fontBody = createFontName(scale(-16), 400, "Segoe UI Variable Text")
	fontTitle = createFontName(scale(-32), 600, "Segoe UI Variable Display")
	fontSubtitle = createFontName(scale(-14), 400, "Segoe UI Variable Text")
	fontSmall = createFontName(scale(-12), 400, "Segoe UI Variable Text")
	fontBrand = createFontName(scale(-21), 600, "Segoe UI Variable Display")
	fontIcon = createFontName(scale(-22), 400, "Segoe MDL2 Assets")
	fontSection = createFontName(scale(-18), 600, "Segoe UI Variable Text")
	fontMetric = createFontName(scale(-28), 600, "Segoe UI Variable Display")
	fontLabel = createFontName(scale(-13), 600, "Segoe UI Variable Text")
}

func applyWindowChromeTheme(hwnd HWND) {
	if hwnd == 0 || !isWindowsBuildAtLeast(22000) {
		return
	}
	dark := int32(0)
	if themeUsesDarkChrome() {
		dark = 1
	}
	_, _, _ = pDwmSetWindowAttribute.Call(uintptr(hwnd), 20, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark)) // DWMWA_USE_IMMERSIVE_DARK_MODE

	// Windows 11 allows app-specific caption text and border colors. Setting
	// those roles prevents a Light LogiMate window from retaining dark chrome
	// after a theme switch, while Gray keeps a neutral graphite frame.
	border := uint32(colBorder)
	text := uint32(colText)
	_, _, _ = pDwmSetWindowAttribute.Call(uintptr(hwnd), 34, uintptr(unsafe.Pointer(&border)), unsafe.Sizeof(border)) // DWMWA_BORDER_COLOR
	_, _, _ = pDwmSetWindowAttribute.Call(uintptr(hwnd), 36, uintptr(unsafe.Pointer(&text)), unsafe.Sizeof(text))     // DWMWA_TEXT_COLOR
}

func setRedirectionBitmapAlpha(hwnd HWND, enabled bool) bool {
	// Windows 11 24H2 (26100+) can explicitly consume premultiplied alpha from
	// the redirected top-level bitmap. LogiMate only enables this on the main
	// window because its renderer owns a 32-bit DIB backbuffer there.
	if hwnd == 0 || hwnd != mainWnd || !isWindowsBuildAtLeast(26100) {
		return true
	}
	value := int32(0)
	if enabled {
		value = 1
	}
	r, _, _ := pDwmSetWindowAttribute.Call(uintptr(hwnd), 39, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value)) // DWMWA_REDIRECTIONBITMAP_ALPHA
	return r == 0
}

func resetWindowMaterial(hwnd HWND) {
	if hwnd == 0 || !isWindowsBuildAtLeast(22000) {
		return
	}
	_ = setRedirectionBitmapAlpha(hwnd, false)
	none := int32(1) // DWMSBT_NONE
	_, _, _ = pDwmSetWindowAttribute.Call(uintptr(hwnd), 38, uintptr(unsafe.Pointer(&none)), unsafe.Sizeof(none))
	margins := MARGINS{0, 0, 0, 0}
	_, _, _ = pDwmExtendFrameIntoClientArea.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&margins)))
	applyWindowChromeTheme(hwnd)
}

func applyWindowMaterial(hwnd HWND) bool {
	prefs := getUISettings()
	if hwnd == 0 {
		return false
	}

	applyWindowChromeTheme(hwnd)
	if isWindowsBuildAtLeast(22000) {
		round := int32(2) // DWMWCP_ROUND
		_, _, _ = pDwmSetWindowAttribute.Call(uintptr(hwnd), 33, uintptr(unsafe.Pointer(&round)), unsafe.Sizeof(round))
	}

	if safeUI || !prefs.Acrylic || highContrastEnabled() {
		logStartup("Window material skipped (safeUI=%v acrylic=%v highContrast=%v)", safeUI, prefs.Acrylic, highContrastEnabled())
		if hwnd == mainWnd {
			rendererStatus = "GDI (opaque)"
			if highContrastEnabled() {
				rendererDetail = "Windows-Material im Hochkontrastmodus deaktiviert"
			} else {
				rendererDetail = "Windows-Material deaktiviert"
			}
		}
		return false
	}
	if !isWindowsBuildAtLeast(22621) {
		logStartup("Window material skipped: Windows build %d < 22621", windowsBuildNumber())
		if hwnd == mainWnd {
			rendererStatus = "GDI (opaque fallback)"
			rendererDetail = fmt.Sprintf("Windows Build %d unterstützt DWMWA_SYSTEMBACKDROP_TYPE nicht", windowsBuildNumber())
		}
		return false
	}

	backdrop, materialName := materialBackdropForTheme(prefs.ThemeMode)
	r, _, err := pDwmSetWindowAttribute.Call(uintptr(hwnd), 38, uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
	if r != 0 {
		resetWindowMaterial(hwnd)
		logStartup("Windows material rejected by DWM (%s): HRESULT=0x%08X err=%v", materialName, uint32(r), err)
		if hwnd == mainWnd {
			rendererStatus = "GDI (opaque fallback)"
			rendererDetail = fmt.Sprintf("DWM lehnte %s ab (HRESULT 0x%08X)", materialName, uint32(r))
		}
		return false
	}

	// System backdrop draws the material for the complete window bounds, while
	// the legacy glass extension exposes transparent client pixels to DWM. The
	// renderer below provides a 32-bit DIB and repairs alpha before presentation.
	margins := MARGINS{-1, -1, -1, -1}
	r, _, err = pDwmExtendFrameIntoClientArea.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&margins)))
	if r != 0 {
		resetWindowMaterial(hwnd)
		logStartup("DwmExtendFrameIntoClientArea failed: HRESULT=0x%08X err=%v", uint32(r), err)
		if hwnd == mainWnd {
			rendererStatus = "GDI (opaque fallback)"
			rendererDetail = fmt.Sprintf("Glas-Client konnte nicht aktiviert werden (HRESULT 0x%08X)", uint32(r))
		}
		return false
	}

	if hwnd == mainWnd {
		if !setRedirectionBitmapAlpha(hwnd, true) {
			resetWindowMaterial(hwnd)
			rendererStatus = "GDI (opaque fallback)"
			rendererDetail = "DWM-Redirection-Alpha konnte auf Windows 11 24H2+ nicht aktiviert werden"
			logStartup("DWMWA_REDIRECTIONBITMAP_ALPHA rejected; falling back to opaque renderer")
			return false
		}
		rendererStatus = materialName + " + Alpha-DIB"
		rendererDetail = "32-Bit-DIB-Backbuffer; GDI-Ausgabe wird vor DWM-Präsentation alpha-korrigiert"
	}
	logStartup("Windows material enabled: %s (backdrop=%d)", materialName, backdrop)
	return true
}

func startWindowMaterialAsync(hwnd HWND) {
	// Despite the historical name this is deliberately synchronous now. All
	// HWND/DWM mutations stay on LogiMate's locked UI thread, eliminating races
	// between theme toggles, shutdown and a late background material worker.
	if safeUI || !getUISettings().Acrylic || hwnd == 0 || materialStarted {
		return
	}
	materialStarted = true
	glassActive = applyWindowMaterial(hwnd)
	logStartup("Window material activation completed on UI thread; active=%v", glassActive)
	invalidate(hwnd)
}

func stopHardwareForLifecycle(reason string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: panic during hardware shutdown: %v", reason, r)
			logStartup("%v", err)
		}
	}()
	stateMu.RLock()
	s := appState
	stateMu.RUnlock()
	if system.NativeOutputSnapshot().Active || system.NativeFFBSnapshot().Active || system.NativeOutputLeaseSnapshot().Active {
		if e := system.NativeOutputEmergencyStop(s); e != nil {
			return fmt.Errorf("%s: %w", reason, e)
		}
	}
	if e := system.NativeOutputRelease(); e != nil {
		return fmt.Errorf("%s: Output-Lease konnte nicht sicher freigegeben werden: %w", reason, e)
	}
	return nil
}

type powerEventKind int

const (
	powerEventNone powerEventKind = iota
	powerEventSuspend
	powerEventResume
)

func classifyPowerBroadcast(wParam uintptr) powerEventKind {
	switch wParam {
	case PBT_APMSUSPEND, PBT_APMSTANDBY:
		return powerEventSuspend
	case PBT_APMRESUMECRITICAL, PBT_APMRESUMESUSPEND, PBT_APMRESUMEAUTOMATIC:
		return powerEventResume
	default:
		return powerEventNone
	}
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			logStartup("wndProc panic msg=0x%X: %v", msg, r)
			ret = 0
		}
	}()
	switch msg {
	case WM_CREATE:
		mainWnd = HWND(hwnd)
		ensureControls(HWND(hwnd))
		return 0
	case WM_DESTROY:
		system.StopTelemetry()
		if err := stopHardwareForLifecycle("WM_DESTROY"); err != nil {
			logStartup("hardware shutdown warning: %v", err)
		}
		pKillTimer.Call(hwnd, timerJoy)
		pKillTimer.Call(hwnd, timerRefresh)
		pKillTimer.Call(hwnd, timerAnim)
		pKillTimer.Call(hwnd, timerMaterial)
		pKillTimer.Call(hwnd, timerStartupStable)
		pKillTimer.Call(hwnd, timerSmoke)
		pKillTimer.Call(hwnd, timerDeviceRefresh)
		pPostQuitMessage.Call(0)
		return 0
	case WM_QUERYENDSESSION:
		if err := stopHardwareForLifecycle("WM_QUERYENDSESSION"); err != nil {
			logStartup("session-end safety warning: %v", err)
		}
		return 1
	case WM_ENDSESSION:
		if wParam != 0 {
			if err := stopHardwareForLifecycle("WM_ENDSESSION"); err != nil {
				logStartup("session-end safety warning: %v", err)
			}
		}
		return 0
	case WM_POWERBROADCAST:
		switch classifyPowerBroadcast(wParam) {
		case powerEventSuspend:
			if err := stopHardwareForLifecycle("power-suspend"); err != nil {
				logStartup("power safety warning: %v", err)
				recordDiagnosticEvent("error", "LIFECYCLE-POWER-STOP", "Lifecycle", "Hardware konnte vor Standby nicht vollständig neutralisiert werden", err.Error())
			}
			system.ClosePreferredInput()
			system.InvalidateSystemCaches()
			recordDiagnosticEvent("info", "LIFECYCLE-SUSPEND", "Lifecycle", "Windows wechselt in Standby", "Output neutralisiert und HID-Sitzung geschlossen.")
		case powerEventResume:
			system.ClosePreferredInput()
			system.InvalidateSystemCaches()
			system.InvalidateEphemeralWheelConfirmations()
			deviceRefreshPending = true
			pKillTimer.Call(hwnd, timerDeviceRefresh)
			pSetTimer.Call(hwnd, timerDeviceRefresh, 500, 0)
			recordDiagnosticEvent("info", "LIFECYCLE-RESUME", "Lifecycle", "Windows wurde fortgesetzt", "Geräte werden neu gebunden; Force Feedback bleibt bis zur nächsten expliziten Freigabe aus.")
		}
		return 1
	case WM_ERASEBKGND:
		return 1
	case WM_KEYDOWN:
		if onKeyDown(uint32(wParam)) {
			return 0
		}
	case WM_COMMAND:
		if handleAccessibilityCommand(int(wParam & 0xffff)) {
			return 0
		}
	case WM_PAINT:
		paint(HWND(hwnd))
		return 0
	case WM_MOUSEMOVE:
		onMouseMove(signedLoWord(lParam), signedHiWord(lParam))
		return 0
	case WM_MOUSELEAVE:
		mouseTracked = false
		hoveredNav = -1
		hoveredSetting = -1
		hoveredTheme = -1
		hoveredAction = -1
		wheelAdvancedHovered = false
		memoryIntegrityHovered = false
		wheelSubtabHover = -1
		d62ViewHover = -1
		d6SliderHover = -1
		d6HoverKind = ""
		d6HoverIndex = -1
		if getUISettings().SidebarAutoExpand {
			sidebarTarget = sidebarExpanded
		} else {
			sidebarTarget = sidebarCollapsed
		}
		startAnimation()
		invalidate(HWND(hwnd))
		return 0
	case WM_LBUTTONDOWN:
		onMouseDown(signedLoWord(lParam), signedHiWord(lParam))
		return 0
	case WM_LBUTTONUP:
		onMouseClick(signedLoWord(lParam), signedHiWord(lParam))
		return 0
	case WM_CANCELMODE, WM_CAPTURECHANGED:
		if pressedAction >= 0 {
			pressedAction = -1
			invalidate(HWND(hwnd))
		}
		if d6SliderDrag >= 0 {
			d6SliderDrag = -1
			invalidate(HWND(hwnd))
		}
		if d6PressedKind != "" {
			d6PressedKind, d6PressedIndex = "", -1
			invalidate(HWND(hwnd))
		}
		return 0
	case WM_MOUSEWHEEL:
		if currentPage == pageAbout && projectPanelOpen {
			return 0
		}
		delta := signedHiWord(wParam)
		if delta != 0 {
			contentScroll -= (delta / 120) * 56
			if contentScroll < 0 {
				contentScroll = 0
			}
			if contentScroll > contentScrollMax {
				contentScroll = contentScrollMax
			}
			invalidate(HWND(hwnd))
		}
		return 0
	case WM_TIMER:
		switch wParam {
		case timerJoy:
			if currentPage == pageWheel {
				updateWheelLive()
			}
		case timerRefresh:
			if !busy && getUISettings().AutoRefresh {
				refreshFastAsync()
			}
		case timerAnim:
			animationTick()
		case timerMaterial:
			pKillTimer.Call(hwnd, timerMaterial)
			startWindowMaterialAsync(HWND(hwnd))
		case timerStartupStable:
			pKillTimer.Call(hwnd, timerStartupStable)
			if startupMarker != "" {
				_ = os.Remove(startupMarker)
			}
			startupStableReached = true
			logStartup("Startup marked stable")
			maybeRunStartupActions()
		case timerSmoke:
			pKillTimer.Call(hwnd, timerSmoke)
			pKillTimer.Call(hwnd, timerDeviceRefresh)
			logStartup("Smoke test completed; closing window")
			postMessage(HWND(hwnd), WM_CLOSE, 0, 0)
		case timerFeedback:
			pKillTimer.Call(hwnd, timerFeedback)
			actionFeedback = ""
			invalidate(HWND(hwnd))
		case timerDeviceRefresh:
			pKillTimer.Call(hwnd, timerDeviceRefresh)
			if deviceRefreshPending {
				if busy {
					// A state scan is already running. Keep the event pending and
					// retry shortly so a USB transition can never be lost.
					pSetTimer.Call(hwnd, timerDeviceRefresh, 250, 0)
				} else {
					deviceRefreshPending = false
					refreshAsync()
				}
			}
		}
		return 0
	case WM_SIZE:
		layout(HWND(hwnd))
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			minW, minH := responsiveMinimumTrackSize(currentDPI, screenMetric(SM_CXSCREEN), screenMetric(SM_CYSCREEN))
			mmi.PtMinTrackSize = POINT{X: minW, Y: minH}
		}
		return 0
	case WM_DPICHANGED:
		currentDPI = uint32(wParam >> 16)
		if currentDPI == 0 {
			currentDPI = 96
		}
		if lParam != 0 {
			r := (*RECT)(unsafe.Pointer(lParam))
			pMoveWindow.Call(hwnd, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 1)
		}
		rebuildFontsForDPI()
		releaseBackBuffer()
		invalidate(HWND(hwnd))
		return 0
	case WM_DEVICECHANGE:
		// Any PnP topology change invalidates assumptions behind a force-output
		// lease. Fail safe before rebuilding device identity, even when Windows
		// later reports that the changed device was unrelated.
		if system.NativeOutputSnapshot().Active || system.NativeFFBSnapshot().Active || system.NativeOutputLeaseSnapshot().Active {
			if err := stopHardwareForLifecycle("WM_DEVICECHANGE"); err != nil {
				logStartup("device-change emergency stop failed; lease remains fail-closed: %v", err)
			}
		}
		system.InvalidateInputCaches()
		system.InvalidateEphemeralWheelConfirmations()
		// USB re-enumeration often emits several broadcasts in a burst. Debounce
		// them on the UI timer rather than spawning background timers; if a state
		// scan is busy, timerDeviceRefresh keeps the refresh pending.
		deviceRefreshPending = true
		pKillTimer.Call(hwnd, timerDeviceRefresh)
		pSetTimer.Call(hwnd, timerDeviceRefresh, 350, 0)
		return 0
	case WM_SETTINGCHANGE:
		if normalizeThemeMode(getUISettings().ThemeMode) == "system" {
			rebuildThemeResources()
		} else {
			invalidate(HWND(hwnd))
		}
		return 0
	case WM_DWMCOMPOSITIONCHANGED:
		if !safeUI && getUISettings().Acrylic {
			materialStarted = false
			startWindowMaterialAsync(HWND(hwnd))
		}
		return 0
	case msgStateReady:
		busy = false
		if need, detail := system.RuntimeOutputRecoveryNeeded(appStateSnapshot().DataDir); need {
			stateMu.RLock()
			rs := appState
			stateMu.RUnlock()
			recovered, recoveredEffect, recoveryErr := system.RecoverRuntimeOutputIfNeeded(rs)
			if recovered && !runtimeRecoveryChecked {
				runtimeRecoveryChecked = true
				queueNotice("Native Output Recovery", "Ein nicht sauber beendeter Motor-Output wurde erkannt ("+detail+"). LogiMate hat den verifizierten Zielzustand neutralisiert ("+recoveredEffect+").", MB_OK|MB_ICONWARNING)
			} else if recoveryErr != nil {
				// Keep the marker and retry on the next fresh state/device event. Do not
				// claim recovery until exact target correlation and writes succeeded.
				logStartup("pending native-output recovery retained: %v", recoveryErr)
			}
		} else {
			runtimeRecoveryChecked = true
		}

		if setupWnd != 0 {
			stateMu.RLock()
			setupState = appState
			stateMu.RUnlock()
			invalidate(setupWnd)
		}
		updatePage()
		if manualRefreshPending {
			manualRefreshPending = false
			setActionFeedback("Status aktualisiert.")
		}
		maybeRunStartupActions()
		invalidate(HWND(hwnd))
		return 0
	case msgOperationText:
		invalidate(HWND(hwnd))
		return 0
	case msgRefreshRequest:
		refreshAsync()
		return 0
	case msgNotice:
		showNextNotice()
		return 0
	case msgUpdateFound:
		handleUpdateFound()
		return 0
	case msgUpdateDownloaded:
		handleUpdateDownloaded()
		return 0
	case msgSetupMigrationDone:
		handleSetupMigrationResult()
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func maybeRunStartupActions() {
	if smokeTest || !startupStableReached || startupActionsDone {
		return
	}
	stateMu.RLock()
	ready := appState.DataDir != ""
	stateMu.RUnlock()
	if !ready {
		return
	}
	startupActionsDone = true
	onStartupStable()
}

func createControls(hwnd HWND) {
	pSetTimer.Call(uintptr(hwnd), timerJoy, 250, 0)
	pSetTimer.Call(uintptr(hwnd), timerRefresh, 7000, 0)
	ensureAccessibilityOverlays(hwnd)
	updatePage()
}

func layout(hwnd HWND) {
	// Action buttons are painted directly into the same double-buffered Acrylic
	// surface as the rest of the UI. Narrow windows force the compact sidebar so
	// content never ends up behind an oversized navigation rail.
	if hwnd != 0 {
		var rc RECT
		pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
		if rc.Right-rc.Left < 980 {
			sidebarTarget = sidebarCollapsed
			if !animRunning || !animationsAllowed() {
				sidebarWidth = sidebarCollapsed
			}
		}
		clampContentScrollForViewport(rc)
		invalidate(hwnd)
	}
}

func pageOffset() int32 {
	if pageAnim >= 1 {
		return 0
	}
	return int32((1 - pageAnim) * 18)
}

func startAnimation() {
	if mainWnd == 0 {
		return
	}
	if !animationsAllowed() {
		pKillTimer.Call(uintptr(mainWnd), timerAnim)
		animRunning = false
		sidebarWidth = sidebarTarget
		pageAnim = 1
		layout(mainWnd)
		invalidate(mainWnd)
		return
	}
	if animRunning {
		return
	}
	animRunning = true
	pSetTimer.Call(uintptr(mainWnd), timerAnim, 16, 0)
}

func animationTick() {
	done := true
	if sidebarWidth != sidebarTarget {
		diff := sidebarTarget - sidebarWidth
		step := diff / 4
		if step == 0 {
			if diff > 0 {
				step = 1
			} else {
				step = -1
			}
		}
		sidebarWidth += step
		if abs32(sidebarTarget-sidebarWidth) <= 2 {
			sidebarWidth = sidebarTarget
		} else {
			done = false
		}
	}

	if pageAnim < 1 {
		pageAnim += (1 - pageAnim) * 0.34
		if pageAnim >= 0.985 {
			pageAnim = 1
		} else {
			done = false
		}
	}

	layout(mainWnd)
	invalidate(mainWnd)
	if done {
		pKillTimer.Call(uintptr(mainWnd), timerAnim)
		animRunning = false
	}
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func onMouseMove(x, y int32) {
	if !mouseTracked {
		trackMouseLeave(mainWnd)
		mouseTracked = true
	}

	prefs := getUISettings()
	// Profile-hub navigation is intentionally stable: by default the full
	// sidebar remains visible instead of changing width underneath the user.
	// The existing preference now acts as a persistent full/compact choice.
	var client RECT
	pGetClientRect.Call(uintptr(mainWnd), uintptr(unsafe.Pointer(&client)))
	if client.Right-client.Left < 980 {
		sidebarTarget = sidebarCollapsed
	} else if prefs.SidebarAutoExpand {
		sidebarTarget = sidebarExpanded
	} else {
		sidebarTarget = sidebarCollapsed
	}

	oldNav := hoveredNav
	oldSetting := hoveredSetting
	oldTheme := hoveredTheme
	oldAction := hoveredAction
	oldAdvanced := wheelAdvancedHovered
	oldMemoryIntegrity := memoryIntegrityHovered
	d6Changed := d6OnMouseMove(x, y)
	wheelReworkChanged := wheelReworkOnMouseMove(x, y)
	diagnosticsChanged := diagnosticsHoverChanged(x, y)
	hoveredNav = navHitTest(x, y)
	hoveredSetting = -1
	hoveredTheme = -1
	if currentPage == pageSettings {
		hoveredTheme = themeHitTest(x, y)
		hoveredSetting = settingsHitTest(x, y)
	}
	hoveredAction = actionHitTest(x, y)
	wheelAdvancedHovered = wheelAdvancedHitTest(x, y)
	memoryIntegrityHovered = memoryIntegrityHitTest(x, y)
	setCursorHand(hoveredNav >= 0 || hoveredSetting >= 0 || hoveredTheme >= 0 || hoveredAction >= 0 || wheelAdvancedHovered || memoryIntegrityHovered || wheelSubtabHover >= 0 || d6HoverKind != "" || wheelReworkHoverKind != "" || diagnosticsHitTest(x, y) != 0 || aboutHubHitTest(x, y) != 0)
	if oldNav != hoveredNav || oldSetting != hoveredSetting || oldTheme != hoveredTheme || oldAction != hoveredAction || oldAdvanced != wheelAdvancedHovered || oldMemoryIntegrity != memoryIntegrityHovered || d6Changed || wheelReworkChanged || diagnosticsChanged || sidebarWidth != sidebarTarget {
		startAnimation()
		invalidate(mainWnd)
	}
}

func onMouseDown(x, y int32) {
	if d6OnMouseDown(x, y) {
		return
	}
	idx := actionHitTest(x, y)
	if idx < 0 {
		return
	}
	pressedAction = idx
	setCapture(mainWnd)
	invalidate(mainWnd)
}

func onMouseClick(x, y int32) {
	stateMu.RLock()
	d6State := appState
	stateMu.RUnlock()
	if d6OnMouseUp(x, y, d6State) {
		return
	}
	if pressedAction >= 0 {
		pressed := pressedAction
		setKeyboardFocusID(focusActionBase + pressed)
		pressedAction = -1
		releaseCapture()
		hit := actionHitTest(x, y)
		invalidate(mainWnd)
		if hit == pressed {
			action(currentPage, pressed)
		}
		return
	}
	if d6HandleClick(x, y, d6State) {
		return
	}
	if wheelReworkHandleClick(x, y, d6State) {
		return
	}
	if diagnosticsHandleClick(x, y, d6State) {
		return
	}
	if memoryIntegrityHitTest(x, y) {
		setKeyboardFocusID(focusMemoryIntegrity)
		activateMemoryIntegritySettings()
		return
	}
	if currentPage == pageWheel && wheelAdvancedHitTest(x, y) {
		setKeyboardFocusID(focusWheelAdvanced)
		wheelAdvancedView = !wheelAdvancedView
		contentScroll = 0
		setActionFeedback(map[bool]string{true: "Erweiterte Lenkradansicht aktiv.", false: "Standardansicht aktiv."}[wheelAdvancedView])
		invalidate(mainWnd)
		return
	}
	if currentPage == pageAbout {
		if code := aboutHubHitTest(x, y); code != 0 && handleAboutHubClick(code) {
			return
		}
	}
	if currentPage == pageSettings {
		if idx := themeHitTest(x, y); idx >= 0 && idx < len(themeChoices) {
			setKeyboardFocusID(focusThemeBase + idx)
			setThemeMode(themeChoices[idx].id)
			return
		}
		if idx := settingsHitTest(x, y); idx >= 0 {
			setKeyboardFocusID(focusSettingBase + idx)
			toggleSetting(idx)
			return
		}
	}
	idx := navHitTest(x, y)
	if idx >= 0 && idx < len(navItems) {
		setKeyboardFocusID(idx)
		setPage(navItems[idx].page)
	}
}

func navItemRect(index int, rc RECT) RECT {
	if index < 0 || index >= len(navItems) {
		return RECT{}
	}
	// Settings is visually separated at the bottom, matching the profile-hub
	// reference. All other destinations keep a stable top navigation stack.
	if navItems[index].page == pageSettings {
		y := rc.Bottom - 92
		return RECT{10, y, sidebarWidth - 10, y + navHeight}
	}
	visualIndex := index
	// Settings lives last in navItems, so indices before it map 1:1.
	y := navTop + int32(visualIndex)*(navHeight+navGap)
	return RECT{10, y, sidebarWidth - 10, y + navHeight}
}

func navHitTest(x, y int32) int {
	if x < 8 || x > sidebarWidth-8 {
		return -1
	}
	var rc RECT
	if mainWnd != 0 {
		pGetClientRect.Call(uintptr(mainWnd), uintptr(unsafe.Pointer(&rc)))
	}
	for i := range navItems {
		r := navItemRect(i, rc)
		if y >= r.Top && y <= r.Bottom {
			return i
		}
	}
	return -1
}

func setPage(p int) {
	if p == currentPage {
		return
	}
	currentPage = p
	if p != pageAbout {
		projectPanelOpen = false
	}
	hoveredSetting = -1
	hoveredTheme = -1
	wheelAdvancedHovered = false
	wheelAdvancedToggleRect = RECT{}
	wheelSubtabHover = -1
	d62ViewHover = -1
	d6SliderHover = -1
	d6HoverKind = ""
	d6HoverIndex = -1
	contentScroll = 0
	pageAnim = 0
	updatePage()
	startAnimation()
	invalidate(mainWnd)
}

func setAction(i int, label string, visible, primary, warning bool) {
	if i < 0 || i >= len(actionButtons) {
		return
	}
	actionButtons[i] = actionButton{
		label: label, icon: actionIcon(label), visible: visible, enabled: visible, primary: primary, warning: warning,
	}
	if !visible {
		if hoveredAction == i {
			hoveredAction = -1
		}
		if pressedAction == i {
			pressedAction = -1
		}
	}
}

func setActionEnabled(i int, enabled bool) {
	if i < 0 || i >= len(actionButtons) {
		return
	}
	actionButtons[i].enabled = enabled && actionButtons[i].visible
	if !actionButtons[i].enabled {
		if hoveredAction == i {
			hoveredAction = -1
		}
		if pressedAction == i {
			pressedAction = -1
		}
	}
}

func actionIcon(label string) string {
	switch label {
	case "Modern / Generic HID", "Generic HID", "Modern aktiv":
		return "↗"
	case "Logitech Legacy", "Legacy wiederherstellen", "Original Logitech", "Legacy aktiv":
		return "↶"
	case "Treiber sichern", "Diagnose-ZIP":
		return "↓"
	case "Aktualisieren", "Status aktualisieren", "Updates prüfen":
		return "↻"
	case "Profilordner", "Diagnoseordner", "Datenordner":
		return "⌂"
	case "Modell wählen", "Modell bestätigen", "Als G25 merken", "Als G27 merken", "Lenkrad wechseln", "Gerät auswählen", "Geräte verwalten":
		return "✓"
	case "Bericht kopieren":
		return "⧉"
	case "Einrichtung prüfen":
		return "✓"
	case "Hardware testen", "Windows-Test", "Pedale lernen", "Native Engine Status":
		return "◉"
	case "Treiber & Modus":
		return "⚙"
	case "Diagnose":
		return "?"
	case "Was ist neu?":
		return "i"
	case "Zurücksetzen", "Einstellungen zurücksetzen", "Erkennung zurücksetzen", "Erkennung neu starten":
		return "↺"
	case "LogiMate auf GitHub", "Indicana Tools":
		return "↗"
	case "Indicana Projekte":
		return "◆"
	case "PayPal kopieren":
		return "⧉"
	case "♥ Sharing is caring":
		return "♥"
	default:
		return "•"
	}
}

func withDiagnosticMeta(s system.State) system.State {
	s.AppVersion = displayVersion(Version) + " · " + buildLabel()
	s.Theme = normalizeThemeMode(getUISettings().ThemeMode) + " (effective: " + effectiveThemeMode(getUISettings().ThemeMode) + ")"
	s.DPI = currentDPI
	s.MonitorCount = monitorCount()
	s.HighContrast = highContrastEnabled()
	s.ReducedMotion = !clientAnimationsEnabled()
	s.WindowsBuild = windowsBuildNumber()
	s.Renderer = rendererStatus
	s.RendererDetail = rendererDetail
	s.MaterialActive = glassActive
	return s
}

func updatePage() {
	for i := 0; i < 4; i++ {
		setAction(i, "", false, false, false)
	}
	stateMu.RLock()
	s := appState
	stateMu.RUnlock()

	var text string
	switch currentPage {
	case pageOverview:
		// The landing page is task-oriented: the primary button always reflects
		// the safest useful next step instead of forcing users to decode state.
		setAction(0, overviewPrimaryAction(s), true, true, false)
		setAction(1, "Einrichtung prüfen", true, false, false)
		setAction(2, "Status aktualisieren", true, false, false)
		setAction(3, "Diagnose", true, false, false)
		text = dashboardText(s)
	case pageWheel:
		needsConfirmation := system.SelectedWheelNeedsModelConfirmation(s)
		_, selectedPresent := system.SelectedWheel(s)
		needsDeviceAttention := s.DeviceDetectionError != "" || !selectedPresent
		setAction(0, "Windows-Test", true, !needsDeviceAttention, false)
		setAction(1, "Geräte verwalten", true, needsDeviceAttention, false)
		setAction(2, "Diagnose", true, false, false)
		if len(s.Wheels) == 0 {
			setAction(3, "Diagnose", true, false, false)
		} else if needsConfirmation {
			setAction(3, "Modell bestätigen", true, true, false)
		} else if !system.HasActionableSelectedWheel(s) {
			setAction(3, "Diagnose", true, false, false)
		} else if system.HasActionableSelectedWheel(s) && strings.Contains(s.ActiveMode, "Generic") {
			setAction(3, "Native Engine Status", true, false, false)
		} else {
			setAction(3, "Treiber & Modus", true, false, false)
		}
		j := system.ReadPreferredWheelInput(s)
		setLiveJoy(j)
		text = wheelText(s, j)
	case pageSystem:
		// Backup is the primary system action; mode changes remain explicit and
		// separated from the safe first-run path. Current modes stay visible but
		// disabled so users can immediately see what is already active.
		setAction(0, "Treiber sichern", true, true, false)
		setActionEnabled(0, s.LegacyDriverError == "" && len(s.LegacyDrivers) > 0)
		modernActive := strings.Contains(s.ActiveMode, "Generic HID")
		legacyActive := s.ActiveMode == "Logitech Legacy"
		modernLabel := "Modern / Generic HID"
		legacyLabel := "Original Logitech"
		if modernActive {
			modernLabel = "Modern aktiv"
		}
		if legacyActive {
			legacyLabel = "Legacy aktiv"
		}
		setAction(1, modernLabel, true, false, false)
		setAction(2, legacyLabel, true, false, true)
		actionable := system.CanChangeSelectedWheelMode(s)
		setActionEnabled(1, actionable && !modernActive)
		setActionEnabled(2, actionable && !legacyActive)
		if system.HasActionableSelectedWheel(s) && modernActive {
			setAction(3, "Native Engine Status", true, false, false)
		}
		// Primary emphasis follows the safe workflow: backup first when needed,
		// otherwise the useful forward action instead of a permanently-primary
		// disabled backup button.
		needBackup := len(s.LegacyDrivers) > 0 && s.BackupCount == 0 && s.LegacyDriverError == ""
		actionButtons[0].primary = needBackup && actionButtons[0].enabled
		if !needBackup && actionButtons[1].enabled {
			actionButtons[1].primary = true
		} else if !needBackup && actionButtons[3].visible && actionButtons[3].enabled {
			actionButtons[3].primary = false
		}
		text = systemText(s)
	case pageDiagnostics:
		setAction(0, "Diagnose-ZIP", true, true, false)
		setAction(1, "Bericht kopieren", true, false, false)
		setAction(2, "Diagnoseordner", true, false, false)
		setAction(3, "Status aktualisieren", true, false, false)
		text = diagnosticReportWithAccessibility(s)
	case pageSettings:
		setAction(0, "Updates prüfen", true, true, false)
		setAction(1, "Was ist neu?", true, false, false)
		setAction(2, "Datenordner", true, false, false)
		setAction(3, "Einstellungen zurücksetzen", true, false, true)
		text = ""
	case pageAbout:
		setAction(0, "LogiMate auf GitHub", true, true, false)
		setAction(1, "Indicana Projekte", true, false, false)
		setAction(2, "PayPal kopieren", true, false, false)
		text = aboutPageText()
	}
	setBody(text)
	layout(mainWnd)
}

func diagnosticReportWithAccessibility(s system.State) string {
	return system.BuildDiagnosticReport(withDiagnosticMeta(s)) + "\r\n\r\nAccessibility / UI Automation:\r\n" + accessibilityRuntimeSummary()
}

func setActionFeedback(text string) {
	actionFeedback = strings.TrimSpace(text)
	if mainWnd == 0 {
		return
	}
	pKillTimer.Call(uintptr(mainWnd), timerFeedback)
	if actionFeedback != "" {
		pSetTimer.Call(uintptr(mainWnd), timerFeedback, 2800, 0)
	}
	invalidate(mainWnd)
}

func materialClearBrush() HBRUSH { return glassBrush }

func setBody(s string) {
	bodyMu.Lock()
	pageBody = s
	bodyMu.Unlock()
}

func setBodyAsync(s string) {
	setBody(s)
	postMessage(mainWnd, msgOperationText, 0, 0)
}

func getBody() string {
	bodyMu.RLock()
	defer bodyMu.RUnlock()
	return pageBody
}

func requestRefresh() {
	if mainWnd != 0 {
		postMessage(mainWnd, msgRefreshRequest, 0, 0)
	}
}

func queueNotice(text, title string, flags uint32) {
	recordDiagnosticNotice(text, title, flags)
	noticeMu.Lock()
	notices = append(notices, uiNotice{text: text, title: title, flags: flags})
	noticeMu.Unlock()
	if mainWnd != 0 {
		postMessage(mainWnd, msgNotice, 0, 0)
	}
}

func showNextNotice() {
	noticeMu.Lock()
	if len(notices) == 0 {
		noticeMu.Unlock()
		return
	}
	n := notices[0]
	notices = notices[1:]
	noticeMu.Unlock()
	messageBox(mainWnd, n.text, n.title, n.flags)
}

func refreshAsync() {
	if busy {
		return
	}
	busy = true
	invalidate(mainWnd)
	go func() {
		s := system.CollectState()
		system.UpdateGameSession(s, getUISettings().NativeWheelOutput)
		stateMu.Lock()
		appState = s
		stateMu.Unlock()
		postMessage(mainWnd, msgStateReady, 0, 0)
	}()
}

func refreshFastAsync() {
	if busy {
		return
	}
	busy = true
	stateMu.RLock()
	previous := appState
	stateMu.RUnlock()
	go func() {
		s := system.CollectStateFast(previous)
		system.UpdateGameSession(s, getUISettings().NativeWheelOutput)
		stateMu.Lock()
		appState = s
		stateMu.Unlock()
		postMessage(mainWnd, msgStateReady, 0, 0)
	}()
}

func updateWheelLive() {
	stateMu.RLock()
	s := appState
	stateMu.RUnlock()
	j := system.ReadPreferredWheelInput(s)
	setLiveJoy(j)
	setBody(wheelText(s, j))
	if mainWnd == 0 {
		return
	}
	// D6.2: live telemetry no longer invalidates the complete top-level window
	// every 250 ms. On the FFB Live & Test view only the monitor card changes.
	// The standard wheel page still refreshes its content surface, but leaves
	// sidebar, title/status band and footer untouched.
	if wheelSubtab == wheelSubtabFFB {
		if d62ActiveView == d62ViewLiveTest && rectUsable(d62LiveMonitorRect) {
			invalidateArea(mainWnd, d62LiveMonitorRect)
		}
		return
	}
	if wheelSubtab == wheelSubtabCalibration && wheelCalibrationWizard.Active {
		// Inline calibration needs current values, but only its compact live
		// rows repaint at 4 Hz. The rest of the page remains static.
		if rectUsable(wheelCalibrationWizardLiveRect) {
			invalidateArea(mainWnd, wheelCalibrationWizardLiveRect)
		}
		return
	}
	// Build 007 UX consistency: calibration, profile and device views are
	// configuration surfaces, not 4 Hz telemetry canvases. Avoid repainting
	// them every 250 ms; normal state refreshes and user actions invalidate
	// these pages when their data can actually change.
	if wheelSubtab != wheelSubtabLive {
		return
	}
	var rc RECT
	pGetClientRect.Call(uintptr(mainWnd), uintptr(unsafe.Pointer(&rc)))
	content := contentRectFor(rc)
	if rectUsable(content) {
		invalidateArea(mainWnd, content)
	} else {
		invalidate(mainWnd)
	}
}

func paint(hwnd HWND) {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	var rc RECT
	pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	w, h := rc.Right-rc.Left, rc.Bottom-rc.Top
	if w <= 0 || h <= 0 {
		return
	}

	paintRect := ps.RcPaint
	if paintRect.Right <= paintRect.Left || paintRect.Bottom <= paintRect.Top {
		paintRect = rc
	}

	drawDC := hdc
	buffered := ensureBackBuffer(hdc, w, h)
	if glassActive && !buffered {
		// Transparent GDI without a 32-bit alpha-owned backbuffer is unreliable and
		// is the exact class of failure that can produce an empty Acrylic window.
		// Prefer a visible opaque UI over a broken material surface.
		disableWindowMaterial(hwnd)
		rendererStatus = "GDI (opaque recovery)"
		rendererDetail = "32-Bit-DIB konnte nicht erstellt werden; Acrylic wurde für diesen Lauf sicher deaktiviert"
		logStartup("Acrylic disabled at paint time because alpha DIB allocation failed (%dx%d)", w, h)
	}
	if buffered {
		drawDC = backDC
	}
	clipSaved, _, _ := pSaveDC.Call(drawDC)
	pIntersectClipRect.Call(drawDC, uintptr(paintRect.Left), uintptr(paintRect.Top), uintptr(paintRect.Right), uintptr(paintRect.Bottom))

	clearBrush := bgBrush
	if glassActive {
		clearBrush = glassBrush
	}
	pFillRect.Call(drawDC, uintptr(unsafe.Pointer(&paintRect)), uintptr(clearBrush))
	pSetBkMode.Call(drawDC, TRANSPARENT)

	paintSidebar(drawDC, rc)
	paintMain(drawDC, rc)
	paintHoverTip(drawDC, rc)
	if clipSaved != 0 {
		pRestoreDC.Call(drawDC, clipSaved)
	}

	if buffered {
		if glassActive {
			repairBackBufferAlpha(w, h)
		}
		pBitBlt.Call(hdc, uintptr(paintRect.Left), uintptr(paintRect.Top), uintptr(paintRect.Right-paintRect.Left), uintptr(paintRect.Bottom-paintRect.Top), backDC, uintptr(paintRect.Left), uintptr(paintRect.Top), SRCCOPY)
	}
	syncAccessibilityOverlays()
}

func ensureBackBuffer(referenceDC uintptr, w, h int32) bool {
	if w <= 0 || h <= 0 || w > 16384 || h > 16384 || int64(w)*int64(h) > 64*1024*1024 {
		return false
	}
	if backDC != 0 && backBitmap != 0 && backBits != 0 && backW == w && backH == h {
		return true
	}
	releaseBackBuffer()
	dc, _, _ := pCreateCompatibleDC.Call(referenceDC)
	if dc == 0 {
		return false
	}
	bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth:       w,
		BiHeight:      -h, // top-down DIB so memory order matches the window
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: BI_RGB,
		BiSizeImage:   uint32(int64(w) * int64(h) * 4),
	}}
	var bits uintptr
	bmp, _, _ := pCreateDIBSection.Call(referenceDC, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		if bmp != 0 {
			deleteObject(bmp)
		}
		pDeleteDC.Call(dc)
		return false
	}
	old, _, _ := pSelectObject.Call(dc, bmp)
	backDC, backBitmap, backOldBitmap, backBits = dc, bmp, old, bits
	backW, backH = w, h
	return true
}

func repairAlphaPixels(pixels []byte) {
	for i := 0; i+3 < len(pixels); i += 4 {
		// DIB memory is BGRA. Pure black is the intentionally transparent glass
		// clear; every painted LogiMate pixel must be visible to DWM. Preserve a
		// non-zero alpha produced by icon rendering, otherwise make it opaque.
		if pixels[i] == 0 && pixels[i+1] == 0 && pixels[i+2] == 0 {
			pixels[i+3] = 0
		} else if pixels[i+3] == 0 {
			pixels[i+3] = 0xFF
		}
	}
}

func repairBackBufferAlpha(w, h int32) {
	if backBits == 0 || w <= 0 || h <= 0 {
		return
	}
	count64 := int64(w) * int64(h) * 4
	if count64 <= 0 || count64 > 256*1024*1024 {
		return
	}
	repairAlphaPixels(unsafe.Slice((*byte)(unsafe.Pointer(backBits)), int(count64)))
}

func releaseBackBuffer() {
	if backDC != 0 {
		if backOldBitmap != 0 {
			pSelectObject.Call(backDC, backOldBitmap)
		}
		if backBitmap != 0 {
			deleteObject(backBitmap)
		}
		pDeleteDC.Call(backDC)
	}
	backDC, backBitmap, backOldBitmap, backBits = 0, 0, 0, 0
	backW, backH = 0, 0
}

func paintSidebar(hdc uintptr, rc RECT) {
	side := RECT{0, 0, sidebarWidth, rc.Bottom}
	// When the documented Windows backdrop is active, let the material show
	// through the navigation rail instead of painting an opaque slab over it.
	// Selected/hovered navigation cards remain solid enough for readability.
	// Safe UI, High Contrast and non-material Windows keep the normal opaque
	// sidebar so accessibility and recovery never depend on composition.
	transparentRail := glassActive && getUISettings().Acrylic && !highContrastEnabled()
	if !transparentRail {
		pFillRect.Call(hdc, uintptr(unsafe.Pointer(&side)), uintptr(sidebarBrush))
	} else if sidebarWidth > 1 {
		line(hdc, sidebarWidth-1, 0, sidebarWidth-1, rc.Bottom, blendColor(colBorder, themeBackground, 28), 1)
	}

	// Embedded LogiMate mark. The same multi-resolution asset is used for
	// the taskbar/window icon and the in-app brand so the product stays
	// visually consistent at every scale.
	brand := RECT{15, 20, 61, 66}
	drawBrandIcon(hdc, brand)
	var old uintptr

	if sidebarWidth >= 152 {
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBrand))
		r := RECT{73, 19, sidebarWidth - 12, 45}
		drawText(hdc, "LogiMate", &r, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		r = RECT{73, 43, sidebarWidth - 12, 66}
		drawText(hdc, "G25 · G27 · DFGT", &r, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}

	for i, n := range navItems {
		r := navItemRect(i, rc)
		y := r.Top
		selected := n.page == currentPage
		hovered := i == hoveredNav
		if selected {
			fillRoundRect(hdc, r, 12, colSelected)
			mark := RECT{10, y + 12, 13, y + navHeight - 12}
			fillRoundRect(hdc, mark, 3, colAccent)
		} else if hovered {
			fillRoundRect(hdc, r, 12, colHover)
		}
		if keyboardFocus == i {
			fill := themeSidebar
			if transparentRail {
				fill = colPanel2
			}
			if selected {
				fill = colSelected
			} else if hovered {
				fill = colHover
			}
			drawRoundRect(hdc, r, 12, fill, colFocusRing)
		}

		iconR := RECT{15, y, 61, y + navHeight}
		pSetTextColor.Call(hdc, map[bool]uintptr{true: colAccent, false: colMuted}[selected])
		if hovered && !selected {
			pSetTextColor.Call(hdc, colText)
		}
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontIcon))
		drawText(hdc, n.icon, &iconR, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)

		if sidebarWidth >= 150 {
			pSetTextColor.Call(hdc, map[bool]uintptr{true: colText, false: colMuted}[selected || hovered])
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
			labelR := RECT{68, y, sidebarWidth - 14, y + navHeight}
			drawText(hdc, n.title, &labelR, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
		}
	}

	if sidebarWidth >= 154 {
		// Personal quote block from the selected profile-hub design. It gives the
		// rail identity without adding another navigation destination.
		quoteTop := rc.Bottom - 236
		pSetTextColor.Call(hdc, colMuted)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontSubtitle))
		qr := RECT{22, quoteTop, sidebarWidth - 18, quoteTop + 72}
		drawText(hdc, "„Kleine Ideen können\ngroße Wirkung haben.“", &qr, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		by := RECT{22, quoteTop + 74, sidebarWidth - 18, quoteTop + 96}
		drawText(hdc, "— Markus Kleine", &by, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)

		vr := RECT{20, rc.Bottom - 38, sidebarWidth - 14, rc.Bottom - 15}
		drawText(hdc, "LogiMate  •  "+displayVersion(Version)+"  •  "+buildLabel(), &vr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func paintMain(hdc uintptr, rc RECT) {
	x := sidebarWidth + 28 + pageOffset()
	right := rc.Right - 28
	if right-x < 420 {
		right = x + 420
	}

	// Profile-hub header: small icon chip + clear page hierarchy.
	pageIcon := "•"
	for _, n := range navItems {
		if n.page == currentPage {
			pageIcon = n.icon
			break
		}
	}
	iconBox := RECT{x, 24, x + 38, 62}
	headerIconFill := blendColor(colAccentSoft, colPanel2, 72)
	fillRoundRect(hdc, iconBox, 11, headerIconFill)
	pSetTextColor.Call(hdc, readableTextColor(colAccent, headerIconFill))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontIcon))
	drawText(hdc, pageIcon, &iconBox, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)

	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontTitle))
	titleR := RECT{x + 50, 20, right - 170, 57}
	drawText(hdc, pageTitle(), &titleR, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSubtitle))
	subR := RECT{x + 50, 58, right - 150, 82}
	drawText(hdc, pageSubtitle(), &subR, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	stateMu.RLock()
	s := appState
	stateMu.RUnlock()

	chip := RECT{right - 145, 27, right, 60}
	fillRoundRect(hdc, chip, 16, colBadge)
	chipText, chipColor := readinessChip(s)
	if currentPage == pageAbout {
		chipText, chipColor = "v"+displayVersion(Version), colAccent
	} else if busy {
		chipText, chipColor = "●  Prüfe …", colAccent
	}
	pSetTextColor.Call(hdc, chipColor)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	drawText(hdc, chipText, &chip, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	if pageShowsStatusBand() {
		status := RECT{x, 101, right, 207}
		modernCard(hdc, status, 0)
		gap := int32(12)
		innerW := (status.Right - status.Left - 30 - 3*gap) / 4
		backupValue, backupColor := backupStatus(s)
		items := []struct {
			label, value string
			color        uintptr
		}{
			{"Lenkrad", compactWheelName(s.WheelModel), statusColor(system.HasActionableSelectedWheel(s))},
			{"Modus", compactMode(s.ActiveMode), statusColor(s.ActiveMode != "" && s.ActiveMode != "KEIN LOGITECH WHEEL" && s.ActiveMode != "AUSWAHL ERFORDERLICH")},
			{"Speicherschutz", map[bool]string{true: "Aktiv", false: "Aus"}[s.HVCI], hvciStatusColor(s)},
			{"Backup", backupValue, backupColor},
		}
		memoryIntegrityRect = RECT{}
		for i, it := range items {
			l := status.Left + 15 + int32(i)*(innerW+gap)
			r := RECT{l, status.Top + 14, l + innerW, status.Bottom - 14}
			fill := colPanel2
			if i == 2 {
				memoryIntegrityRect = r
				if memoryIntegrityHovered {
					fill = colHover
				}
			}
			fillRoundRect(hdc, r, 13, fill)
			dot := RECT{r.Left + 14, r.Top + 15, r.Left + 21, r.Top + 22}
			fillRoundRect(hdc, dot, 4, it.color)
			pSetTextColor.Call(hdc, colMuted2)
			old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
			lr := RECT{r.Left + 29, r.Top + 8, r.Right - 10, r.Top + 31}
			drawText(hdc, it.label, &lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
			pSetTextColor.Call(hdc, colText)
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
			vr := RECT{r.Left + 14, r.Top + 36, r.Right - 10, r.Bottom - 8}
			drawText(hdc, it.value, &vr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
		}
	}

	content := RECT{x, contentTopForPage(), right, rc.Bottom - 96}
	if currentPage == pageSettings {
		modernCard(hdc, content, 0)
		section := RECT{content.Left + 20, content.Top + 14, content.Right - 20, content.Top + 42}
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBrand))
		drawText(hdc, pageSectionTitle(), &section, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		paintSettingsContent(hdc, content)
		paintActionFeedback(hdc, rc)
		paintActionButtons(hdc, rc)
		return
	}
	if currentPage == pageWheel {
		modernCard(hdc, content, 0)
		sectionRight := content.Right - 20
		wheelAdvancedToggleRect = RECT{}
		if wheelSubtab == wheelSubtabLive {
			wheelAdvancedToggleRect = RECT{content.Right - 196, content.Top + 10, content.Right - 20, content.Top + 43}
			sectionRight = wheelAdvancedToggleRect.Left - 12
		}
		section := RECT{content.Left + 20, content.Top + 14, sectionRight, content.Top + 42}
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBrand))
		drawText(hdc, pageSectionTitle(), &section, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)

		if wheelSubtab == wheelSubtabLive {
			buttonFill, buttonBorder, buttonText := colPanel2, colBorder, colText
			if wheelAdvancedView {
				buttonFill = blendColor(colAccentSoft, colPanel2, 24)
				buttonBorder = colAccent
				buttonText = readableTextColor(colAccent, buttonFill)
			}
			if wheelAdvancedHovered {
				buttonFill = blendColor(buttonFill, colText, 8)
			}
			if keyboardFocus == focusWheelAdvanced {
				buttonBorder = colFocusRing
			}
			drawRoundRect(hdc, wheelAdvancedToggleRect, 11, buttonFill, buttonBorder)
			pSetTextColor.Call(hdc, buttonText)
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
			advancedLabel := "Erweiterte Ansicht  ▾"
			if wheelAdvancedView {
				advancedLabel = "Standardansicht  ▴"
			}
			drawText(hdc, advancedLabel, &wheelAdvancedToggleRect, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
		}

		paintWheelDashboard(hdc, content, s)
		paintActionFeedback(hdc, rc)
		paintActionButtons(hdc, rc)
		return
	}

	// Overview, System, Diagnostics and About are true dashboard pages. They no
	// longer inherit a giant generic text surface.
	modernPageSurface(hdc, content, s)
	paintActionFeedback(hdc, rc)
	paintActionButtons(hdc, rc)
}

func paintActionFeedback(hdc uintptr, rc RECT) {
	if strings.TrimSpace(actionFeedback) == "" {
		return
	}
	x := sidebarWidth + 28 + pageOffset()
	right := rc.Right - 28
	if right-x < 420 {
		right = x + 420
	}
	w := int32(300)
	if right-x < w {
		w = right - x
	}
	r := RECT{right - w, rc.Bottom - 111, right, rc.Bottom - 84}
	drawRoundRect(hdc, r, 13, colBadge, colBorder)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
	drawText(hdc, actionFeedback, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}

func actionLayout(rc RECT) {
	x := sidebarWidth + 28 + pageOffset()
	right := rc.Right - 28
	if right-x < 420 {
		right = x + 420
	}
	visible := make([]int, 0, len(actionButtons))
	for i, b := range actionButtons {
		if b.visible {
			visible = append(visible, i)
		}
		actionRects[i] = RECT{}
	}
	if len(visible) == 0 {
		return
	}
	gap := int32(10)
	available := right - x
	bw := (available - gap*int32(len(visible)-1)) / int32(len(visible))
	if bw > 236 {
		bw = 236
	}
	if bw < 132 {
		bw = 132
	}
	bh := int32(48)
	by := rc.Bottom - 76
	bx := x
	for _, i := range visible {
		actionRects[i] = RECT{bx, by, bx + bw, by + bh}
		bx += bw + gap
	}
}

func memoryIntegrityHitTest(x, y int32) bool {
	if !pageShowsStatusBand() {
		return false
	}
	r := memoryIntegrityRect
	return r.Right > r.Left && r.Bottom > r.Top && x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom
}

func activateMemoryIntegritySettings() {
	if err := openMemoryIntegritySettings(); err != nil {
		messageBox(mainWnd, "Die Windows-Sicherheitseinstellung konnte nicht geöffnet werden:\r\n\r\n"+err.Error(), "Speicherintegrität", MB_OK|MB_ICONERROR)
	} else {
		setActionFeedback("Windows-Sicherheit · Speicherintegrität geöffnet. LogiMate ändert die Einstellung nicht selbst.")
	}
}

func openMemoryIntegritySettings() error {
	// This URI opens Windows Security/Core Isolation. LogiMate intentionally
	// never toggles Memory Integrity/HVCI itself.
	uri := "windowsdefender://coreisolation"
	r, _, callErr := pShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(uri))), 0, 0, SW_SHOWNORMAL)
	if r <= 32 {
		// Fallback for Windows builds that do not accept the Windows Security URI.
		fallback := "ms-settings:windowsdefender"
		r, _, callErr = pShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(fallback))), 0, 0, SW_SHOWNORMAL)
	}
	if r <= 32 {
		return fmt.Errorf("Windows Security konnte nicht geöffnet werden (ShellExecute=%d, %v)", r, callErr)
	}
	return nil
}

func wheelAdvancedHitTest(x, y int32) bool {
	if currentPage != pageWheel {
		return false
	}
	r := wheelAdvancedToggleRect
	if r.Right <= r.Left || r.Bottom <= r.Top {
		return false
	}
	return x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom
}

func actionHitTest(x, y int32) int {
	if mainWnd == 0 {
		return -1
	}
	var rc RECT
	pGetClientRect.Call(uintptr(mainWnd), uintptr(unsafe.Pointer(&rc)))
	actionLayout(rc)
	for i, b := range actionButtons {
		if !b.visible || !b.enabled {
			continue
		}
		r := actionRects[i]
		if x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom {
			return i
		}
	}
	return -1
}

func paintActionButtons(hdc uintptr, rc RECT) {
	actionLayout(rc)
	for i, b := range actionButtons {
		if !b.visible {
			continue
		}
		r := actionRects[i]
		hovered := hoveredAction == i
		pressed := pressedAction == i

		fill := colPanel2
		border := colBorder
		iconFill := blendColor(colPanel2, colText, 8)
		iconText := colMuted
		text := colText
		if !b.enabled {
			fill = blendColor(colPanel2, themeBackground, 45)
			border = blendColor(colBorder, themeBackground, 42)
			iconFill = fill
			iconText = colMuted2
			text = colMuted2
		} else if b.primary {
			fill = blendColor(colAccentSoft, colPanel2, 18)
			border = colAccent
			iconFill = colAccent
			iconText = colOnAccent
		} else if b.warning {
			fill = colWarningSurface
			border = colWarning
			iconFill = blendColor(colWarningSurface, colText, 18)
			iconText = colText
		}
		if b.enabled && hovered {
			if b.primary {
				fill = blendColor(fill, colText, 8)
			} else if b.warning {
				fill = blendColor(fill, colText, 7)
			} else {
				fill = colHover
			}
			border = blendColor(border, colText, 18)
		}
		if b.enabled && pressed {
			fill = blendColor(fill, colPressed, 34)
			border = blendColor(border, colPressed, 18)
		}

		if keyboardFocus == focusActionBase+i {
			border = colFocusRing
		}
		drawRoundRect(hdc, r, 14, fill, border)
		chip := RECT{r.Left + 8, r.Top + 8, r.Left + 40, r.Bottom - 8}
		fillRoundRect(hdc, chip, 10, iconFill)
		pSetTextColor.Call(hdc, iconText)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontBody))
		drawText(hdc, b.icon, &chip, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)

		pSetTextColor.Call(hdc, text)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
		tr := RECT{chip.Right + 10, r.Top, r.Right - 12, r.Bottom}
		drawText(hdc, b.label, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func blendColor(a, b uintptr, pct int) uintptr {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	ar, ag, ab := int(a&0xff), int((a>>8)&0xff), int((a>>16)&0xff)
	br, bg, bb := int(b&0xff), int((b>>8)&0xff), int((b>>16)&0xff)
	mix := func(x, y int) byte { return byte((x*(100-pct) + y*pct) / 100) }
	return rgb(mix(ar, br), mix(ag, bg), mix(ab, bb))
}

func drawRoundRect(hdc uintptr, r RECT, radius int32, fill, border uintptr) {
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
	oldBrush, _, _ := pSelectObject.Call(hdc, uintptr(brush))
	oldPen, _, _ := pSelectObject.Call(hdc, pen)
	pRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius), uintptr(radius))
	pSelectObject.Call(hdc, oldBrush)
	pSelectObject.Call(hdc, oldPen)
}

type settingDef struct {
	group string
	title string
	desc  string
}

type settingsGroupMeta struct {
	title string
	desc  string
}

type settingsGroupHeader struct {
	meta settingsGroupMeta
	rect RECT
}

var settingGroups = map[string]settingsGroupMeta{
	"appearance": {"Darstellung", "Material, Bewegung und Navigation"},
	"wheel":      {"Lenkrad & Verhalten", "Status und LogiMate Native Engine"},
	"updates":    {"Updates", "Versionen, Downloads und Changelog"},
	"startup":    {"Windows & Einstieg", "Autostart und Ersteinrichtung"},
}

var settingDefs = []settingDef{
	{"appearance", "Transparenz / Windows-Material", "Dunkel: Acrylic · Grau: Mica Alt · Hell: Mica"},
	{"appearance", "Animationen", "Weiche Sidebar- und Seitenwechsel-Animationen"},
	{"appearance", "Breite Profil-Sidebar", "Aktiv: dauerhaft breit · Aus: kompakte Icon-Leiste"},
	{"wheel", "Status automatisch aktualisieren", "Lenkrad-, Treiber- und Sicherheitsstatus alle 7 Sekunden"},
	{"wheel", "Native Wheel Output (Experimental)", "Sichere Rotation/LED/Autocenter-Testbefehle direkt aus LogiMate freischalten"},
	{"updates", "Beim Start nach Updates suchen", "GitHub im Hintergrund auf eine neuere LogiMate-Version prüfen"},
	{"updates", "Updates automatisch herunterladen", "Neue Versionen nach Fund im Hintergrund laden und SHA-256 prüfen"},
	{"updates", "Preview-Versionen erhalten", "Alpha-, Beta- und RC-Releases in die Update-Suche einbeziehen"},
	{"updates", "Was ist neu automatisch anzeigen", "Nach Versionswechsel das Changelog-Fenster öffnen"},
	{"startup", "Mit Windows starten", "LogiMate automatisch bei der Benutzeranmeldung öffnen"},
	{"startup", "Ersteinrichtung beim Start anbieten", "Optionalen Assistenten beim ersten/erneut aktivierten Start anbieten"},
}

func settingsValues(p uiSettings) []bool {
	return []bool{
		p.Acrylic, p.Animations, p.SidebarAutoExpand, p.AutoRefresh, p.NativeWheelOutput,
		p.CheckUpdates, p.AutoDownloadUpdate, p.PreviewUpdates, p.ShowWhatsNew, p.StartWithWindows, p.OfferSetup,
	}
}

func pageShowsStatusBand() bool {
	// The redesigned pages own their status cards. The wheel page keeps the
	// compact global band because live diagnostics benefit from seeing it.
	return currentPage == pageWheel
}

func contentTopForPage() int32 {
	if pageShowsStatusBand() {
		return 224
	}
	return 101
}

func contentRectFor(rc RECT) RECT {
	gap := int32(28)
	if rc.Right-rc.Left < 980 {
		gap = 16
	}
	x := sidebarWidth + gap + pageOffset()
	right := rc.Right - gap
	// Never manufacture an off-screen content width. Older builds forced at
	// least 420 px even when the client area was smaller, which clipped text and
	// controls at high DPI or small remote-desktop sizes.
	if right < x+280 {
		x = min32(sidebarWidth+10+pageOffset(), max32(10, right-280))
	}
	if right < x+80 {
		right = x + 80
	}
	bottom := rc.Bottom - 72
	if bottom < contentTopForPage()+120 {
		bottom = contentTopForPage() + 120
	}
	return RECT{x, contentTopForPage(), right, bottom}
}

func screenMetric(index uintptr) int32 {
	r, _, _ := pGetSystemMetrics.Call(index)
	return int32(r)
}

func responsiveMinimumTrackSize(dpi uint32, screenW, screenH int32) (int32, int32) {
	if dpi == 0 {
		dpi = 96
	}
	preferredW := int32(1080) * int32(dpi) / 96
	preferredH := int32(740) * int32(dpi) / 96
	if screenW > 0 {
		preferredW = min32(preferredW, max32(760, screenW-64))
	}
	if screenH > 0 {
		preferredH = min32(preferredH, max32(560, screenH-96))
	}
	return preferredW, preferredH
}

func clampContentScrollForViewport(rc RECT) {
	if contentScroll < 0 {
		contentScroll = 0
	}
	if contentScrollMax >= 0 && contentScroll > contentScrollMax {
		contentScroll = contentScrollMax
	}
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

const (
	settingRowH     int32 = 62
	settingRowGap   int32 = 6
	settingGroupH   int32 = 48
	settingGroupGap int32 = 12
	settingFooterH  int32 = 82
)

func offsetRect(r RECT, dy int32) RECT {
	r.Top += dy
	r.Bottom += dy
	return r
}

// settingsLayout is the single source of truth for drawing and hit-testing.
// Keeping the geometry here avoids the old long-list layout being duplicated
// in multiple event paths.
func settingsLayout(content RECT) (rows []RECT, headers []settingsGroupHeader, footerTop, totalHeight int32) {
	rows = make([]RECT, len(settingDefs))
	y := content.Top + 55 + themePickerHeight
	lastGroup := ""
	for i, d := range settingDefs {
		if d.group != lastGroup {
			if lastGroup != "" {
				y += settingGroupGap
			}
			meta := settingGroups[d.group]
			headers = append(headers, settingsGroupHeader{meta: meta, rect: RECT{content.Left + 20, y, content.Right - 20, y + settingGroupH}})
			y += settingGroupH
			lastGroup = d.group
		}
		rows[i] = RECT{content.Left + 20, y, content.Right - 20, y + settingRowH}
		y += settingRowH + settingRowGap
	}
	footerTop = y + 8
	totalHeight = footerTop + settingFooterH - (content.Top + 52)
	return
}

func paintSettingsContent(hdc uintptr, content RECT) {
	prefs := getUISettings()
	values := settingsValues(prefs)
	rows, headers, footerTop, totalHeight := settingsLayout(content)
	settingRects = make([]RECT, len(rows))

	visibleTop := content.Top + 52
	visibleBottom := content.Bottom - 16
	visibleHeight := visibleBottom - visibleTop
	contentScrollMax = totalHeight - visibleHeight
	if contentScrollMax < 0 {
		contentScrollMax = 0
	}
	if contentScroll > contentScrollMax {
		contentScroll = contentScrollMax
	}

	saved, _, _ := pSaveDC.Call(hdc)
	pIntersectClipRect.Call(hdc, uintptr(content.Left+16), uintptr(visibleTop), uintptr(content.Right-14), uintptr(visibleBottom))
	paintThemePicker(hdc, content)

	for _, h := range headers {
		r := offsetRect(h.rect, -contentScroll)
		if r.Bottom < visibleTop || r.Top > visibleBottom {
			continue
		}
		pSetTextColor.Call(hdc, colText)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontSection))
		title := RECT{r.Left + 2, r.Top, r.Right, r.Top + 24}
		drawText(hdc, h.meta.title, &title, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		desc := RECT{r.Left + 2, r.Top + 23, r.Right, r.Bottom}
		drawFittedParagraph(hdc, h.meta.desc, desc, colMuted2, fontSmall, fontSmall)
		pSelectObject.Call(hdc, old)
	}

	for i, d := range settingDefs {
		r := offsetRect(rows[i], -contentScroll)
		settingRects[i] = r
		if r.Bottom < visibleTop || r.Top > visibleBottom {
			continue
		}
		rowColor := colPanel2
		if hoveredSetting == i {
			rowColor = colHover
		}
		if keyboardFocus == focusSettingBase+i {
			drawRoundRect(hdc, r, 13, rowColor, colFocusRing)
		} else {
			drawRoundRect(hdc, r, 13, rowColor, colBorder)
		}

		pSetTextColor.Call(hdc, colText)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontBody))
		tr := RECT{r.Left + 15, r.Top + 6, r.Right - 92, r.Top + 29}
		drawText(hdc, d.title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)

		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		dr := RECT{r.Left + 15, r.Top + 29, r.Right - 92, r.Bottom - 5}
		drawFittedParagraph(hdc, d.desc, dr, colMuted2, fontSmall, fontSmall)
		pSelectObject.Call(hdc, old)

		midY := (r.Top + r.Bottom) / 2
		paintToggle(hdc, RECT{r.Right - 68, midY - 13, r.Right - 18, midY + 13}, values[i])
	}

	footerTop -= contentScroll
	themeLabel := map[string]string{"dark": "Dunkel", "gray": "Grau", "light": "Hell", "system": "System"}[normalizeThemeMode(prefs.ThemeMode)]
	footer := fmt.Sprintf("Darstellung: %s  ·  Windows-Material: %s  ·  Renderer: %s\r\nUpdater: %s  ·  github.com/%s\r\n\r\nSicherheit: Treiber-Backup und Bestätigung kritischer Aktionen bleiben immer aktiv.", themeLabel, onOff(prefs.Acrylic), rendererStatus, getUpdateStatus(), system.DefaultLogiMateRepository)
	fr := RECT{content.Left + 22, footerTop, content.Right - 22, footerTop + settingFooterH}
	drawFittedParagraph(hdc, footer, fr, colMuted2, fontSmall, fontSmall)

	if saved != 0 {
		pRestoreDC.Call(hdc, saved)
	}
	paintScrollBar(hdc, content, visibleHeight, totalHeight)
}

func paintToggle(hdc uintptr, r RECT, enabled bool) {
	track := colToggleOff
	if enabled {
		track = colAccent
	}
	fillRoundRect(hdc, r, 13, track)
	knobW := int32(18)
	x := r.Left + 5
	if enabled {
		x = r.Right - knobW - 5
	}
	knob := RECT{x, r.Top + 4, x + knobW, r.Bottom - 4}
	knobColor := colToggleKnobOff
	if enabled {
		knobColor = colToggleKnobOn
	}
	fillRoundRect(hdc, knob, 9, knobColor)
}

func paintScrollBar(hdc uintptr, content RECT, visibleHeight, totalHeight int32) {
	if contentScrollMax <= 0 {
		return
	}
	track := RECT{content.Right - 8, content.Top + 58, content.Right - 5, content.Bottom - 18}
	fillRoundRect(hdc, track, 2, colTrack)
	trackH := track.Bottom - track.Top
	thumbH := max32(30, trackH*visibleHeight/max32(totalHeight, 1))
	travel := trackH - thumbH
	thumbY := track.Top
	if contentScrollMax > 0 {
		thumbY += travel * contentScroll / contentScrollMax
	}
	thumb := RECT{track.Left, thumbY, track.Right, thumbY + thumbH}
	fillRoundRect(hdc, thumb, 2, colAccentSoft)
}

func settingsHitTest(x, y int32) int {
	if currentPage != pageSettings || mainWnd == 0 {
		return -1
	}
	var rc RECT
	pGetClientRect.Call(uintptr(mainWnd), uintptr(unsafe.Pointer(&rc)))
	content := contentRectFor(rc)
	if x < content.Left+18 || x > content.Right-18 || y < content.Top+52 || y > content.Bottom-16 {
		return -1
	}
	rows, _, _, _ := settingsLayout(content)
	for i, row := range rows {
		r := offsetRect(row, -contentScroll)
		if x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom {
			return i
		}
	}
	return -1
}

func toggleSetting(idx int) {
	if idx < 0 || idx >= len(settingDefs) {
		return
	}
	p := getUISettings()
	switch idx {
	case 0:
		p.Acrylic = !p.Acrylic
		if p.Acrylic && safeUI {
			queueNotice("Safe UI ist für diesen Start aktiv. Die Einstellung für Transparenz / Windows-Material wurde gespeichert und gilt wieder beim nächsten normalen Start.", "Darstellung", MB_OK|MB_ICONINFORMATION)
		}
	case 1:
		p.Animations = !p.Animations
	case 2:
		p.SidebarAutoExpand = !p.SidebarAutoExpand
	case 3:
		p.AutoRefresh = !p.AutoRefresh
	case 4:
		if !p.NativeWheelOutput {
			if p.NativeOutputRiskVersion != nativeOutputRiskTextVersion || strings.TrimSpace(p.NativeOutputRiskAcceptedAt) == "" {
				msg := "Native Wheel Output steuert Motor, Lenkwinkel und LEDs direkt. Ein Fehler, USB-Abbruch oder inkompatibles Gerät kann unerwartete Kräfte verursachen.\r\n\r\nNur mit eindeutig erkanntem Wheel verwenden, Hände beim ersten Test locker halten und jederzeit USB/Netzteil trennen können.\r\n\r\nDiese Risikobestätigung wird versionsgebunden gespeichert. Native Output jetzt freischalten?"
				if messageBox(mainWnd, msg, "Native Wheel Output · Sicherheitsfreigabe", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
					return
				}
				p.NativeOutputRiskVersion = nativeOutputRiskTextVersion
				p.NativeOutputRiskAcceptedAt = time.Now().UTC().Format(time.RFC3339)
			}
			p.NativeWheelOutput = true
		} else {
			// Turning output off is itself a hardware transition. Do not persist
			// the switch as disabled until the wheel has been proven neutral.
			if err := system.NativeOutputEmergencyStop(appStateSnapshot()); err != nil {
				queueNotice("Native Wheel Output bleibt aktiviert, weil das Lenkrad nicht sicher neutralisiert werden konnte:\r\n\r\n"+err.Error(), "Native Output · Sicherheitsabbruch", MB_OK|MB_ICONERROR)
				return
			}
			if err := system.NativeOutputRelease(); err != nil {
				queueNotice("Native Wheel Output bleibt aktiviert, weil der Output-Lease nicht sicher freigegeben werden konnte:\r\n\r\n"+err.Error(), "Native Output · Sicherheitsabbruch", MB_OK|MB_ICONERROR)
				return
			}
			p.NativeWheelOutput = false
		}
	case 5:
		p.CheckUpdates = !p.CheckUpdates
	case 6:
		p.AutoDownloadUpdate = !p.AutoDownloadUpdate
	case 7:
		p.PreviewUpdates = !p.PreviewUpdates
	case 8:
		p.ShowWhatsNew = !p.ShowWhatsNew
	case 9:
		want := !p.StartWithWindows
		if err := setStartupEnabled(want); err != nil {
			queueNotice(err.Error(), "Windows-Autostart", MB_OK|MB_ICONERROR)
			return
		}
		p.StartWithWindows = want
	case 10:
		p.OfferSetup = !p.OfferSetup
		if p.OfferSetup {
			p.SetupCompleted = false
		}
	}
	if err := replaceUISettings(p); err != nil {
		queueNotice(err.Error(), "Einstellungen speichern", MB_OK|MB_ICONERROR)
		return
	}
	applyUISettings()
	invalidate(mainWnd)
}

func applyUISettings() {
	p := getUISettings()
	if !p.Animations {
		pKillTimer.Call(uintptr(mainWnd), timerAnim)
		animRunning = false
		pageAnim = 1
	}
	if p.SidebarAutoExpand {
		sidebarTarget = sidebarExpanded
	} else {
		sidebarTarget = sidebarCollapsed
	}
	startAnimation()
	if !p.Acrylic {
		disableWindowMaterial(mainWnd)
		if changelogWnd != 0 {
			disableWindowMaterial(changelogWnd)
			changelogGlassActive = false
		}
		if setupWnd != 0 {
			disableWindowMaterial(setupWnd)
			setupGlassActive = false
		}
	} else if !safeUI {
		materialStarted = false
		startWindowMaterialAsync(mainWnd)
		if changelogWnd != 0 {
			changelogGlassActive = applyWindowMaterial(changelogWnd)
		}
		if setupWnd != 0 {
			setupGlassActive = applyWindowMaterial(setupWnd)
		}
	}
	layout(mainWnd)
}

func disableWindowMaterial(hwnd HWND) {
	if hwnd == 0 {
		return
	}
	resetWindowMaterial(hwnd)
	if hwnd == mainWnd {
		glassActive = false
		materialStarted = false
	}
	if hwnd == changelogWnd {
		changelogGlassActive = false
	}
	if hwnd == setupWnd {
		setupGlassActive = false
	}
	logStartup("Windows material disabled/reset for window=%v", uintptr(hwnd))
}

func resetUISettings() {
	if messageBox(mainWnd, "Darstellungs- und Verhaltenseinstellungen auf die empfohlenen Standardwerte zurücksetzen?", "Einstellungen zurücksetzen", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
		return
	}
	_ = setStartupEnabled(false)
	p := defaultUISettings()
	p.StartWithWindows = false
	if err := resetUISettingsStorage(p); err != nil {
		messageBox(mainWnd, err.Error(), "Einstellungen", MB_OK|MB_ICONERROR)
		return
	}
	rebuildThemeResources()
	applyUISettings()
	setActionFeedback("Einstellungen zurückgesetzt.")
	invalidate(mainWnd)
}

func paintHoverTip(hdc uintptr, rc RECT) {
	if hoveredNav < 0 || hoveredNav >= len(navItems) || sidebarWidth >= 150 {
		return
	}
	if getUISettings().SidebarAutoExpand && sidebarTarget == sidebarExpanded {
		return
	}
	y := navTop + int32(hoveredNav)*(navHeight+navGap)
	// Keep the tooltip anchored to the collapsed rail. Using the animated
	// sidebarWidth here made the tooltip move every frame and visibly flicker.
	r := RECT{sidebarCollapsed + 9, y + 5, sidebarCollapsed + 151, y + navHeight - 5}
	fillRoundRect(hdc, r, 10, colPanel2)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
	drawText(hdc, navItems[hoveredNav].title, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	_ = rc
}

func fillRoundRect(hdc uintptr, r RECT, radius int32, color uintptr) {
	brush, ok := roundBrushes[color]
	if !ok || brush == 0 {
		brush = createSolidBrush(color)
		roundBrushes[color] = brush
	}
	pen, ok := roundPens[color]
	if !ok || pen == 0 {
		pen, _, _ = pCreatePen.Call(PS_SOLID, 1, color)
		roundPens[color] = pen
	}
	oldBrush, _, _ := pSelectObject.Call(hdc, uintptr(brush))
	oldPen, _, _ := pSelectObject.Call(hdc, pen)
	pRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius), uintptr(radius))
	pSelectObject.Call(hdc, oldBrush)
	pSelectObject.Call(hdc, oldPen)
}

func drawText(hdc uintptr, s string, r *RECT, flags uint32) {
	u := syscall.StringToUTF16(s)
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(r)), uintptr(flags))
}

func measureText(hdc uintptr, s string, r *RECT, flags uint32) {
	u := syscall.StringToUTF16(s)
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(r)), uintptr(flags|DT_CALCRECT))
}

// drawFittedParagraph prevents custom-painted text from being silently clipped.
// It first tries the preferred font, then the compact fallback and finally
// draws an ellipsized wrapped paragraph if the available card area is still too
// small (for example at 150-200% DPI or with a long translated/error string).
// The return value reports whether the full text fit without ellipsis.
func drawFittedParagraph(hdc uintptr, text string, r RECT, color uintptr, preferred, fallback HFONT) bool {
	if strings.TrimSpace(text) == "" || r.Right <= r.Left || r.Bottom <= r.Top {
		return true
	}
	availableH := r.Bottom - r.Top
	fitsWith := func(font HFONT) bool {
		old, _, _ := pSelectObject.Call(hdc, uintptr(font))
		m := RECT{r.Left, r.Top, r.Right, r.Top + 8192}
		measureText(hdc, text, &m, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
		return m.Bottom-m.Top <= availableH
	}
	font := preferred
	full := fitsWith(font)
	if !full && fallback != 0 && fallback != preferred {
		font = fallback
		full = fitsWith(font)
	}
	pSetTextColor.Call(hdc, color)
	old, _, _ := pSelectObject.Call(hdc, uintptr(font))
	flags := uint32(DT_LEFT | DT_WORDBREAK | DT_NOPREFIX)
	if !full {
		flags |= DT_END_ELLIPSIS
	}
	drawText(hdc, text, &r, flags)
	pSelectObject.Call(hdc, old)
	return full
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func compactWheelName(s string) string {
	if s == "" {
		return "Nicht erkannt"
	}
	if strings.Contains(s, "Kompatibilitätsmodus") {
		return "C294 • Modell offen"
	}
	if strings.Contains(strings.ToLower(s), "driving force gt") {
		return "Driving Force GT"
	}
	if strings.Contains(s, "G27") {
		return "Logitech G27"
	}
	if strings.Contains(s, "G25") {
		return "Logitech G25"
	}
	return s
}

func compactMode(s string) string {
	if s == "" {
		return "Wird geprüft"
	}
	s = strings.ReplaceAll(s, "Generic HID / Modern", "Generic HID")
	s = strings.ReplaceAll(s, "KEIN LOGITECH WHEEL", "Nicht erkannt")
	return s
}

func statusColor(ok bool) uintptr {
	if ok {
		return colGood
	}
	return colBad
}

func pageTitle() string {
	return []string{"Startseite", "Lenkrad", "System", "Diagnose", "Einstellungen", "Über mich"}[currentPage]
}

func pageSubtitle() string {
	if currentPage == pageWheel {
		switch wheelSubtab {
		case wheelSubtabFFB:
			return "Force Feedback, Signalformung und sichere Live-Tests"
		case wheelSubtabCalibration:
			return "Lenkung, Pedale, Tasten und H-Shifter direkt am ausgewählten Wheel"
		case wheelSubtabProfiles:
			return "Wheel-, Spiel- und effektive Einstellungen transparent zusammengeführt"
		case wheelSubtabDevice:
			return "Geräteidentität, HID-Pfade, Reports und technische Details"
		}
	}
	return []string{
		"Alles Wichtige auf einen Blick · klar, sicher und modern",
		"Live-Daten, Geräteauswahl und Hardwaretest",
		"Treiber, Backups, Betriebsmodus und Wheel Engine",
		"Fehleranalyse, Support-Berichte und Recovery",
		"Darstellung, Verhalten, Autostart und Updates",
		"LogiMate, Entwickler, Open Source, Datenschutz und Support",
	}[currentPage]
}

func pageSectionTitle() string {
	if currentPage == pageWheel {
		switch wheelSubtab {
		case wheelSubtabFFB:
			return "Wheel Control Panel"
		case wheelSubtabCalibration:
			return "Kalibrierung"
		case wheelSubtabProfiles:
			return "Profile & effektive Einstellungen"
		case wheelSubtabDevice:
			return "Gerät & HID"
		}
	}
	return []string{"Dashboard", "Live-Hardwaretest", "Treiber & Wheel Engine", "Diagnose & Support", "Darstellung & Verhalten", "Über LogiMate"}[currentPage]
}

func detectionEvidenceLabel(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Noch keine eindeutige Quelle"
	}
	return s
}

func backupStatus(s system.State) (string, uintptr) {
	if s.BackupCount > 0 {
		return fmt.Sprintf("%d INF", s.BackupCount), colGood
	}
	if len(s.LegacyDrivers) > 0 {
		return "Fehlt", colBad
	}
	return "Keins nötig", colMuted
}

func hvciStatusColor(s system.State) uintptr {
	if s.HVCI && s.ActiveMode == "Logitech Legacy" {
		return colWarning
	}
	if s.HVCI {
		return colGood
	}
	return colMuted
}

func readinessChip(s system.State) (string, uintptr) {
	if s.DeviceDetectionError != "" {
		return "●  Erkennung prüfen", colBad
	}
	if s.LegacyDriverError != "" {
		return "●  Treiberprüfung bei Bedarf", colWarning
	}
	if len(s.Wheels) == 0 || s.SelectedWheelID == "" || !system.HasActionableSelectedWheel(s) {
		return "●  Aktion nötig", colWarning
	}
	if len(s.LegacyDrivers) > 0 && s.BackupCount == 0 {
		return "●  Backup fehlt", colWarning
	}
	return "●  Bereit", colGood
}

func overviewPrimaryAction(s system.State) string {
	if s.DeviceDetectionError != "" {
		return "Erkennung neu starten"
	}
	if len(s.Wheels) == 0 {
		return "Geräte verwalten"
	}
	if s.SelectedWheelID == "" || len(s.Wheels) > 1 && !system.HasReadableSelectedWheel(s) {
		return "Geräte verwalten"
	}
	if system.SelectedWheelNeedsModelConfirmation(s) {
		return "Modell bestätigen"
	}
	if !system.HasActionableSelectedWheel(s) {
		return "Geräte verwalten"
	}
	if len(s.LegacyDrivers) > 0 && s.BackupCount == 0 && s.LegacyDriverError == "" {
		return "Treiber sichern"
	}
	return "Hardware testen"
}

func runOverviewPrimaryAction(s system.State) {
	switch overviewPrimaryAction(s) {
	case "Erkennung neu starten":
		manualRefreshPending = true
		setActionFeedback("Hardware wird vollständig neu erkannt …")
		refreshAsync()
	case "Geräte verwalten":
		showWheelManager(s)
	case "Modell bestätigen":
		showSetupAssistant(s)
	case "Treiber sichern":
		elevate("backup", s.DataDir)
	default:
		setPage(pageWheel)
	}
}

func dashboardText(s system.State) string {
	var b strings.Builder
	b.WriteString("EMPFOHLENER NÄCHSTER SCHRITT\r\n")
	b.WriteString(setupRecommendation(s))
	b.WriteString("\r\n\r\n")

	selected := "keins"
	selectedDetail := "Noch kein Ziel ausgewählt"
	if w, ok := system.SelectedWheel(s); ok {
		selected = w.Name
		selectedDetail = system.WheelDeviceLabel(w)
	}
	fmt.Fprintf(&b, "Lenkrad & Auswahl\r\n%s\r\nAktiver Modus: %s\r\nErkennungsweg: %s\r\nAuswahlstatus: %s\r\nErkannte Räder: %d\r\nAusgewählt: %s\r\n%s\r\n", s.WheelModel, s.ActiveMode, detectionEvidenceLabel(s.DetectionEvidence), s.SelectionStatus, len(s.Wheels), selected, selectedDetail)
	if s.DeviceDetectionError != "" {
		fmt.Fprintf(&b, "Erkennungsfehler: %s\r\n", s.DeviceDetectionError)
	}
	b.WriteString("\r\n")

	backupValue, _ := backupStatus(s)
	fmt.Fprintf(&b, "Sicherheit & Wiederherstellung\r\nSpeicherintegrität / HVCI: %s\r\nTreiber-Backup: %s\r\n%s\r\n\r\n", onOff(s.HVCI), backupValue, modeChangeGuidance(s))

	engineGate := "READY"
	if err := system.NativeEngineCoreGate(); err != nil {
		engineGate = "BLOCKED: " + err.Error()
	}
	snap := system.NativeWheelEngineSnapshot()
	fmt.Fprintf(&b, "Wheel Engine\r\nLogiMate Native: %s\r\nEngine-Generation: %d\r\nExterne Wheel-Runtime erforderlich: Nein\r\nLogitech LCore läuft: %s\r\n", engineGate, snap.Generation, yesNo(s.LCoreRunning))
	b.WriteString("\r\n\r\n")
	fmt.Fprintf(&b, "Windows & Umgebung\r\n%s\r\nRenderer: %s\r\n", s.OS, rendererStatus)
	return b.String()
}

func wheelText(s system.State, j system.JoyState) string {
	var b strings.Builder
	b.WriteString(system.FormatJoy(j))
	b.WriteString("\r\n\r\nERKENNUNG\r\n")
	b.WriteString("Modellquelle: " + detectionEvidenceLabel(s.DetectionEvidence) + "\r\n")
	b.WriteString("Auswahlstatus: " + s.SelectionStatus + "\r\n")
	if s.DeviceDetectionError != "" {
		b.WriteString("Erkennungsfehler: " + s.DeviceDetectionError + "\r\n")
	}
	if len(s.Wheels) > 1 {
		b.WriteString(fmt.Sprintf("Mehrere Räder: %d erkannt · Ziel-ID: %s\r\n", len(s.Wheels), map[bool]string{true: s.SelectedWheelID, false: "nicht ausgewählt"}[s.SelectedWheelID != ""]))
		for i, w := range s.Wheels {
			mark := "  "
			if s.SelectedWheelID != "" && strings.EqualFold(w.ID, s.SelectedWheelID) {
				mark = "→ "
			}
			b.WriteString(fmt.Sprintf("%s%d. %s · %s\r\n", mark, i+1, system.WheelDeviceLabel(w), w.Mode))
		}
	}
	b.WriteString("\r\nSPIELPROFILE / WHEEL ENGINE\r\n")
	if system.IsG25Model(s.WheelModel) {
		b.WriteString("G25 erkannt. LogiMate verwendet denselben Native-Kern für Direct HID, Kalibrierung, Range, FFB, Profile und Game-Output. Reale G25-Motorzertifizierung bleibt vor Stable erforderlich.\r\n")
	} else if system.IsDFGTModel(s.WheelModel) {
		b.WriteString("Driving Force GT erkannt. LogiMate verwendet denselben Native-Kern für Direct HID, Kalibrierung, Range, FFB, Profile und Game-Output. Modellfähigkeiten verhindern G27-spezifische LED-Befehle.\r\n")
	} else if system.IsG27Model(s.WheelModel) {
		b.WriteString("G27 erkannt. LogiMate Native stellt Direct HID, Kalibrierung, Range, FFB, Rev-LEDs, Spielprofile und Wreckfest-2-Pino ohne externe Wheel-Runtime bereit.\r\n")
	} else {
		b.WriteString("Noch kein eindeutig bestätigtes Modell. Der Live-Test bleibt verfügbar; modellabhängige Engine-Aktionen werden zurückgehalten.\r\n")
	}
	if system.IsCompatibilityModel(s.WheelModel) {
		b.WriteString("\r\nMODELLIDENTITÄT\r\nPID_C294 ist ein gemeinsamer Logitech-Kompatibilitätsmodus und nicht exklusiv für G25/G27/DFGT. Bestätige ein Modell nur, wenn das angeschlossene Wheel tatsächlich eines der von LogiMate unterstützten Modelle G25, G27 oder Driving Force GT ist; C299/C29A/C29B haben immer Vorrang.")
	} else if strings.Contains(s.WheelModel, "manuell bestätigt / C294") {
		b.WriteString("\r\nMODELLIDENTITÄT\r\nDas Modell wurde für den gemeinsamen C294-Modus manuell gespeichert. Sobald Windows C299 (G25), C29A (Driving Force GT) oder C29B (G27) sieht, überschreibt die native PID diese Vorgabe automatisch.")
	} else if strings.Contains(s.WheelModel, "HID-Produkt bestätigt / C294") {
		b.WriteString("\r\nMODELLIDENTITÄT\r\nDer direkte HID-Scan meldet für den gemeinsamen C294-Modus einen expliziten G25/G27-Produktnamen. Diese Bestätigung gilt nur für die aktuelle Gerätesitzung und wird nicht blind auf ein anderes C294 übertragen.")
	} else if strings.Contains(s.WheelModel, "WinMM bestätigt / C294") {
		b.WriteString("\r\nMODELLIDENTITÄT\r\nWindows-PnP meldet den gemeinsamen C294-Modus; WinMM liefert zusätzlich einen expliziten G25/G27-Produktnamen. Diese Bestätigung gilt nur für die aktuelle Erkennung und wird nicht blind dauerhaft gespeichert.")
	}
	return b.String()
}

func driverText(s system.State) string {
	var b strings.Builder
	selected := "keins"
	if w, ok := system.SelectedWheel(s); ok {
		selected = system.WheelDeviceLabel(w)
	}
	fmt.Fprintf(&b, "Erkanntes Lenkrad: %s\r\nAktiver Treibermodus: %s\r\nPhysische Räder: %d\r\nAuswahlstatus: %s\r\nAusgewähltes Ziel: %s\r\n\r\n", s.WheelModel, s.ActiveMode, len(s.Wheels), s.SelectionStatus, selected)
	fmt.Fprintf(&b, "LOGITECH LEGACY-PAKETE\r\nGefunden: %d\r\n", len(s.LegacyDrivers))
	for _, d := range s.LegacyDrivers {
		fmt.Fprintf(&b, "• %s  •  %s  •  %s\r\n", d.PublishedName, d.OriginalName, d.ProviderName)
	}
	fmt.Fprintf(&b, "\r\nBACKUP\r\nVollständig gesicherte INF-Dateien: %d\r\nDatenordner: %s\r\n", s.BackupCount, s.DataDir)
	return b.String()
}

func nativeEngineText(s system.State) string {
	if system.IsCompatibilityModel(s.WheelModel) {
		return "LOGIMATE NATIVE WHEEL ENGINE\r\nDas Lenkrad befindet sich im gemeinsamen Logitech-PID_C294-Kompatibilitätsmodus. Modellabhängige Native-Engine-Aktionen und Treiberwechsel bleiben gesperrt, bis G25, G27 oder Driving Force GT sicher bestätigt ist."
	}
	gate := "READY"
	detail := "Standalone Core-Gate bestanden"
	if err := system.NativeEngineCoreGate(); err != nil {
		gate = "BLOCKED"
		detail = err.Error()
	}
	snap := system.NativeWheelEngineSnapshot()
	return fmt.Sprintf("LOGIMATE NATIVE WHEEL ENGINE\r\nStatus: %s · %s\r\nEngine-Generation: %d\r\n\r\nG25, G27 und Driving Force GT verwenden im Modern-Modus ausschließlich LogiMates eigene Geräte-, Input-, FFB-, Profil-, Adapter- und HID-Transport-Schichten. Es wird keine externe Wheel-Runtime gesucht, installiert oder gestartet.\r\n\r\nOpenG27 bleibt ausschließlich als historisches Importformat und Referenz/Provenance im Quellprojekt erhalten.", gate, detail, snap.Generation)
}

func systemText(s system.State) string {
	var modes strings.Builder
	modes.WriteString("SICHERHEITSSTATUS\r\n" + modeChangeGuidance(s) + "\r\n\r\n")
	modes.WriteString("BETRIEBSMODI\r\n")
	modes.WriteString("Generic HID: moderner Windows-Modus. G25, G27 und Driving Force GT verwenden den gemeinsamen LogiMate-Native-Kern; modellabhängige Fähigkeiten werden automatisch begrenzt.\r\n")
	modes.WriteString("Original Logitech / Legacy: alter Logitech-WingMan/LGS-Modus; kann mit aktivierter Speicherintegrität inkompatibel sein.\r\n\r\n")
	return modes.String() + driverText(s) + "\r\n\r\n" + nativeEngineText(s)
}

func modeChangeGuidance(s system.State) string {
	if s.DeviceDetectionError != "" {
		return "Treiberwechsel gesperrt: Die Geräteerkennung ist fehlgeschlagen. Erst den Erkennungsfehler beheben."
	}
	if len(s.Wheels) > 1 {
		return "Treiberwechsel gesperrt: Mehrere unterstützte Lenkräder sind gleichzeitig verbunden. Wähle dein Ziel für Diagnose aus und trenne für einen Moduswechsel die anderen Räder."
	}
	if !system.HasActionableSelectedWheel(s) {
		if w, ok := system.SelectedWheel(s); ok && system.IsCompatibilityModel(w.Model) {
			return "Treiberwechsel gesperrt: C294 ist erkannt, aber das genaue Modell ist noch unbestätigt. Bestätige G25, G27 oder Driving Force GT zuerst am aktuell angeschlossenen Gerät."
		}
		return "Treiberwechsel gesperrt: Es ist noch kein eindeutig unterstütztes Zielgerät ausgewählt."
	}
	if s.LegacyDriverError != "" {
		return "Treiberwechsel gesperrt: Die Legacy-Treiberinventur konnte nicht zuverlässig gelesen werden."
	}
	if len(s.LegacyDrivers) > 0 && s.BackupCount == 0 {
		return "Empfehlung: Vor dem ersten Wechsel ein Treiber-Backup erstellen. LogiMate bietet dieses bewusst als primäre System-Aktion an."
	}
	return "Bereit: Das ausgewählte Lenkrad ist eindeutig. Kritische Änderungen bleiben bestätigt, protokolliert und rollback-orientiert."
}

func setupRecommendation(s system.State) string {
	prefs := getUISettings()
	if prefs.OfferSetup && !prefs.SetupCompleted && prefs.SetupResumeStep > 0 {
		step := prefs.SetupResumeStep + 1
		if step > setupPageCount {
			step = setupPageCount
		}
		return fmt.Sprintf("Ersteinrichtung pausiert · Schritt %d/%d. Öffne System → Einrichtung prüfen, um genau dort weiterzumachen. Bereits abgeschlossene Schritte bleiben erhalten.", step, setupPageCount)
	}
	if len(s.Wheels) > 1 && s.SelectedWheelID == "" {
		return "Mehrere unterstützte Lenkräder erkannt. Öffne Lenkrad → Geräte verwalten und wähle zuerst das Ziel; Treiberaktionen bleiben bis dahin gesperrt."
	}
	if len(s.Devices) == 0 {
		return "Kein unterstütztes Logitech-Lenkrad erkannt. Öffne Lenkrad → Geräte verwalten und starte dort „Neu erkennen“. Prüfe zusätzlich USB/Strom."
	}
	if system.SelectedWheelNeedsModelConfirmation(s) {
		return "Das Modell ist noch nicht sicher bestätigt. Nutze „Modell bestätigen“ oder den Einrichtungsassistenten und wähle das physische Modell. Eine native PID hat weiterhin Vorrang."
	}
	if s.BackupCount == 0 && len(s.LegacyDrivers) > 0 {
		return "Vor jedem Treiberwechsel zuerst unter System ein vollständiges Treiber-Backup erstellen."
	}
	if system.HasActionableSelectedWheel(s) && strings.Contains(s.ActiveMode, "Generic") {
		if err := system.NativeEngineCoreGate(); err != nil {
			return "Das Wheel läuft im Generic-HID-Modus, aber der Native-Engine-Sicherheitscheck ist blockiert. Öffne Native Engine Status, bevor du Hardware-Ausgabe verwendest."
		}
		return "Das Wheel läuft mit LogiMate Native. Für den Modern-Betrieb wird keine externe Wheel-Runtime benötigt; Hardware-Ausgabe bleibt bis zur jeweiligen Modellzertifizierung Alpha/Experimental."
	}
	return "Grundstatus sieht plausibel aus. Nutze Hardware testen für Eingaben oder System nur dann, wenn du Treiber/Modus ändern möchtest."
}

func yesNo(v bool) string {
	if v {
		return "Ja"
	}
	return "Nein"
}
func onOff(v bool) string {
	if v {
		return "Aktiv"
	}
	return "Aus"
}

func action(page, idx int) {
	stateMu.RLock()
	s := appState
	stateMu.RUnlock()
	switch page {
	case pageOverview:
		switch idx {
		case 0:
			runOverviewPrimaryAction(s)
		case 1:
			showSetupAssistant(s)
		case 2:
			manualRefreshPending = true
			setActionFeedback("Status wird aktualisiert …")
			refreshAsync()
		case 3:
			setPage(pageDiagnostics)
		}
	case pageWheel:
		needsConfirmation := system.SelectedWheelNeedsModelConfirmation(s)
		switch idx {
		case 0:
			if err := system.OpenGameControllers(); err != nil {
				messageBox(mainWnd, err.Error(), "Windows-Controller-Test", MB_OK|MB_ICONERROR)
			} else {
				setActionFeedback("Windows-Controller-Test geöffnet.")
			}
		case 1:
			showWheelManager(s)
		case 2:
			setPage(pageDiagnostics)
		case 3:
			if len(s.Wheels) == 0 {
				setPage(pageDiagnostics)
			} else if needsConfirmation {
				showSetupAssistant(s)
			} else if !system.HasActionableSelectedWheel(s) {
				setPage(pageDiagnostics)
			} else if system.HasActionableSelectedWheel(s) && strings.Contains(s.ActiveMode, "Generic") {
				showEngineHealth(s)
			} else {
				setPage(pageSystem)
			}
		}
	case pageSystem:
		switch idx {
		case 0:
			elevate("backup", s.DataDir)
		case 1:
			doSwitchOpen(s)
		case 2:
			doSwitchLegacy(s)
		case 3:
			showEngineHealth(s)
		}
	case pageDiagnostics:
		switch idx {
		case 0:
			exportDiag(s)
		case 1:
			if setClipboardText(mainWnd, diagnosticReportWithAccessibility(s)) {
				setActionFeedback("Diagnosebericht kopiert.")
			} else {
				messageBox(mainWnd, "Die Zwischenablage konnte nicht geöffnet werden.", "LogiMate", MB_OK|MB_ICONERROR)
			}
		case 2:
			p := filepath.Join(s.DataDir, "Diagnostics")
			_ = os.MkdirAll(p, 0755)
			if err := system.OpenFolder(p); err != nil {
				messageBox(mainWnd, err.Error(), "Diagnoseordner", MB_OK|MB_ICONERROR)
			} else {
				setActionFeedback("Diagnoseordner geöffnet.")
			}
		case 3:
			manualRefreshPending = true
			setActionFeedback("Status wird aktualisiert …")
			refreshAsync()
		}
	case pageSettings:
		switch idx {
		case 0:
			setActionFeedback("Update-Prüfung gestartet …")
			checkForUpdatesAsync(true)
		case 1:
			showCurrentWhatsNew()
		case 2:
			if err := system.OpenFolder(s.DataDir); err != nil {
				messageBox(mainWnd, err.Error(), "Datenordner", MB_OK|MB_ICONERROR)
			} else {
				setActionFeedback("Datenordner geöffnet.")
			}
		case 3:
			resetUISettings()
		}
	case pageAbout:
		switch idx {
		case 0:
			if err := openExternalURL(aboutPayPalURL); err != nil {
				messageBox(mainWnd, err.Error(), "PayPal", MB_OK|MB_ICONERROR)
			} else {
				setActionFeedback("PayPal im Browser geöffnet. Danke für deine Unterstützung ♥")
			}
		case 1:
			if err := openExternalURL(aboutGitHubURL); err != nil {
				messageBox(mainWnd, err.Error(), "GitHub", MB_OK|MB_ICONERROR)
			}
		case 2:
			projectPanelOpen = true
			invalidate(mainWnd)
		case 3:
			if setClipboardText(mainWnd, aboutPayPalEmail) {
				setActionFeedback("PayPal-Adresse kopiert: " + aboutPayPalEmail)
			} else {
				messageBox(mainWnd, "Die PayPal-Adresse konnte nicht kopiert werden.", "PayPal", MB_OK|MB_ICONERROR)
			}
		}
	}
}

func openExternalURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" || !strings.HasPrefix(strings.ToLower(url), "https://") {
		return fmt.Errorf("ungültige externe Adresse")
	}
	r, _, callErr := pShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(utf16("open"))),
		uintptr(unsafe.Pointer(utf16(url))),
		0, 0, SW_SHOWNORMAL,
	)
	if r <= 32 {
		return fmt.Errorf("die Adresse konnte nicht im Standardbrowser geöffnet werden (ShellExecute=%d, %v)", r, callErr)
	}
	return nil
}

func showWheelManager(s system.State) {
	const (
		wheelButtonBase       = 1000
		wheelRefreshID        = 1901
		wheelResetID          = 1902
		wheelHIDDetailsID     = 1903
		wheelNativeActivateID = 1904
	)

	buttons := make([]modernDialogButton, 0, len(s.Wheels)+3)
	defaultID := wheelRefreshID
	for i, w := range s.Wheels {
		marker := ""
		if s.SelectedWheelID != "" && strings.EqualFold(w.ID, s.SelectedWheelID) {
			marker = " · AKTIV"
			defaultID = wheelButtonBase + i
		}
		buttons = append(buttons, modernDialogButton{
			ID:          wheelButtonBase + i,
			Title:       system.WheelDeviceLabel(w) + marker,
			Description: w.Mode + " · " + w.Evidence,
			Primary:     defaultID == wheelButtonBase+i,
		})
	}
	if w, ok := system.SelectedWheel(s); ok && system.IsG27Model(w.Model) && system.HasActionableSelectedWheel(s) && strings.Contains(strings.ToUpper(w.InstanceID), "PID_C294") && !strings.Contains(strings.ToLower(s.ActiveMode), "legacy") {
		buttons = append(buttons, modernDialogButton{ID: wheelNativeActivateID, Title: "G27 jetzt nativ verbinden (C294 → C29B)", Description: "Schaltet das G27 sicher in den nativen Modus und zeigt bei Fehlern die exakte Stufe", Primary: true})
		defaultID = wheelNativeActivateID
	}
	buttons = append(buttons,
		modernDialogButton{ID: wheelRefreshID, Title: "Neu erkennen", Description: "HID-first + USB/PnP + Raw Input vollständig neu erkennen", Primary: len(s.Wheels) == 0},
		modernDialogButton{ID: wheelHIDDetailsID, Title: "Direkte HID-Erkennung anzeigen", Description: "Zeigt C294/C299/C29A/C29B-HID-Kandidaten und Report-Capabilities wie der OpenG27-Rettungspfad"},
		modernDialogButton{ID: wheelResetID, Title: "Erkennung zurücksetzen", Description: "Gespeicherte Geräteauswahl und C294-Modellbestätigungen löschen", Warning: true},
	)

	content := "Wähle das Lenkrad, das LogiMate für Live-Diagnose verwenden soll. Die Auswahl bleibt gespeichert und springt beim Abziehen niemals still auf ein anderes Gerät."
	if s.SelectedWheelID != "" {
		if _, ok := system.SelectedWheel(s); !ok {
			content += " Das zuvor ausgewählte Lenkrad ist aktuell nicht verbunden; LogiMate wartet bewusst auf deine Entscheidung."
		}
	}
	if len(s.Wheels) == 0 {
		content = "Aktuell wurde kein unterstütztes Lenkrad erkannt. Du kannst die Hardware neu erkennen oder gespeicherte Erkennungsdaten zurücksetzen."
	} else if len(s.Wheels) > 1 {
		content += " Treiberwechsel bleiben bei mehreren gleichzeitig angeschlossenen Rädern aus Sicherheitsgründen gesperrt."
	}

	result, ok := runModernDialog(modernDialogSpec{
		Parent:       mainWnd,
		WindowTitle:  "LogiMate · Geräte verwalten",
		Heading:      "Lenkräder erkennen und auswählen",
		Subtitle:     "LogiMate · Geräteverwaltung",
		Content:      content,
		Kind:         "info",
		Buttons:      buttons,
		CommandLinks: true,
		DefaultID:    defaultID,
		CancelID:     IDCANCEL,
		Width:        760,
		Height:       650,
	})
	if !ok {
		messageBox(mainWnd, "Die moderne Geräteverwaltung konnte nicht geöffnet werden. Bitte starte LogiMate einmal neu und öffne anschließend Diagnose, falls der Fehler bleibt.", "Geräte verwalten", MB_OK|MB_ICONERROR)
		return
	}
	switch result {
	case wheelRefreshID:
		manualRefreshPending = true
		setActionFeedback("Hardware wird vollständig neu erkannt …")
		refreshAsync()
	case wheelHIDDetailsID:
		messageBox(mainWnd, system.DirectHIDCandidateDiagnostics(s.RawInputDevices), "Direkte HID-Erkennung", MB_OK|MB_ICONINFORMATION)
	case wheelNativeActivateID:
		setActionFeedback("G27 Native-Aktivierung läuft …")
		if err := system.ActivateSelectedWheelNativeMode(s); err != nil {
			phase, detail, _ := system.NativeActivationStatus()
			messageBox(mainWnd, fmt.Sprintf("Native-Aktivierung fehlgeschlagen.\r\n\r\nPhase: %s\r\nDetail: %s\r\n\r\n%s", phase, detail, err.Error()), "G27 C294 → C29B", MB_OK|MB_ICONERROR)
		} else {
			messageBox(mainWnd, "G27 wurde als C29B neu erkannt. LogiMate bindet den Direct-HID-Pfad jetzt neu.", "G27 C294 → C29B", MB_OK|MB_ICONINFORMATION)
		}
		manualRefreshPending = true
		refreshAsync()
	case wheelResetID:
		resetWheelDetection(s)
	default:
		idx := result - wheelButtonBase
		if idx >= 0 && idx < len(s.Wheels) {
			selectWheel(s, s.Wheels[idx])
		}
	}
}

func selectWheel(s system.State, w system.WheelDevice) {
	if !system.CanPersistWheelSelection(w) {
		messageBox(mainWnd, "Dieses Gerät stammt nur aus einer transienten/Fallback-Erkennung und besitzt keine verifizierte stabile PnP-Identität. LogiMate speichert eine solche Auswahl nicht dauerhaft. Bitte Windows die Geräteerkennung abschließen lassen und erneut aktualisieren.", "Lenkrad auswählen", MB_OK|MB_ICONWARNING)
		return
	}
	if err := system.NativeOutputEmergencyStop(s); err != nil {
		messageBox(mainWnd, "Lenkradwechsel wurde blockiert, weil der bisherige Hardware-Output nicht sicher neutralisiert werden konnte:\r\n\r\n"+err.Error(), "Lenkrad auswählen · Sicherheitsabbruch", MB_OK|MB_ICONERROR)
		return
	}
	if err := system.NativeOutputRelease(); err != nil {
		messageBox(mainWnd, "Lenkradwechsel wurde blockiert, weil der Output-Lease nicht sicher freigegeben werden konnte:\r\n\r\n"+err.Error(), "Lenkrad auswählen · Sicherheitsabbruch", MB_OK|MB_ICONERROR)
		return
	}
	if err := system.SaveSelectedWheelID(s.DataDir, w.ID); err != nil {
		messageBox(mainWnd, err.Error(), "Lenkrad auswählen", MB_OK|MB_ICONERROR)
		return
	}
	system.ClosePreferredInput()
	system.InvalidateInputCaches()
	setActionFeedback("Ausgewählt: " + system.WheelDeviceLabel(w))
	refreshFastAsync()
}

func resetWheelDetection(s system.State) {
	if messageBox(mainWnd, "Geräteauswahl und manuelle C294-Modellerkennung zurücksetzen?\r\n\r\nTreiber, Backups und Pedal-Kalibrierungen werden nicht gelöscht. Anschließend wird die Hardware vollständig neu erkannt.", "Erkennung zurücksetzen", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
		return
	}
	if err := system.NativeOutputEmergencyStop(s); err != nil {
		messageBox(mainWnd, "Erkennung wurde nicht zurückgesetzt, weil der Hardware-Output nicht sicher neutralisiert werden konnte:\r\n\r\n"+err.Error(), "Erkennung zurücksetzen · Sicherheitsabbruch", MB_OK|MB_ICONERROR)
		return
	}
	if err := system.NativeOutputRelease(); err != nil {
		messageBox(mainWnd, "Erkennung wurde nicht zurückgesetzt, weil der Output-Lease nicht sicher freigegeben werden konnte:\r\n\r\n"+err.Error(), "Erkennung zurücksetzen · Sicherheitsabbruch", MB_OK|MB_ICONERROR)
		return
	}
	if err := system.ClearWheelDetectionState(s.DataDir); err != nil {
		messageBox(mainWnd, err.Error(), "Erkennung zurücksetzen", MB_OK|MB_ICONERROR)
		return
	}
	setActionFeedback("Lenkraderkennung wurde zurückgesetzt und wird neu aufgebaut …")
	manualRefreshPending = true
	refreshAsync()
}

func doSwitchOpen(s system.State) {
	if len(s.Wheels) > 1 {
		messageBox(mainWnd, "Mehrere unterstützte Logitech-Lenkräder sind gleichzeitig verbunden. Für einen sicheren Moduswechsel bitte alle anderen Räder vorübergehend trennen. Geräteauswahl und Live-Diagnose bleiben trotzdem verfügbar.", "Modern / Generic HID", MB_OK|MB_ICONWARNING)
		return
	}
	if !system.CanChangeSelectedWheelMode(s) {
		if system.IsCompatibilityModel(s.WheelModel) {
			messageBox(mainWnd, "Das gemeinsame C294-Gerät ist erkannt, aber das genaue Modell ist noch nicht bestätigt. Öffne zuerst die Einrichtung und bestätige G25, G27 oder Driving Force GT.", "Modern / Generic HID", MB_OK|MB_ICONWARNING)
		} else {
			messageBox(mainWnd, "Kein eindeutig ausgewähltes, unterstütztes Logitech-Lenkrad ist verbunden. LogiMate verändert ohne genau ein Zielgerät keine Treiber.", "Modern / Generic HID", MB_OK|MB_ICONWARNING)
		}
		return
	}
	if s.LegacyDriverError != "" {
		setActionFeedback("Treiberinventur wird beim Administrator-Schritt erneut geprüft.")
	}
	// Always use the same guided transaction as first-run setup. The previous
	// direct System-button path had subtly different backup/HVCI rules,
	// which made identical mode changes behave differently depending on where the
	// user clicked.
	showSetupAssistantForMode(s, "modern")
}

func doSwitchLegacy(s system.State) {
	if len(s.Wheels) > 1 {
		messageBox(mainWnd, "Mehrere unterstützte Logitech-Lenkräder sind gleichzeitig verbunden. Für einen sicheren Legacy-Wechsel bitte alle anderen Räder vorübergehend trennen. Geräteauswahl und Live-Diagnose bleiben trotzdem verfügbar.", "Original Logitech / Legacy", MB_OK|MB_ICONWARNING)
		return
	}
	if !system.CanChangeSelectedWheelMode(s) {
		if system.IsCompatibilityModel(s.WheelModel) {
			messageBox(mainWnd, "Das gemeinsame C294-Gerät ist erkannt, aber das genaue Modell ist noch nicht bestätigt. Öffne zuerst die Einrichtung und bestätige G25, G27 oder Driving Force GT.", "Original Logitech / Legacy", MB_OK|MB_ICONWARNING)
		} else {
			messageBox(mainWnd, "Kein eindeutig ausgewähltes, unterstütztes Logitech-Lenkrad ist verbunden. LogiMate verändert ohne genau ein Zielgerät keine Treiber.", "Original Logitech / Legacy", MB_OK|MB_ICONWARNING)
		}
		return
	}
	if s.LegacyDriverError != "" {
		setActionFeedback("Treiberinventur wird beim Administrator-Schritt erneut geprüft.")
	}
	showSetupAssistantForMode(s, "legacy")
}

func exportDiag(s system.State) {
	p, err := system.ExportDiagnostics(s.DataDir, withDiagnosticMeta(s))
	if err != nil {
		messageBox(mainWnd, err.Error(), "Diagnose", MB_OK|MB_ICONERROR)
		return
	}
	messageBox(mainWnd, "Diagnose-ZIP erstellt:\r\n"+p, "Diagnose", MB_OK|MB_ICONINFORMATION)
	if err := system.OpenFolder(filepath.Dir(p)); err != nil {
		messageBox(mainWnd, "Die Diagnose wurde erstellt, aber der Ordner konnte nicht geöffnet werden:\r\n"+err.Error(), "Diagnoseordner", MB_OK|MB_ICONWARNING)
	} else {
		setActionFeedback("Diagnose erstellt und Ordner geöffnet.")
	}
}

func elevate(action, dataDir string) bool {
	return elevateMigration(action, dataDir, "")
}

func elevateMigration(action, dataDir, migrationToken string) bool {
	args := fmt.Sprintf("--admin-action %s --data \"%s\"", action, strings.ReplaceAll(dataDir, "\"", ""))
	if migrationToken != "" {
		args += fmt.Sprintf(" --migration-token \"%s\"", strings.ReplaceAll(migrationToken, "\"", ""))
	}
	if !shellRunAs(currentExe(), args) {
		if migrationToken != "" {
			system.CancelMigration(dataDir, migrationToken, "Windows-UAC wurde abgebrochen oder konnte nicht gestartet werden.")
		}
		messageBox(mainWnd, "Windows-UAC wurde abgebrochen oder konnte nicht gestartet werden.", "Administratorrechte", MB_OK|MB_ICONWARNING)
		return false
	}
	setActionFeedback("Administratoraktion gestartet …")
	time.AfterFunc(4*time.Second, requestRefresh)
	return true
}

func AdminAction(action, dataDir, migrationToken string) int {
	if dataDir == "" {
		dataDir, _ = system.GetDataDir()
	}
	title := "LogiMate"
	if strings.HasPrefix(action, "setup-modern") || strings.HasPrefix(action, "setup-legacy") {
		return runTrackedSetupAdminAction(action, dataDir, migrationToken)
	}
	switch action {
	case "backup":
		msg, err := system.BackupDrivers(dataDir)
		if err != nil {
			messageBox(0, msg+"\r\n\r\n"+err.Error(), title, MB_OK|MB_ICONERROR)
			return 10
		}
		messageBox(0, msg, title, MB_OK|MB_ICONINFORMATION)
	case "switch-open", "switch-open-g27", "switch-open-g27-direct", "switch-legacy", "switch-legacy-hvci":
		// Deprecated pre-transaction entry points. Keeping these executable would
		// create a second driver-changing path that bypasses the current target
		// identity, backup journal and rollback policy. Old shortcuts/scripts fail
		// closed and the user must start the guided Modern/Legacy flow in LogiMate.
		messageBox(0, "Dieser alte direkte Moduswechsel wird aus Sicherheitsgründen nicht mehr ausgeführt. Öffne LogiMate und starte den Modern-/Legacy-Wechsel über System bzw. den Einrichtungsassistenten.", title, MB_OK|MB_ICONWARNING)
		return 14
	}
	return 0
}

func runTrackedSetupAdminAction(action, dataDir, migrationToken string) (code int) {
	if migrationToken == "" {
		messageBox(0, "Sicherheitsabbruch: Setup-Migration ohne persistentes Journal ist nicht zulässig.", "LogiMate Einrichtung", MB_OK|MB_ICONERROR)
		return 22
	}
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("Migration sicher abgebrochen: %v", r)
			_ = system.CompleteMigration(dataDir, migrationToken, false, "Migration wegen Journal-/Laufzeitfehler abgebrochen", err)
			messageBox(0, err.Error()+"\r\n\r\nDer aktuelle Zustand wird nicht als erfolgreich verbucht. Beim nächsten Start prüft LogiMate das unvollständige Journal erneut.", "LogiMate Einrichtung", MB_OK|MB_ICONERROR)
			code = 23
		}
	}()
	if err := system.MarkMigrationRunning(dataDir, migrationToken, action); err != nil {
		messageBox(0, "Migrationsjournal konnte nicht sicher gestartet werden:\r\n"+err.Error(), "LogiMate Einrichtung", MB_OK|MB_ICONERROR)
		return 22
	}
	progress := func(name, status, detail string) error {
		return system.AppendMigrationStep(dataDir, migrationToken, name, status, detail)
	}
	finish := func(code int, msg string, err error) int {
		if migrationToken != "" {
			if journalErr := system.CompleteMigration(dataDir, migrationToken, err == nil, msg, err); journalErr != nil {
				messageBox(0, "Migration beendet, aber der Abschluss konnte nicht dauerhaft protokolliert werden:\r\n"+journalErr.Error()+"\r\n\r\nDer Zustand gilt deshalb NICHT als bestätigt.", "LogiMate Einrichtung", MB_OK|MB_ICONERROR)
				return 24
			}
			return code
		}
		if err != nil {
			messageBox(0, strings.TrimSpace(msg+"\r\n\r\n"+err.Error()), "LogiMate Einrichtung", MB_OK|MB_ICONERROR)
		} else {
			messageBox(0, msg, "LogiMate Einrichtung", MB_OK|MB_ICONINFORMATION)
		}
		return code
	}

	if strings.HasPrefix(action, "setup-modern") {
		uninstallProfiler := strings.Contains(action, "uninstall-profiler")
		msg, err := system.SetupModernGenericTracked(dataDir, uninstallProfiler, progress)
		if err != nil {
			return finish(20, msg, err)
		}
		msg += "\r\n\r\nDie moderne Einrichtung ist abgeschlossen."
		return finish(0, msg, nil)
	}

	withProfiler := strings.Contains(action, "profiler")
	// Build 018 security invariant: LogiMate never changes Memory Integrity/HVCI.
	// Legacy setup fails closed while it is effectively enabled and directs the
	// user to Windows Security for a deliberate, user-owned change.
	if system.HVCIEnabled() {
		err := errors.New("Speicherintegrität/HVCI ist aktiv. LogiMate ändert diese Windows-Sicherheitsfunktion nicht automatisch. Öffne Windows-Sicherheit > Gerätesicherheit > Kernisolierung, ändere die Einstellung bewusst selbst und starte Windows neu, bevor du Legacy erneut versuchst")
		_ = progress("Speicherintegrität", "blocked", err.Error())
		return finish(21, "Legacy-Einrichtung wurde sicher blockiert.", err)
	}
	msg, err := system.SetupLegacyTracked(dataDir, withProfiler, progress)
	if err != nil {
		return finish(22, msg, err)
	}
	return finish(0, msg, nil)
}

func uninstallSafetyPreflight(title string) (func(), bool) {
	release, err := system.AcquireApplicationInstanceLock()
	if err != nil {
		messageBox(0, "Deinstallation wurde blockiert, weil eine andere LogiMate-Instanz läuft oder die Instanzsperre nicht sicher übernommen werden konnte.\r\n\r\nBeende LogiMate vollständig und versuche es erneut.\r\n\r\n"+err.Error(), title, MB_OK|MB_ICONWARNING)
		return nil, false
	}
	dataDir, _ := system.GetDataDir()
	if need, detail := system.RuntimeOutputRecoveryNeeded(dataDir); need {
		release()
		messageBox(0, "Deinstallation wurde blockiert, weil noch ein ungeklärter Native-Output-Recovery-Zustand existiert. Starte LogiMate zuerst normal und lasse den sicheren Recovery-Vorgang abschließen.\r\n\r\n"+detail, title, MB_OK|MB_ICONERROR)
		return nil, false
	}
	return release, true
}

func Uninstall() int {
	exe := currentExe()
	if isAdmin() {
		return UninstallAdmin()
	}
	// Preflight under the same singleton used by the normal GUI. Release before
	// UAC hand-off; the elevated process repeats the check and then holds it for
	// the entire destructive phase, so a racing GUI start fails closed.
	release, ok := uninstallSafetyPreflight("LogiMate Uninstall")
	if !ok {
		return 4
	}
	release()
	if !shellRunAs(exe, "--uninstall-admin") {
		messageBox(0, "Administratorrechte wurden nicht erteilt.", "LogiMate Uninstall", MB_OK|MB_ICONWARNING)
		return 1
	}
	return 0
}

func UninstallAdmin() int {
	release, ok := uninstallSafetyPreflight("LogiMate deinstallieren")
	if !ok {
		return 4
	}
	defer release()

	exe := currentExe()
	programFiles := strings.TrimSpace(os.Getenv("ProgramFiles"))
	installedRoot := filepath.Clean(filepath.Join(programFiles, "LogiMate"))
	expectedExe := filepath.Join(installedRoot, "LogiMate.exe")
	if programFiles == "" || !strings.EqualFold(filepath.Clean(exe), filepath.Clean(expectedExe)) {
		messageBox(0, "Diese Kopie scheint eine portable Version zu sein. Eine portable EXE wird nicht über den Installer-Uninstaller entfernt.", "LogiMate", MB_OK|MB_ICONWARNING)
		return 2
	}
	if messageBox(0, "LogiMate wirklich deinstallieren?\r\n\r\nTreiber-Backups und Benutzerdaten unter LocalAppData bleiben erhalten.", "LogiMate deinstallieren", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
		return 0
	}
	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "LogiMate.lnk")
	publicDesktop := filepath.Join(os.Getenv("PUBLIC"), "Desktop", "LogiMate.lnk")
	_ = os.Remove(desktop)
	_ = os.Remove(publicDesktop)
	startMenu := filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs", "LogiMate")
	_ = os.RemoveAll(startMenu)
	if err := system.DeleteMachineRegistryTree(`Software\Microsoft\Windows\CurrentVersion\Uninstall\LogiMate`); err != nil {
		messageBox(0, "Der Deinstallations-Eintrag konnte nicht vollständig aus der Registry entfernt werden:\r\n"+err.Error(), "LogiMate deinstallieren", MB_OK|MB_ICONWARNING)
	}
	installDir := filepath.Clean(filepath.Dir(exe))
	if !strings.EqualFold(installDir, installedRoot) {
		messageBox(0, "Deinstallation wurde blockiert: Installationspfad ist nicht der verwaltete LogiMate-Ordner.", "LogiMate deinstallieren", MB_OK|MB_ICONERROR)
		return 3
	}
	quotedDir := strings.ReplaceAll(installDir, "'", "''")
	ps := "Start-Sleep -Seconds 2; Remove-Item -LiteralPath '" + quotedDir + "' -Recurse -Force"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		messageBox(0, "Die Verknüpfungen wurden entfernt, aber der Programmordner konnte nicht zur verzögerten Löschung eingeplant werden:\r\n"+err.Error()+"\r\n\r\nOrdner: "+installDir, "LogiMate deinstallieren", MB_OK|MB_ICONWARNING)
		return 3
	}
	messageBox(0, "LogiMate wird entfernt. Benutzerdaten und Treiber-Backups wurden bewusst beibehalten.", "LogiMate", MB_OK|MB_ICONINFORMATION)
	return 0
}

func cleanup() {
	system.ClosePreferredInput()
	releaseBrandIcons()
	releaseBackBuffer()
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
	deleteObject(uintptr(glassBrush))
	deleteObject(uintptr(fontBody))
	deleteObject(uintptr(fontTitle))
	deleteObject(uintptr(fontSubtitle))
	deleteObject(uintptr(fontSmall))
	deleteObject(uintptr(fontBrand))
	deleteObject(uintptr(fontIcon))
	deleteObject(uintptr(fontSection))
	deleteObject(uintptr(fontMetric))
	deleteObject(uintptr(fontLabel))
}

func init() { _ = createControls }

var controlsOnce sync.Once

func ensureControls(hwnd HWND) { controlsOnce.Do(func() { createControls(hwnd) }) }
