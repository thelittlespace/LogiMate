//go:build windows

package app

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"unsafe"

	"github.com/thelittlespace/LogiMate/internal/system"
)

const setupPageCount = 8

const (
	setupMigrationProgressMsg = WM_APP + 81
	setupMigrationAnimTimer   = 81
)

var (
	setupWnd                HWND
	setupClassRegistered    bool
	setupState              system.State
	setupStep               int
	setupFromStartup        bool
	setupGlassActive        bool
	setupHoverButton        = -1
	setupButtonRects        [3]RECT
	setupWheelChoiceRects   [4]RECT
	setupHoverWheelChoice   = -1
	setupModeRects          [2]RECT
	setupOptionRect         RECT
	setupHoverMode          = -1
	setupHoverOption        bool
	setupModeChoice         string
	setupUninstallProfiler  bool
	setupRestoreProfiler    bool
	setupMouseTracked       bool
	setupMigrationRunning   bool
	setupMigrationToken     string
	setupMigrationMu        sync.Mutex
	setupMigrationResult    system.MigrationResult
	setupMigrationLive      system.MigrationResult
	setupShowCommandView    bool
	setupProgressToggleRect RECT
	setupInputCheckStatus   string
)

func friendlyUserName() string {
	name := strings.TrimSpace(os.Getenv("USERNAME"))
	if name == "" {
		return ""
	}
	// Account names are often all-lowercase. Make only the first rune friendly
	// without trying to guess a person's real name from machine/account data.
	r := []rune(name)
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
	}
	return string(r)
}

func maybeOfferFirstRunSetup() bool {
	if smokeTest {
		return false
	}
	p := getUISettings()
	if !p.OfferSetup || p.SetupCompleted {
		return false
	}
	if raw := strings.TrimSpace(p.SetupDeferredUntil); raw != "" {
		if until, err := time.Parse(time.RFC3339, raw); err == nil && time.Now().Before(until) {
			return false
		}
	}
	stateMu.RLock()
	s := appStateSnapshot()
	stateMu.RUnlock()
	return showSetupGuideWindow(s, true)
}

func showSetupAssistant(s system.State) { showSetupGuideWindow(s, false) }

func showSetupAssistantForMode(s system.State, mode string) {
	if !showSetupGuideWindow(s, false) {
		return
	}
	if mode == "legacy" || mode == "modern" {
		setupModeChoice = mode
		// A mode-specific entry point already comes from a detected/selected wheel.
		// Start at the mode page while Back still lets the user review identity.
		setupStep = 1
		invalidate(setupWnd)
	}
}

func showSetupGuideWindow(s system.State, fromStartup bool) bool {
	if setupWnd != 0 {
		setupState = s
		pSetForegroundWindow.Call(uintptr(setupWnd))
		invalidate(setupWnd)
		return true
	}
	setupState = s
	prefs := getUISettings()
	setupStep = prefs.SetupResumeStep
	if setupStep < 0 || setupStep >= setupPageCount {
		setupStep = 0
	}
	setupFromStartup = fromStartup
	setupHoverButton = -1
	setupHoverWheelChoice = -1
	setupHoverMode = -1
	setupHoverOption = false
	setupUninstallProfiler = false
	setupRestoreProfiler = true
	if strings.Contains(strings.ToLower(s.ActiveMode), "legacy") {
		setupModeChoice = "legacy"
	} else {
		setupModeChoice = "modern"
	}
	setupMouseTracked = false
	setupMigrationRunning = false
	setupMigrationToken = ""
	setupShowCommandView = false
	setupProgressToggleRect = RECT{}
	setupInputCheckStatus = "Noch nicht geprüft"
	setupMigrationMu.Lock()
	setupMigrationLive = system.MigrationResult{}
	setupMigrationMu.Unlock()

	hinst, _, _ := pGetModuleHandleW.Call(0)
	className := utf16("LogiMateSetupGuideWindow")
	if !setupClassRegistered {
		cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
		icon := brandIcon(64)
		if icon == 0 {
			fallback, _, _ := pLoadIconW.Call(0, IDI_APPLICATION)
			icon = HICON(fallback)
		}
		wc := WNDCLASSEX{
			CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
			Style:         0x0002 | 0x0001,
			LpfnWndProc:   syscall.NewCallback(setupWndProc),
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
		setupClassRegistered = true
	}

	title := fmt.Sprintf("LogiMate %s – Ersteinrichtung", displayVersion(Version))
	h, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16(title))),
		WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN,
		245, 120, 820, 665,
		uintptr(mainWnd), 0, hinst, 0,
	)
	if h == 0 {
		return false
	}
	setupWnd = HWND(h)
	applyWindowChromeTheme(setupWnd)
	pShowWindow.Call(h, SW_SHOW)
	pUpdateWindow.Call(h)
	setupGlassActive = applyWindowMaterial(setupWnd)
	invalidate(setupWnd)
	return true
}

func setupWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		paintSetupGuide(HWND(hwnd))
		return 0
	case WM_MOUSEMOVE:
		if !setupMouseTracked {
			trackMouseLeave(HWND(hwnd))
			setupMouseTracked = true
		}
		x, y := signedLoWord(lParam), signedHiWord(lParam)
		oldButton, oldChoice, oldMode, oldOption := setupHoverButton, setupHoverWheelChoice, setupHoverMode, setupHoverOption
		setupHoverWheelChoice = setupWheelChoiceHitTest(x, y)
		setupHoverMode = setupModeHitTest(x, y)
		setupHoverOption = setupOptionHitTest(x, y)
		if setupHoverWheelChoice >= 0 || setupHoverMode >= 0 || setupHoverOption {
			setupHoverButton = -1
		} else {
			setupHoverButton = setupButtonHitTest(x, y)
		}
		setCursorHand(setupHoverButton >= 0 || setupHoverWheelChoice >= 0 || setupHoverMode >= 0 || setupHoverOption)
		if oldButton != setupHoverButton || oldChoice != setupHoverWheelChoice || oldMode != setupHoverMode || oldOption != setupHoverOption {
			invalidate(HWND(hwnd))
		}
		return 0
	case WM_MOUSELEAVE:
		setupMouseTracked = false
		setupHoverButton = -1
		setupHoverWheelChoice = -1
		setupHoverMode = -1
		setupHoverOption = false
		setCursorHand(false)
		invalidate(HWND(hwnd))
		return 0
	case WM_LBUTTONUP:
		x, y := signedLoWord(lParam), signedHiWord(lParam)
		if setupMigrationRunning && setupProgressToggleRect.Right > setupProgressToggleRect.Left && x >= setupProgressToggleRect.Left && x <= setupProgressToggleRect.Right && y >= setupProgressToggleRect.Top && y <= setupProgressToggleRect.Bottom {
			setupShowCommandView = !setupShowCommandView
			invalidate(HWND(hwnd))
			return 0
		}
		if idx := setupWheelChoiceHitTest(x, y); idx >= 0 {
			applySetupWheelChoice(idx)
			return 0
		}
		if idx := setupModeHitTest(x, y); idx >= 0 {
			applySetupModeChoice(idx)
			return 0
		}
		if setupOptionHitTest(x, y) {
			if setupModeChoice == "modern" {
				setupUninstallProfiler = !setupUninstallProfiler
			} else {
				setupRestoreProfiler = !setupRestoreProfiler
			}
			invalidate(setupWnd)
			return 0
		}
		if idx := setupButtonHitTest(x, y); idx >= 0 {
			handleSetupButton(idx)
		}
		return 0
	case WM_CLOSE:
		if setupMigrationRunning {
			messageBox(HWND(hwnd), "Die Einrichtung läuft gerade. Bitte warte auf das bestätigte Ergebnis, damit kein Zwischenzustand übersehen wird.", "Einrichtung läuft", MB_OK|MB_ICONINFORMATION)
			return 0
		}
	case WM_KEYDOWN:
		switch wParam {
		case VK_ESCAPE:
			if setupMigrationRunning {
				return 0
			}
			pDestroyWindow.Call(hwnd)
			return 0
		case VK_LEFT:
			if setupStep > 0 {
				setupStep--
				persistSetupProgress(setupStep)
				invalidate(HWND(hwnd))
			}
			return 0
		case VK_RIGHT:
			// Step 4 is the actual migration boundary and may never be skipped by
			// an arrow key. Enter/primary button is required there.
			if setupStep < setupPageCount-1 && setupStep != 3 {
				setupStep++
				persistSetupProgress(setupStep)
				invalidate(HWND(hwnd))
			}
			return 0
		case VK_RETURN:
			buttons := setupButtonsForStep()
			if len(buttons) > 0 {
				handleSetupButton(len(buttons) - 1)
			}
			return 0
		}
	case setupMigrationProgressMsg:
		invalidate(HWND(hwnd))
		return 0
	case WM_TIMER:
		if wParam == setupMigrationAnimTimer && setupMigrationRunning {
			invalidate(HWND(hwnd))
			return 0
		}
	case WM_DWMCOMPOSITIONCHANGED:
		setupGlassActive = applyWindowMaterial(HWND(hwnd))
		invalidate(HWND(hwnd))
		return 0
	case WM_SETTINGCHANGE:
		applyWindowChromeTheme(HWND(hwnd))
		if !safeUI && getUISettings().Acrylic {
			setupGlassActive = applyWindowMaterial(HWND(hwnd))
		}
		invalidate(HWND(hwnd))
		return 0
	case WM_DESTROY:
		pKillTimer.Call(hwnd, setupMigrationAnimTimer)
		wasStartup := setupFromStartup
		setupWnd = 0
		setupGlassActive = false
		setupHoverButton = -1
		setupHoverWheelChoice = -1
		setupHoverMode = -1
		setupHoverOption = false
		setupMouseTracked = false
		setupFromStartup = false
		if wasStartup {
			continueStartupAfterSetup()
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func setupButtonHitTest(x, y int32) int {
	for i, r := range setupButtonRects {
		if r.Right <= r.Left || r.Bottom <= r.Top {
			continue
		}
		if x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom {
			return i
		}
	}
	return -1
}

type setupWheelChoice struct {
	label string
	desc  string
	model string
}

var setupWheelChoices = []setupWheelChoice{
	{label: "Automatisch", desc: "VID/PID + Raw Input entscheiden", model: ""},
	{label: "Driving Force GT", desc: "Nur als C294-Fallback merken", model: system.ModelDFGT},
	{label: "G25", desc: "Nur als C294-Fallback merken", model: system.ModelG25},
	{label: "G27", desc: "Nur als C294-Fallback merken", model: system.ModelG27},
}

func paintSetupWheelChoiceIcon(hdc uintptr, r RECT, idx int, selected bool) {
	accent := colMuted
	if selected || idx == setupHoverWheelChoice {
		accent = colAccent
	}
	cx := (r.Left + r.Right) / 2
	cy := (r.Top + r.Bottom) / 2
	if idx == 0 {
		// Automatic detection: a compact radar/target symbol rather than a
		// model badge. This makes the "automatic" card visually distinct.
		ellipse(hdc, RECT{cx - 14, cy - 14, cx + 14, cy + 14}, colPanel, accent)
		ellipse(hdc, RECT{cx - 6, cy - 6, cx + 6, cy + 6}, colPanel2, accent)
		line(hdc, cx, cy-20, cx, cy-13, accent, 2)
		line(hdc, cx+13, cy-13, cx+19, cy-19, accent, 2)
		line(hdc, cx+18, cy, cx+25, cy, accent, 2)
		return
	}

	// Model choices share a small steering-wheel glyph; the tiny hub badge is
	// only a discriminator and the full accessible model name remains text.
	ellipse(hdc, RECT{cx - 20, cy - 20, cx + 20, cy + 20}, colPanel, accent)
	ellipse(hdc, RECT{cx - 6, cy - 6, cx + 6, cy + 6}, colPanel2, accent)
	line(hdc, cx-5, cy-3, cx-16, cy-13, accent, 2)
	line(hdc, cx+5, cy-3, cx+16, cy-13, accent, 2)
	line(hdc, cx, cy+5, cx, cy+17, accent, 2)
	line(hdc, cx, cy-22, cx, cy-16, colAccent, 3)

	badge := ""
	switch idx {
	case 1:
		badge = "GT"
	case 2:
		badge = "25"
	case 3:
		badge = "27"
	}
	if badge != "" {
		br := RECT{cx - 12, cy + 24, cx + 12, cy + 40}
		pSetTextColor.Call(hdc, accent)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
		drawText(hdc, badge, &br, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
	}
}

func setupWheelChoiceHitTest(x, y int32) int {
	if setupStep != 0 {
		return -1
	}
	for i, r := range setupWheelChoiceRects {
		if r.Right > r.Left && r.Bottom > r.Top && x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom {
			return i
		}
	}
	return -1
}

func selectedSetupWheelChoice() int {
	pref := strings.TrimSpace(setupState.WheelPreference)
	if pref == "" {
		return 0
	}
	for i, c := range setupWheelChoices {
		if c.model == pref {
			return i
		}
	}
	return 0
}

func applySetupWheelChoice(idx int) {
	if idx < 0 || idx >= len(setupWheelChoices) {
		return
	}
	choice := setupWheelChoices[idx]
	selectedID := strings.TrimSpace(setupState.SelectedWheelID)
	if choice.model == "" {
		// Clear persisted state first. Do not update the in-memory wizard until all
		// requested writes succeeded; otherwise the UI can claim "Automatic" while
		// a stale C294 model fallback remains on disk after a write/delete failure.
		if selectedID != "" {
			if err := system.ClearWheelModelConfirmation(setupState.DataDir, selectedID); err != nil {
				messageBox(setupWnd, err.Error(), "Lenkradauswahl", MB_OK|MB_ICONERROR)
				return
			}
			_ = system.ClearWheelDevicePreference(setupState.DataDir, selectedID) // legacy schema cleanup
		}
		if len(setupState.Wheels) <= 1 {
			if err := system.ClearWheelPreference(setupState.DataDir); err != nil {
				messageBox(setupWnd, err.Error(), "Lenkradauswahl", MB_OK|MB_ICONERROR)
				return
			}
		}
		setupState.WheelPreference = ""
		if strings.Contains(setupState.WheelModel, "manuell bestätigt / C294") {
			setupState.WheelModel = system.SummarizeWheelModel(setupState.Devices)
		}
		setActionFeedback("Lenkraderkennung steht wieder auf Automatisch.")
	} else {
		// A manual model choice is only meaningful for the shared C294 identity.
		// Native C299/C29A/C29B already identify G25/DFGT/G27 authoritatively,
		// and a choice without an attached physical target would become a global
		// guess that could later be applied to the wrong wheel.
		if selectedID == "" {
			messageBox(setupWnd, "Wähle zuerst das aktuell angeschlossene C294-Lenkrad aus. Eine Modellbestätigung wird immer an genau dieses physische Gerät gebunden.", "Lenkradmodell bestätigen", MB_OK|MB_ICONWARNING)
			return
		}
		if !system.IsCompatibilityModel(setupState.WheelModel) {
			messageBox(setupWnd, "Dieses Lenkrad ist bereits über seine native USB-ID eindeutig erkannt. Eine manuelle Modellbestätigung ist weder nötig noch sinnvoll.", "Lenkradmodell bereits eindeutig", MB_OK|MB_ICONINFORMATION)
			return
		}
		var selectedWheel system.WheelDevice
		for _, w := range setupState.Wheels {
			if strings.EqualFold(w.ID, selectedID) {
				selectedWheel = w
				break
			}
		}
		if err := system.SaveWheelModelConfirmation(setupState.DataDir, selectedWheel, choice.model); err != nil {
			messageBox(setupWnd, err.Error(), "Lenkradmodell bestätigen", MB_OK|MB_ICONERROR)
			return
		}
		confirmed := choice.model + " (manuell bestätigt / C294)"
		setupState.WheelPreference = choice.model
		setupState.WheelModel = confirmed
		if selectedWheel.PersistentIdentity {
			setupState.DetectionEvidence = "C294 + dauerhafte Nutzerbestätigung (Hardware-Fingerprint)"
		} else {
			setupState.DetectionEvidence = "C294 + Nutzerbestätigung nur für aktuelle Gerätesitzung"
		}
		for i := range setupState.Wheels {
			if strings.EqualFold(setupState.Wheels[i].ID, selectedID) {
				setupState.Wheels[i].Model = confirmed
				setupState.Wheels[i].Evidence = setupState.DetectionEvidence
				setupState.Wheels[i].Supported = true
				setupState.Wheels[i].ModelConfirmed = true
			}
		}
		// Keep the already-rendered main-window snapshot coherent until the
		// asynchronous hardware refresh finishes. This state can only be reached
		// after the per-physical-wheel preference was durably written above. The
		// elevated migration still re-enumerates and validates the real hardware.
		stateMu.Lock()
		if strings.EqualFold(appState.SelectedWheelID, selectedID) {
			appState.WheelPreference = choice.model
			appState.WheelModel = confirmed
			appState.DetectionEvidence = setupState.DetectionEvidence
			for i := range appState.Wheels {
				if strings.EqualFold(appState.Wheels[i].ID, selectedID) {
					appState.Wheels[i].Model = confirmed
					appState.Wheels[i].ModelConfirmed = true
					appState.Wheels[i].Evidence = setupState.DetectionEvidence
					appState.Wheels[i].Supported = true
				}
			}
		}
		stateMu.Unlock()
		setActionFeedback(choice.label + " wird nur als Fallback für dieses C294-Wheel gemerkt.")
	}
	requestRefresh()
	invalidate(setupWnd)
}

func setupModeHitTest(x, y int32) int {
	if setupStep != 1 {
		return -1
	}
	for i, r := range setupModeRects {
		if r.Right > r.Left && r.Bottom > r.Top && x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom {
			return i
		}
	}
	return -1
}

func setupOptionHitTest(x, y int32) bool {
	if setupStep != 1 || setupOptionRect.Right <= setupOptionRect.Left {
		return false
	}
	r := setupOptionRect
	return x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom
}

func applySetupModeChoice(idx int) {
	switch idx {
	case 0:
		setupModeChoice = "modern"
	case 1:
		setupModeChoice = "legacy"
	default:
		return
	}
	invalidate(setupWnd)
}

func setupModeTitle() string {
	if setupModeChoice == "legacy" {
		return "Original Logitech / Legacy"
	}
	return "Modern / LogiMate Native"
}

func setupPlanText() string {
	s := setupState
	if setupModeChoice == "legacy" {
		var base string
		switch {
		case len(s.LegacyDrivers) > 0:
			base = "1. Aktive Logitech-Legacy-Treiber prüfen und – falls noch nicht vorhanden – als Sicherheits-Backup exportieren.\r\n2. Bestehenden Legacy-Treiber unverändert aktiv lassen und Windows-Gerätestatus neu prüfen."
		case s.BackupCount > 0:
			base = "1. Vorhandenes LogiMate-Treiber-Backup per SHA-256 prüfen.\r\n2. Logitech-Legacy-Treiber aus dem verifizierten Backup installieren.\r\n3. Windows-Geräte neu einlesen."
		default:
			base = "1. Offizielle Logitech Gaming Software 5.10.127 direkt von Logitech laden.\r\n2. Authenticode-Signatur auf einen gültigen Logitech-Herausgeber prüfen.\r\n3. Erst danach den offiziellen Installer starten und anschließend die Legacy-Treiber erneut inventarisieren."
		}
		profiler := "\r\n4. Profiler-Anwendung wird nicht zusätzlich wiederhergestellt."
		if setupRestoreProfiler {
			if s.BackupCount == 0 && len(s.LegacyDrivers) == 0 {
				profiler = "\r\n4. Der offizielle Logitech-Installer richtet Profiler/LGS zusammen mit dem Legacy-Stack ein."
			} else {
				profiler = "\r\n4. Profiler-Einstellungen aus dem letzten Backup wiederherstellen; einen gesicherten Installer nur verwenden, wenn er tatsächlich vorhanden und verifiziert ist."
			}
		}
		return base + profiler
	}

	profiler := "Der Logitech Profiler bleibt installiert; LCore wird beim Generic-HID-Wechsel beendet."
	if setupUninstallProfiler {
		profiler = "Profiler-Konfiguration wird gesichert und der Profiler anschließend optional deinstalliert."
	}
	engine := "LogiMates Native Wheel Engine wird vor jeder destruktiven Treiberänderung über den Native-Engine-Sicherheitscheck geprüft. Danach wird das bestätigte G25/G27/Driving Force GT bei Bedarf vom gemeinsamen C294-Modus auf seine modellrichtige native PID vorbereitet. Direct HID, Kalibrierung, Range, FFB, Profile und Game-Adapter laufen in LogiMate; modellabhängige Fähigkeiten wie G27-Rev-LEDs werden über Capabilities begrenzt."
	return "1. " + engine + "\r\n2. Logitech-Profiler-Einstellungen sichern.\r\n3. Vorhandene Logitech-Legacy-Treiber vollständig sichern.\r\n4. " + profiler + "\r\n5. Gebundene Legacy-Treiber sicher entfernen und Windows Generic HID neu einlesen."
}

func executeSetupPlan() {
	stateMu.RLock()
	s := appStateSnapshot()
	stateMu.RUnlock()

	// Fail before UAC. The elevated migration repeats the same validation, but
	// users should never approve an Administrator prompt only to discover that
	// the target wheel was ambiguous, stale or one of several attached wheels.
	if s.DeviceDetectionError != "" {
		messageBox(setupWnd, "Die Windows-Geräteerkennung ist derzeit nicht zuverlässig:\r\n\r\n"+s.DeviceDetectionError+"\r\n\r\nBehebe den Scanfehler und starte die Erkennung erneut.", "Einrichtung gesperrt", MB_OK|MB_ICONERROR)
		return
	}
	if len(s.Wheels) == 0 {
		messageBox(setupWnd, "Es ist kein unterstütztes Logitech-Lenkrad verbunden. Schließe das Wheel an und starte die Erkennung erneut.", "Einrichtung gesperrt", MB_OK|MB_ICONWARNING)
		return
	}
	if len(s.Wheels) > 1 {
		messageBox(setupWnd, "Mehrere Logitech-Lenkräder sind gleichzeitig verbunden. Für einen Treiber- oder Moduswechsel müssen alle anderen Wheels vorübergehend getrennt werden.", "Einrichtung gesperrt", MB_OK|MB_ICONWARNING)
		return
	}
	if !system.HasActionableSelectedWheel(s) {
		if system.IsCompatibilityModel(s.WheelModel) {
			messageBox(setupWnd, "Das C294-Gerät ist erkannt, aber das physische Modell ist noch nicht sicher bestätigt. Gehe zurück zu „Lenkradmodell“ und bestätige G25, G27 oder Driving Force GT nur dann, wenn du das angeschlossene Modell kennst.", "Modellbestätigung erforderlich", MB_OK|MB_ICONWARNING)
		} else {
			messageBox(setupWnd, "Das erkannte Lenkrad ist noch kein eindeutig ausgewähltes, unterstütztes Ziel. Öffne „Geräte verwalten“ und prüfe die Erkennung.", "Einrichtung gesperrt", MB_OK|MB_ICONWARNING)
		}
		return
	}

	plan := setupPlanText()
	if messageBox(setupWnd, "Folgende Schritte werden ausgeführt:\r\n\r\n"+plan+"\r\n\r\nJetzt fortfahren?", setupModeTitle(), MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
		return
	}
	var actionName string
	if setupModeChoice == "legacy" {
		// On a fresh machine there is no LogiMate backup to restore. The only safe
		// fresh Legacy path is Logitech's signed LGS/Profiler installer, which also
		// provisions the old wheel driver stack. Make that dependency explicit here
		// instead of letting the elevated migration fail several steps later.
		if s.BackupCount == 0 && len(s.LegacyDrivers) == 0 && !setupRestoreProfiler {
			messageBox(setupWnd, "Auf diesem PC ist weder ein Legacy-Backup noch ein alter Logitech-Wheel-Treiber vorhanden. Für einen frischen Legacy-Aufbau muss „Profiler / LGS installieren“ aktiviert bleiben.", "Legacy neu einrichten", MB_OK|MB_ICONWARNING)
			return
		}
		actionName = "setup-legacy"
		if setupRestoreProfiler {
			actionName += "-profiler"
		}
		if s.HVCI {
			if messageBox(setupWnd, "Speicherintegrität/HVCI ist aktiv. Alte Logitech-WingMan-Treiber können damit inkompatibel sein. LogiMate ändert diese Windows-Sicherheitsfunktion nicht selbst.\r\n\r\nWindows-Sicherheit jetzt öffnen? Ändere die Einstellung dort nur bewusst selbst und starte Windows anschließend neu, bevor du Legacy erneut versuchst.", "Legacy / Speicherintegrität", MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) == IDYES {
				if err := openMemoryIntegritySettings(); err != nil {
					messageBox(setupWnd, err.Error(), "Windows-Sicherheit", MB_OK|MB_ICONERROR)
				}
			}
			return
		}
	} else {
		actionName = "setup-modern"
		if setupUninstallProfiler {
			actionName += "-uninstall-profiler"
		}
	}
	token, err := system.BeginMigration(s.DataDir, actionName)
	if err != nil {
		messageBox(setupWnd, "Migrationsjournal konnte nicht angelegt werden:\r\n"+err.Error(), "Einrichtung", MB_OK|MB_ICONERROR)
		return
	}
	if !elevateMigration(actionName, s.DataDir, token) {
		return
	}
	setupMigrationRunning = true
	setupMigrationToken = token
	setupShowCommandView = false
	setupMigrationMu.Lock()
	setupMigrationLive = system.MigrationResult{ID: token, Action: actionName, Status: "pending"}
	setupMigrationMu.Unlock()
	if setupWnd != 0 {
		pSetTimer.Call(uintptr(setupWnd), setupMigrationAnimTimer, 120, 0)
	}
	setActionFeedback("Einrichtung läuft …")
	invalidate(setupWnd)
	go waitForSetupMigration(s.DataDir, token)
}

func waitForSetupMigration(dataDir, token string) {
	startedAt := time.Now()
	deadline := startedAt.Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		m, err := system.LoadMigration(dataDir, token)
		if err == nil {
			setupMigrationMu.Lock()
			setupMigrationLive = m
			setupMigrationMu.Unlock()
			if setupWnd != 0 {
				postMessage(setupWnd, setupMigrationProgressMsg, 0, 0)
			}
		}
		if err == nil && m.Completed {
			setupMigrationMu.Lock()
			setupMigrationResult = m
			setupMigrationMu.Unlock()
			postMessage(mainWnd, msgSetupMigrationDone, 0, 0)
			return
		}
		if err == nil && m.Status == "pending" && time.Since(startedAt) > 20*time.Second {
			system.CancelMigration(dataDir, token, "Der Administratorprozess hat die Migration nicht gestartet.")
			m, _ = system.LoadMigration(dataDir, token)
			setupMigrationMu.Lock()
			setupMigrationResult = m
			setupMigrationMu.Unlock()
			postMessage(mainWnd, msgSetupMigrationDone, 0, 0)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	m := system.MigrationResult{ID: token, Status: "failed", Completed: true, Success: false, Error: "Zeitlimit beim Warten auf die Administratoraktion überschritten."}
	setupMigrationMu.Lock()
	setupMigrationResult = m
	setupMigrationMu.Unlock()
	postMessage(mainWnd, msgSetupMigrationDone, 0, 0)
}

func handleSetupMigrationResult() {
	setupMigrationMu.Lock()
	m := setupMigrationResult
	setupMigrationResult = system.MigrationResult{}
	setupMigrationMu.Unlock()
	setupMigrationRunning = false
	setupMigrationToken = ""
	setupShowCommandView = false
	setupProgressToggleRect = RECT{}
	setupMigrationMu.Lock()
	setupMigrationLive = system.MigrationResult{}
	setupMigrationMu.Unlock()
	requestRefresh()
	if m.Success {
		msg := strings.TrimSpace(m.Message)
		if msg == "" {
			msg = "Betriebsart wurde erfolgreich vorbereitet."
		}
		setupStep = 4
		persistSetupProgress(setupStep)
		setupInputCheckStatus = "Betriebsart eingerichtet · jetzt Eingaben prüfen"
		setActionFeedback(msg)
		invalidate(setupWnd)
		return
	}
	errText := strings.TrimSpace(m.Error)
	if errText == "" {
		errText = "Die Einrichtung wurde nicht erfolgreich abgeschlossen."
	}
	messageBox(setupWnd, errText+"\r\n\r\nDer Assistent bleibt geöffnet. Es wurde kein erfolgreicher Abschluss gespeichert.", "Einrichtung fehlgeschlagen", MB_OK|MB_ICONERROR)
	setActionFeedback("Einrichtung fehlgeschlagen – Details im Migrationsjournal.")
	invalidate(setupWnd)
}

func setupButtonsForStep() []string {
	if setupMigrationRunning {
		return []string{"Einrichtung läuft …"}
	}
	switch setupStep {
	case 0:
		if setupFromStartup {
			return []string{"Nicht mehr automatisch", "Weiter"}
		}
		return []string{"Schließen", "Weiter"}
	case 1, 2:
		return []string{"Zurück", "Weiter"}
	case 3:
		return []string{"Zurück", "Später erinnern", "Jetzt einrichten"}
	case 4:
		return []string{"Zurück", "Eingang prüfen", "Weiter"}
	case 5:
		return []string{"Zurück", "Kalibrierung öffnen", "Weiter"}
	case 6:
		return []string{"Zurück", "Force Feedback öffnen", "Weiter"}
	default:
		return []string{"Zurück", "Fertig"}
	}
}

func persistSetupProgress(step int) {
	p := getUISettings()
	if step < 0 {
		step = 0
	}
	if step >= setupPageCount {
		step = setupPageCount - 1
	}
	p.SetupResumeStep = step
	if err := replaceUISettings(p); err != nil {
		logStartup("setup progress persistence failed: %v", err)
	}
}

func deferSetupGuide() bool {
	p := getUISettings()
	p.SetupCompleted = false
	p.OfferSetup = true
	p.SetupResumeStep = setupStep
	p.SetupDeferredUntil = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	if err := replaceUISettings(p); err != nil {
		messageBox(setupWnd, "Die Erinnerung konnte nicht gespeichert werden:\r\n\r\n"+err.Error(), "Ersteinrichtung", MB_OK|MB_ICONERROR)
		return false
	}
	setActionFeedback("Ersteinrichtung pausiert · wird später wieder angeboten.")
	return true
}

func handleSetupButton(idx int) {
	buttons := setupButtonsForStep()
	if idx < 0 || idx >= len(buttons) {
		return
	}
	label := buttons[idx]
	if label == "Einrichtung läuft …" {
		return
	}
	switch label {
	case "Weiter":
		if setupStep < setupPageCount-1 {
			setupStep++
			persistSetupProgress(setupStep)
			invalidate(setupWnd)
		}
	case "Zurück":
		if setupStep > 0 {
			setupStep--
			persistSetupProgress(setupStep)
			invalidate(setupWnd)
		}
	case "Schließen":
		pDestroyWindow.Call(uintptr(setupWnd))
	case "Nicht mehr automatisch":
		p := getUISettings()
		p.OfferSetup = false
		p.SetupCompleted = false
		p.SetupDeferredUntil = ""
		p.SetupResumeStep = setupStep
		if err := replaceUISettings(p); err != nil {
			messageBox(setupWnd, "Die Auswahl konnte nicht dauerhaft gespeichert werden:\r\n\r\n"+err.Error(), "Ersteinrichtung", MB_OK|MB_ICONERROR)
			return
		}
		setActionFeedback("Ersteinrichtung wird nicht mehr automatisch angeboten.")
		pDestroyWindow.Call(uintptr(setupWnd))
	case "Später erinnern":
		if deferSetupGuide() {
			pDestroyWindow.Call(uintptr(setupWnd))
		}
	case "Eingang prüfen":
		j := getLiveJoy()
		if ok, why := learningSampleOK(j); ok {
			setupInputCheckStatus = fmt.Sprintf("✓ Gültiges Sample · %s · %.1f Hz", emptyFallback(j.InputSource, j.Selection), j.ReportRateHz)
		} else {
			setupInputCheckStatus = "! Noch nicht bereit · " + strings.ReplaceAll(why, "\r\n", " ")
		}
		invalidate(setupWnd)
	case "Kalibrierung öffnen":
		persistSetupProgress(5)
		pDestroyWindow.Call(uintptr(setupWnd))
		setPage(pageWheel)
		wheelSubtab = wheelSubtabCalibration
		contentScroll = 0
		invalidate(mainWnd)
	case "Force Feedback öffnen":
		persistSetupProgress(6)
		pDestroyWindow.Call(uintptr(setupWnd))
		setPage(pageWheel)
		wheelSubtab = wheelSubtabFFB
		contentScroll = 0
		invalidate(mainWnd)
	case "Fertig":
		completeSetupGuide()
	case "Jetzt einrichten":
		executeSetupPlan()
	}
}

func completeSetupGuide() {
	p := getUISettings()
	p.SetupCompleted = true
	p.OfferSetup = false
	p.SetupDeferredUntil = ""
	p.SetupResumeStep = 0
	if err := replaceUISettings(p); err != nil {
		messageBox(setupWnd, "Die Ersteinrichtung ist funktional abgeschlossen, aber der Abschlussstatus konnte nicht dauerhaft gespeichert werden. Der Assistent bleibt offen, damit kein falscher 'fertig'-Zustand entsteht:\r\n\r\n"+err.Error(), "Ersteinrichtung", MB_OK|MB_ICONERROR)
		return
	}
	if setupFromStartup {
		logStartup("First-run setup completed")
	}
	if setupWnd != 0 {
		pDestroyWindow.Call(uintptr(setupWnd))
	}
	setActionFeedback("Ersteinrichtung abgeschlossen.")
	invalidate(mainWnd)
}

func setupPageText() (string, string, string) {
	s := setupState
	name := friendlyUserName()
	greeting := "Hallo,"
	if name != "" {
		greeting = "Hallo " + name + ","
	}

	switch setupStep {
	case 0:
		model := emptyFallback(s.WheelModel, "Noch nicht erkannt")
		body := fmt.Sprintf("%s\r\n\r\nLogiMate beginnt nur mit Lesen: Geräteidentität, PnP/HID-Evidenz und aktueller Betriebsmodus werden geprüft. Eine eindeutige native PID gewinnt immer. Die Modellkarten unten sind ausschließlich ein sicherer C294-Fallback.\r\n\r\nErkannt: %s\r\nModus: %s\r\nAuswahl: %s", greeting, model, emptyFallback(s.ActiveMode, "—"), emptyFallback(s.SelectionStatus, "—"))
		return "Willkommen bei LogiMate", "Schritt 1 von 8 · Lenkrad erkennen", body
	case 1:
		body := fmt.Sprintf("Wähle die Betriebsart.\r\n\r\nMODERN / LOGIMATE NATIVE\r\nWindows Generic HID + LogiMate Native. Geräteidentität, Input, Kalibrierung, FFB, Profile und Telemetrie bleiben in LogiMate.\r\n\r\nORIGINAL / LEGACY\r\nWingMan/LGS-Treiber und Profiler bleiben als bewusster Kompatibilitätsweg verfügbar.\r\n\r\nAktueller Profiler: %s", emptyFallback(s.ProfilerSummary, "nicht erkannt"))
		return "Betriebsart wählen", "Schritt 2 von 8 · Modern oder Original", body
	case 2:
		core := "PASS"
		if err := system.NativeEngineCoreGate(); err != nil {
			core = "BLOCKED · " + err.Error()
		}
		wheel := map[bool]string{true: "PASS · eindeutiges Wheel", false: "OFFEN · Wheel noch nicht eindeutig"}[system.HasActionableSelectedWheel(s)]
		backup := fmt.Sprintf("%d verifizierte INF-Backups", s.BackupCount)
		if len(s.LegacyDrivers) > 0 && s.BackupCount == 0 {
			backup = "WARNUNG · Legacy-Treiber vorhanden, Backup fehlt"
		}
		body := fmt.Sprintf("Vor Änderungen prüft LogiMate die Voraussetzungen.\r\n\r\nWheel: %s\r\nNative Core: %s\r\nTreiber-Backup: %s\r\nMemory Integrity: %s\r\nGerätescan: %s\r\n\r\nEin BLOCKED-Zustand wird nicht durch den Assistenten umgangen.", wheel, core, backup, map[bool]string{true: "aktiv", false: "aus"}[s.HVCI], errorStateLabel(s.DeviceDetectionError))
		return "Sicherheitscheck", "Schritt 3 von 8 · Erst prüfen, dann ändern", body
	case 3:
		body := "Ausgewählt: " + setupModeTitle() + "\r\n\r\n" + setupPlanText() + "\r\n\r\nErst „Jetzt einrichten“ startet den erhöhten Migrationspfad. Jeder Schritt wird protokolliert; ein Fehlschlag wird nicht als Erfolg gespeichert."
		return "Einrichtung durchführen", "Schritt 4 von 8 · Backup → Treiber → Re-enumeration", body
	case 4:
		j := getLiveJoy()
		sample := "wartet auf gültiges Sample"
		if j.SampleValid {
			sample = fmt.Sprintf("gültig · %s · %.1f Hz", emptyFallback(j.InputSource, j.Selection), j.ReportRateHz)
		}
		body := fmt.Sprintf("Bewege das Lenkrad, drücke Pedale und einige Tasten. Dieser Schritt schreibt nichts.\r\n\r\nLive-Status: %s\r\nPrüfung: %s\r\n\r\n„Eingang prüfen“ bewertet den zuletzt gültigen Report. Fehlerhafte oder synthetische Samples werden nicht als bestanden behandelt.", sample, setupInputCheckStatus)
		return "Eingaben testen", "Schritt 5 von 8 · Live-Sample und Controls", body
	case 5:
		steer, pedals, buttons, shifter := calibrationStatus(s)
		body := fmt.Sprintf("Kalibrierungen gehören jetzt direkt zum Lenkrad und sind nicht mehr in einem Sammelmenü versteckt.\r\n\r\nLenkung: %s\r\nPedale: %s\r\nTasten: %s\r\nH-Shifter: %s\r\n\r\n„Kalibrierung öffnen“ springt in den neuen festen Lenkrad-Reiter. Der Assistent merkt sich diesen Schritt und kann danach fortgesetzt werden.", steer, pedals, buttons, shifter)
		return "Kalibrieren", "Schritt 6 von 8 · Lenkung, Pedale und Controls", body
	case 6:
		ffb := system.NativeFFBSnapshot()
		hid := system.NativeHIDTransportMetricsSnapshot()
		body := fmt.Sprintf("Force Feedback wird separat und mit niedriger Teststärke geprüft. Kein Test startet automatisch.\r\n\r\nNative Output: %s\r\nHID Backend: %s\r\nLetzter HID-Fehler: %s\r\nFFB Engine: %s\r\n\r\nÖffne das Control Panel für Constant, Spring, Damper, Friction, Autocenter und Emergency Stop.", map[bool]string{true: "freigeschaltet", false: "aus"}[getUISettings().NativeWheelOutput], emptyFallback(hid.LastBackend, "noch nicht geprüft"), emptyFallback(hid.LastError, "kein aktueller Fehler"), map[bool]string{true: "aktiv · " + ffb.Effect, false: "inaktiv"}[ffb.Active])
		return "Force Feedback prüfen", "Schritt 7 von 8 · sicherer Hardwaretest", body
	default:
		body := fmt.Sprintf("Die Grundeinrichtung kann abgeschlossen werden. Offene Hardware-Zertifizierungen bleiben davon unabhängig sichtbar.\r\n\r\nWheel: %s\r\nModus: %s\r\nKalibrierung: im Lenkrad-Reiter jederzeit änderbar\r\nFFB: im Control Panel jederzeit prüfbar\r\nDiagnose: zentrale Probleme, Tests, Protokoll und Export\r\n\r\nLogiMate bleibt %s · %s, bis die Alpha-Baseline gemeinsam freigegeben wird.", emptyFallback(s.WheelModel, "—"), emptyFallback(s.ActiveMode, "—"), displayVersion(Version), buildLabel())
		return "Fertig", "Schritt 8 von 8 · Zusammenfassung", body
	}
}

func paintSetupGuide(hwnd HWND) {
	if setupMigrationRunning {
		paintSetupMigrationProgress(hwnd)
		return
	}
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	var rc RECT
	pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	clear := bgBrush
	if setupGlassActive {
		clear = materialClearBrush()
	}
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(clear))
	pSetBkMode.Call(hdc, TRANSPARENT)

	title, subtitle, body := setupPageText()
	logo := RECT{30, 22, 78, 70}
	drawBrandIcon(hdc, logo)
	header := RECT{92, 26, rc.Right - 190, 72}
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontTitle))
	drawText(hdc, title, &header, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	badge := RECT{rc.Right - 166, 33, rc.Right - 34, 66}
	drawRoundRect(hdc, badge, 16, colSelected, colAccent)
	pSetTextColor.Call(hdc, readableTextColor(colAccent, colSelected))
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	drawText(hdc, fmt.Sprintf("%d / %d", setupStep+1, setupPageCount), &badge, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)

	sub := RECT{94, 71, rc.Right - 36, 103}
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSubtitle))
	drawText(hdc, subtitle, &sub, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	// Eight-step progress rail stays visible on every page.
	progL, progR := int32(30), rc.Right-30
	segGap := int32(6)
	segW := (progR - progL - int32(setupPageCount-1)*segGap) / int32(setupPageCount)
	for i := 0; i < setupPageCount; i++ {
		r := RECT{progL + int32(i)*(segW+segGap), 108, progL + int32(i)*(segW+segGap) + segW, 116}
		c := colTrack
		if i < setupStep {
			c = colGood
		}
		if i == setupStep {
			c = colAccent
		}
		fillRoundRect(hdc, r, 4, c)
	}
	card := RECT{30, 128, rc.Right - 30, rc.Bottom - 94}
	drawRoundRect(hdc, card, 18, colPanel, colBorder)

	// Accent rail makes the guide visually identical to the premium information
	// surfaces while keeping long explanatory text easy to scan.
	rail := RECT{card.Left + 18, card.Top + 20, card.Left + 22, card.Bottom - 20}
	fillRoundRect(hdc, rail, 2, colAccent)
	textBottom := card.Bottom - 24
	if setupStep == 0 {
		textBottom = card.Bottom - 190
	} else if setupStep == 1 {
		textBottom = card.Bottom - 210
	}
	textRect := RECT{card.Left + 40, card.Top + 24, card.Right - 30, textBottom}
	drawFittedParagraph(hdc, body, textRect, colText, fontBody, fontSmall)

	for i := range setupWheelChoiceRects {
		setupWheelChoiceRects[i] = RECT{}
	}
	if setupStep == 0 {
		innerL, innerR := card.Left+34, card.Right-26
		gap := int32(10)
		cw := (innerR - innerL - gap) / 2
		ch := int32(70)
		top := card.Bottom - (ch*2 + gap + 22)
		selected := selectedSetupWheelChoice()
		for i, choice := range setupWheelChoices {
			row, col := int32(i/2), int32(i%2)
			l := innerL + col*(cw+gap)
			t := top + row*(ch+gap)
			r := RECT{l, t, l + cw, t + ch}
			setupWheelChoiceRects[i] = r
			fill, border := colPanel2, colBorder
			if i == selected {
				fill, border = colSelected, colAccent
			}
			if i == setupHoverWheelChoice {
				fill, border = colHover, colAccent
			}
			drawRoundRect(hdc, r, 14, fill, border)
			iconR := RECT{r.Left + 10, r.Top + 7, r.Left + 66, r.Bottom - 7}
			paintSetupWheelChoiceIcon(hdc, iconR, i, i == selected)
			pSetTextColor.Call(hdc, colText)
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
			lr := RECT{r.Left + 72, r.Top + 8, r.Right - 10, r.Top + 31}
			drawText(hdc, choice.label, &lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
			dr := RECT{r.Left + 72, r.Top + 32, r.Right - 10, r.Bottom - 6}
			drawFittedParagraph(hdc, choice.desc, dr, colMuted, fontSmall, fontSmall)
		}
	}

	for i := range setupModeRects {
		setupModeRects[i] = RECT{}
	}
	setupOptionRect = RECT{}
	if setupStep == 1 {
		innerL, innerR := card.Left+34, card.Right-26
		gap := int32(12)
		mw := (innerR - innerL - gap) / 2
		top := card.Bottom - 190
		labels := []struct{ title, desc, icon string }{
			{"Modern / LogiMate Native", "Generic HID · Direct HID · Native FFB · keine externe Wheel-Runtime", "⚡"},
			{"Original / Legacy", "Backup oder offizielles Logitech LGS 5.10.127 · Profiler optional", "↶"},
		}
		for i, item := range labels {
			r := RECT{innerL + int32(i)*(mw+gap), top, innerL + int32(i)*(mw+gap) + mw, top + 92}
			setupModeRects[i] = r
			selected := (i == 0 && setupModeChoice == "modern") || (i == 1 && setupModeChoice == "legacy")
			fill, border := colPanel2, colBorder
			if selected {
				fill, border = colSelected, colAccent
			}
			if i == setupHoverMode {
				fill, border = colHover, colAccent
			}
			drawRoundRect(hdc, r, 14, fill, border)
			chip := RECT{r.Left + 14, r.Top + 16, r.Left + 54, r.Top + 56}
			fillRoundRect(hdc, chip, 12, colInfoChip)
			pSetTextColor.Call(hdc, readableTextColor(colAccent, colInfoChip))
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontSection))
			drawText(hdc, item.icon, &chip, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
			pSelectObject.Call(hdc, old)
			pSetTextColor.Call(hdc, colText)
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
			tr := RECT{r.Left + 66, r.Top + 12, r.Right - 10, r.Top + 36}
			drawText(hdc, item.title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
			dr := RECT{r.Left + 66, r.Top + 39, r.Right - 10, r.Bottom - 9}
			drawFittedParagraph(hdc, item.desc, dr, colMuted, fontSmall, fontSmall)
		}
		setupOptionRect = RECT{innerL, top + 104, innerR, top + 150}
		active := setupRestoreProfiler
		label := "Profiler-Einstellungen beim Legacy-Rollback wiederherstellen"
		if setupModeChoice == "legacy" && setupState.BackupCount == 0 && len(setupState.LegacyDrivers) == 0 {
			label = "Offizielle Logitech Gaming Software / Profiler für Legacy installieren"
		}
		if setupModeChoice == "modern" {
			active = setupUninstallProfiler
			label = "Logitech Gaming Software / Profiler nach dem Backup deinstallieren"
		}
		fill := colPanel2
		if setupHoverOption {
			fill = colHover
		}
		drawRoundRect(hdc, setupOptionRect, 13, fill, colBorder)
		toggle := RECT{setupOptionRect.Left + 14, setupOptionRect.Top + 10, setupOptionRect.Left + 52, setupOptionRect.Bottom - 10}
		fillRoundRect(hdc, toggle, 12, map[bool]uintptr{true: colAccent, false: colToggleOff}[active])
		knobX := toggle.Left + 10
		if active {
			knobX = toggle.Right - 10
		}
		ellipse(hdc, RECT{knobX - 6, (toggle.Top+toggle.Bottom)/2 - 6, knobX + 6, (toggle.Top+toggle.Bottom)/2 + 6}, colToggleKnobOn, colToggleKnobOn)
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
		lr := RECT{setupOptionRect.Left + 64, setupOptionRect.Top, setupOptionRect.Right - 12, setupOptionRect.Bottom}
		drawText(hdc, label, &lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}

	buttons := setupButtonsForStep()
	for i := range setupButtonRects {
		setupButtonRects[i] = RECT{}
	}
	gap := int32(10)
	bw := int32(190)
	if len(buttons) == 3 {
		bw = 205
	}
	total := int32(len(buttons))*bw + int32(len(buttons)-1)*gap
	left := rc.Right - 30 - total
	if left < 30 {
		left = 30
		bw = (rc.Right - 60 - int32(len(buttons)-1)*gap) / int32(len(buttons))
	}
	for i, label := range buttons {
		r := RECT{left + int32(i)*(bw+gap), rc.Bottom - 70, left + int32(i)*(bw+gap) + bw, rc.Bottom - 24}
		setupButtonRects[i] = r
		primary := label == "Weiter" || label == "Jetzt einrichten"
		fill, border, text := colPanel2, colBorder, colText
		if primary {
			fill, border, text = colAccent, colAccent, colOnAccent
		}
		if setupHoverButton == i {
			if primary {
				fill = blendColor(fill, colText, 8)
			} else {
				fill = colHover
			}
			border = blendColor(border, colText, 16)
		}
		drawRoundRect(hdc, r, 14, fill, border)
		pSetTextColor.Call(hdc, text)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
		drawText(hdc, label, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func setupMigrationSnapshot() system.MigrationResult {
	setupMigrationMu.Lock()
	defer setupMigrationMu.Unlock()
	return setupMigrationLive
}

func migrationPhaseEstimate(action string) int {
	a := strings.ToLower(action)
	switch {
	case strings.Contains(a, "modern") && strings.Contains(a, "g27"):
		return 10
	case strings.Contains(a, "modern"):
		return 8
	case strings.Contains(a, "legacy"):
		return 7
	default:
		return 6
	}
}

func migrationProgressStats(m system.MigrationResult) (completed, total int, current string) {
	total = migrationPhaseEstimate(m.Action)
	terminal := map[string]bool{}
	for _, step := range m.Steps {
		name := strings.TrimSpace(step.Name)
		if name == "" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(step.Status))
		if status == "running" {
			current = step.Name
		}
		if status == "done" || status == "warning" || status == "rollback" || status == "failed" {
			terminal[name] = true
			if current == "" {
				current = step.Name
			}
		}
	}
	completed = len(terminal)
	if len(terminal) > total {
		total = len(terminal)
	}
	if m.Completed {
		completed = total
	}
	if current == "" {
		if m.Status == "pending" {
			current = "Administratorprozess wird gestartet …"
		} else {
			current = "Einrichtung wird vorbereitet …"
		}
	}
	return
}

func migrationCommandViewText(m system.MigrationResult) string {
	var b strings.Builder
	if strings.TrimSpace(m.Action) != "" {
		fmt.Fprintf(&b, "> LogiMate.exe --admin-action %s\r\n", m.Action)
	}
	if m.ID != "" {
		fmt.Fprintf(&b, "  Journal: %s\r\n\r\n", m.ID)
	}
	if len(m.Steps) == 0 {
		b.WriteString("Noch keine Administrator-Schritte protokolliert. Warte auf UAC bzw. Prozessstart …")
		return b.String()
	}
	for _, step := range m.Steps {
		ts := step.Timestamp
		if len(ts) >= 19 {
			ts = strings.ReplaceAll(ts[11:19], "T", " ")
		}
		fmt.Fprintf(&b, "[%s] %-8s  %s", ts, strings.ToUpper(step.Status), step.Name)
		if strings.TrimSpace(step.Detail) != "" {
			fmt.Fprintf(&b, "\r\n    %s", step.Detail)
		}
		b.WriteString("\r\n")
	}
	return strings.TrimSpace(b.String())
}

func paintSetupMigrationProgress(hwnd HWND) {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	var rc RECT
	pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	clear := bgBrush
	if setupGlassActive {
		clear = materialClearBrush()
	}
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(clear))
	pSetBkMode.Call(hdc, TRANSPARENT)

	m := setupMigrationSnapshot()
	completed, total, current := migrationProgressStats(m)

	logo := RECT{30, 22, 78, 70}
	drawBrandIcon(hdc, logo)
	header := RECT{92, 26, rc.Right - 190, 72}
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontTitle))
	drawText(hdc, "Einrichtung läuft", &header, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	badge := RECT{rc.Right - 166, 33, rc.Right - 34, 66}
	drawRoundRect(hdc, badge, 16, colSelected, colAccent)
	pSetTextColor.Call(hdc, readableTextColor(colAccent, colSelected))
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	drawText(hdc, fmt.Sprintf("%d / ~%d", completed, total), &badge, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)

	sub := RECT{94, 71, rc.Right - 36, 103}
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSubtitle))
	drawText(hdc, "Live-Fortschritt · Fenster offen lassen, bis LogiMate den Abschluss bestätigt", &sub, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	// Eight-step progress rail stays visible on every page.
	progL, progR := int32(30), rc.Right-30
	segGap := int32(6)
	segW := (progR - progL - int32(setupPageCount-1)*segGap) / int32(setupPageCount)
	for i := 0; i < setupPageCount; i++ {
		r := RECT{progL + int32(i)*(segW+segGap), 108, progL + int32(i)*(segW+segGap) + segW, 116}
		c := colTrack
		if i < setupStep {
			c = colGood
		}
		if i == setupStep {
			c = colAccent
		}
		fillRoundRect(hdc, r, 4, c)
	}
	card := RECT{30, 128, rc.Right - 30, rc.Bottom - 94}
	drawRoundRect(hdc, card, 18, colPanel, colBorder)
	fillRoundRect(hdc, RECT{card.Left + 18, card.Top + 20, card.Left + 22, card.Bottom - 20}, 2, colAccent)

	statusLabel := "Wird vorbereitet"
	if strings.EqualFold(m.Status, "running") {
		statusLabel = "Wird ausgeführt"
	}
	if strings.EqualFold(m.Status, "failed") {
		statusLabel = "Fehler"
	}
	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSection))
	r := RECT{card.Left + 40, card.Top + 26, card.Right - 30, card.Top + 58}
	drawText(hdc, statusLabel, &r, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
	r = RECT{card.Left + 40, card.Top + 61, card.Right - 30, card.Top + 94}
	drawText(hdc, current, &r, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	// Determinate-by-phases base plus an animated activity highlight. The phase
	// count is deliberately approximate because Windows/Logitech installers can
	// add or skip propagation steps at runtime.
	bar := RECT{card.Left + 40, card.Top + 108, card.Right - 40, card.Top + 124}
	fillRoundRect(hdc, bar, 8, colTrack)
	fraction := float64(completed) / float64(maxInt(total, 1))
	if fraction > 1 {
		fraction = 1
	}
	filledRight := bar.Left + int32(float64(bar.Right-bar.Left)*fraction)
	if filledRight > bar.Left {
		fillRoundRect(hdc, RECT{bar.Left, bar.Top, filledRight, bar.Bottom}, 8, colAccent)
	}
	if !m.Completed {
		span := max32(50, (bar.Right-bar.Left)/7)
		travel := max32(1, (bar.Right-bar.Left)-span)
		phase := int32((time.Now().UnixMilli() / 120) % int64(travel))
		x := bar.Left + phase
		fillRoundRect(hdc, RECT{x, bar.Top, x + span, bar.Bottom}, 8, blendColor(colAccent, colText, 18))
	}

	setupProgressToggleRect = RECT{card.Left + 40, card.Top + 146, card.Left + 280, card.Top + 190}
	fill := colPanel2
	if setupShowCommandView {
		fill = colSelected
	}
	drawRoundRect(hdc, setupProgressToggleRect, 13, fill, map[bool]uintptr{true: colAccent, false: colBorder}[setupShowCommandView])
	toggleText := colText
	if setupShowCommandView {
		toggleText = readableTextColor(colAccent, fill)
	}
	pSetTextColor.Call(hdc, toggleText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
	label := "Befehlsansicht anzeigen"
	if setupShowCommandView {
		label = "Befehlsansicht ausblenden"
	}
	drawText(hdc, label, &setupProgressToggleRect, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	if setupShowCommandView {
		logCard := RECT{card.Left + 40, card.Top + 205, card.Right - 40, card.Bottom - 24}
		drawRoundRect(hdc, logCard, 12, colPanel2, colBorder)
		text := migrationCommandViewText(m)
		pSetTextColor.Call(hdc, colMuted)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		lr := RECT{logCard.Left + 14, logCard.Top + 12, logCard.Right - 14, logCard.Bottom - 12}
		drawText(hdc, text, &lr, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	} else {
		steps := m.Steps
		start := 0
		if len(steps) > 5 {
			start = len(steps) - 5
		}
		y := card.Top + 212
		for i := start; i < len(steps); i++ {
			step := steps[i]
			accent := colMuted
			switch strings.ToLower(step.Status) {
			case "done":
				accent = colGood
			case "warning", "rollback":
				accent = colWarning
			case "failed":
				accent = colBad
			case "running":
				accent = colAccent
			}
			fillRoundRect(hdc, RECT{card.Left + 42, y + 7, card.Left + 50, y + 15}, 4, accent)
			pSetTextColor.Call(hdc, colText)
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
			tr := RECT{card.Left + 62, y, card.Right - 40, y + 24}
			drawText(hdc, step.Name, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
			pSetTextColor.Call(hdc, colMuted)
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
			dr := RECT{card.Left + 62, y + 23, card.Right - 40, y + 46}
			drawText(hdc, step.Detail, &dr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
			y += 54
			if y+54 > card.Bottom-20 {
				break
			}
		}
		if len(steps) == 0 {
			r := RECT{card.Left + 40, card.Top + 218, card.Right - 40, card.Bottom - 30}
			drawFittedParagraph(hdc, "Warte auf Windows-UAC und den Administratorprozess …", r, colMuted, fontBody, fontSmall)
		}
	}

	// No close/cancel action while a driver transition is in progress. This is
	// intentionally rendered as a disabled status button rather than looking
	// like the UI has frozen.
	button := RECT{rc.Right - 250, rc.Bottom - 70, rc.Right - 30, rc.Bottom - 24}
	drawRoundRect(hdc, button, 14, blendColor(colPanel2, themeBackground, 40), blendColor(colBorder, themeBackground, 35))
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
	drawText(hdc, "Einrichtung läuft …", &button, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
}
