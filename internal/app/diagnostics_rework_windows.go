//go:build windows

package app

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/thelittlespace/LogiMate/internal/system"
)

const (
	diagnosticsTabOverview = iota
	diagnosticsTabProblems
	diagnosticsTabTests
	diagnosticsTabLog
	diagnosticsTabExport
	diagnosticsTabRelease
	diagnosticsTabCount
)

type diagnosticUIEvent struct {
	At                                    time.Time
	Level, Code, Subsystem, Title, Detail string
}

var (
	diagnosticsTab         = diagnosticsTabOverview
	diagnosticsTabRects    [diagnosticsTabCount]RECT
	diagnosticsActionRects [4]RECT
	diagnosticsHoverKind   string
	diagnosticsHoverIndex  = -1
	diagnosticsEventMu     sync.Mutex
	diagnosticsEvents      []diagnosticUIEvent
)

func recordDiagnosticEvent(level, code, subsystem, title, detail string) {
	diagnosticsEventMu.Lock()
	defer diagnosticsEventMu.Unlock()
	e := diagnosticUIEvent{At: time.Now(), Level: level, Code: code, Subsystem: subsystem, Title: strings.TrimSpace(title), Detail: strings.TrimSpace(detail)}
	if e.Title == "" {
		e.Title = "LogiMate"
	}
	diagnosticsEvents = append(diagnosticsEvents, e)
	if len(diagnosticsEvents) > 200 {
		diagnosticsEvents = append([]diagnosticUIEvent(nil), diagnosticsEvents[len(diagnosticsEvents)-200:]...)
	}
}

func recordDiagnosticNotice(text, title string, flags uint32) {
	level := "info"
	code := "UI-INFO"
	if flags&MB_ICONERROR != 0 {
		level, code = "error", "UI-ERROR"
	} else if flags&MB_ICONWARNING != 0 {
		level, code = "warning", "UI-WARN"
	}
	recordDiagnosticEvent(level, code, "UI", title, text)
}

func diagnosticEventSnapshot() []diagnosticUIEvent {
	diagnosticsEventMu.Lock()
	defer diagnosticsEventMu.Unlock()
	out := make([]diagnosticUIEvent, len(diagnosticsEvents))
	copy(out, diagnosticsEvents)
	return out
}

func activeDiagnosticIssues(s system.State) []diagnosticUIEvent {
	var out []diagnosticUIEvent
	add := func(level, code, sub, title, detail string) {
		if strings.TrimSpace(detail) != "" {
			out = append(out, diagnosticUIEvent{At: time.Now(), Level: level, Code: code, Subsystem: sub, Title: title, Detail: detail})
		}
	}
	add("error", "DEV-001", "Geräteerkennung", "Windows-Geräteerkennung", s.DeviceDetectionError)
	add("error", "DRV-001", "Treiber", "Legacy-Treiberinventur", s.LegacyDriverError)
	add("warning", "LGS-001", "Profiler", "Logitech Profiler", s.ProfilerError)
	add("warning", "SEC-001", "Windows-Sicherheit", "Memory Integrity / HVCI", s.HVCIStatusError)
	add("error", "APP-001", "Runtime", "Letzter Systemfehler", s.LastError)
	add("warning", "RAW-001", "Input", "Raw Input", s.RawInputError)
	j := getLiveJoy()
	add("warning", "IN-001", "Input", "Wheel Input", j.LastInputError)
	if j.Error != j.LastInputError {
		add("warning", "IN-002", "Input", "Inputquelle", j.Error)
	}
	ffb := system.NativeFFBSnapshot()
	add("error", "FFB-001", "Force Feedback", "Native FFB", ffb.LastError)
	no := system.NativeOutputSnapshot()
	add("error", "OUT-001", "Output", "Native Output", no.LastError)
	hid := system.NativeHIDTransportMetricsSnapshot()
	add("error", "HID-001", "HID", "HID Transport", hid.LastError)
	return out
}

func diagnosticsSeverityColor(level string) uintptr {
	switch level {
	case "error":
		return colBad
	case "warning":
		return colWarning
	default:
		return colAccent
	}
}
func diagnosticsSeverityLabel(level string) string {
	switch level {
	case "error":
		return "FEHLER"
	case "warning":
		return "WARNUNG"
	default:
		return "INFO"
	}
}

func paintDiagnosticsTabs(hdc uintptr, content RECT) {
	labels := []string{"Übersicht", "Probleme", "Tests", "Protokoll", "Export", "Release"}
	left := content.Left
	top := content.Top
	gap := int32(7)
	avail := content.Right - content.Left
	w := (avail - int32(len(labels)-1)*gap) / int32(len(labels))
	for i, l := range labels {
		r := RECT{left + int32(i)*(w+gap), top, left + int32(i)*(w+gap) + w, top + 34}
		diagnosticsTabRects[i] = r
		fill, border, text := colPanel2, colBorder, colMuted
		if diagnosticsTab == i {
			fill, border = blendColor(colAccentSoft, colPanel2, 28), colAccent
			text = readableTextColor(colAccent, fill)
		} else if diagnosticsHoverKind == "tab" && diagnosticsHoverIndex == i {
			fill = colHover
		}
		if keyboardFocus == focusDiagnosticsTabBase+i {
			border = colFocusRing
		}
		drawRoundRect(hdc, r, 10, fill, border)
		pSetTextColor.Call(hdc, text)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
		drawText(hdc, l, &r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func diagnosticsButton(hdc uintptr, idx int, r RECT, label string, accent uintptr) {
	diagnosticsActionRects[idx] = r
	focused := keyboardFocus == focusDiagnosticsActionBase+idx
	wheelPanelButton(hdc, r, label, accent, (diagnosticsHoverKind == "action" && diagnosticsHoverIndex == idx) || focused)
}

func paintDiagnosticsHub(hdc uintptr, content RECT, s system.State) {
	for i := range diagnosticsActionRects {
		diagnosticsActionRects[i] = RECT{}
	}
	paintDiagnosticsTabs(hdc, content)
	area := RECT{content.Left, content.Top + 46, content.Right, content.Bottom}
	issues := activeDiagnosticIssues(s)
	events := diagnosticEventSnapshot()
	switch diagnosticsTab {
	case diagnosticsTabOverview:
		paintDiagnosticsOverviewV2(hdc, area, s, issues, events)
	case diagnosticsTabProblems:
		paintDiagnosticsProblemsV2(hdc, area, issues)
	case diagnosticsTabTests:
		paintDiagnosticsTestsV2(hdc, area, s)
	case diagnosticsTabLog:
		paintDiagnosticsLogV2(hdc, area, events)
	case diagnosticsTabExport:
		paintDiagnosticsExportV2(hdc, area, s)
	case diagnosticsTabRelease:
		paintDiagnosticsReleaseV2(hdc, area, s)
	}
	contentScrollMax = 0
}

func paintDiagnosticsOverviewV2(hdc uintptr, r RECT, s system.State, issues, events []diagnosticUIEvent) {
	errs, warns := 0, 0
	for _, x := range issues {
		if x.Level == "error" {
			errs++
		} else if x.Level == "warning" {
			warns++
		}
	}
	gap := int32(12)
	w := (r.Right - r.Left - gap*2) / 3
	metricCard(hdc, RECT{r.Left, r.Top, r.Left + w, r.Top + 126}, "!", "Aktive Probleme", fmt.Sprintf("%d Fehler · %d Warnungen", errs, warns), map[bool]string{true: "Handlungsbedarf", false: "Keine kritischen Fehler"}[errs > 0], map[bool]uintptr{true: colBad, false: colGood}[errs > 0])
	metricCard(hdc, RECT{r.Left + w + gap, r.Top, r.Left + 2*w + gap, r.Top + 126}, "H", "HID Transport", map[bool]string{true: "Fehler", false: "Bereit"}[strings.TrimSpace(system.NativeHIDTransportMetricsSnapshot().LastError) != ""], fmt.Sprintf("Writes %d · Timeouts %d", system.NativeHIDTransportMetricsSnapshot().Writes, system.NativeHIDTransportMetricsSnapshot().Timeouts), colAccent)
	metricCard(hdc, RECT{r.Left + 2*(w+gap), r.Top, r.Right, r.Top + 126}, "≡", "Ereignisse", fmt.Sprintf("%d gespeichert", len(events)), "UI-Warnungen und Fehler der aktuellen Sitzung", rgb(174, 112, 255))
	big := RECT{r.Left, r.Top + 138, r.Right, r.Bottom}
	modernCard(hdc, big, colAccent)
	cardHeader(hdc, big, "?", "Diagnosezentrum", "Alle Meldungen an einem Ort – nicht nur Support-Export", colAccent)
	rows := []infoRow{{"Geräteerkennung", errorStateLabel(s.DeviceDetectionError), map[bool]uintptr{true: colBad, false: colGood}[s.DeviceDetectionError != ""]}, {"Input", map[bool]string{true: "gültiges Sample", false: "wartet / ungültig"}[getLiveJoy().SampleValid], map[bool]uintptr{true: colGood, false: colWarning}[getLiveJoy().SampleValid]}, {"Native FFB", map[bool]string{true: "aktiv", false: "inaktiv"}[system.NativeFFBSnapshot().Active], colAccent}, {"Release Trust", system.InspectReleaseTrust(Version).Status, colAccent}}
	paintInfoRows(hdc, RECT{big.Left + 18, big.Top + 67, big.Right - 18, big.Bottom - 16}, rows)
}

func paintDiagnosticsProblemsV2(hdc uintptr, r RECT, issues []diagnosticUIEvent) {
	card := RECT{r.Left, r.Top, r.Right, r.Bottom}
	modernCard(hdc, card, colBad)
	cardHeader(hdc, card, "!", "Aktive Probleme", fmt.Sprintf("%d aktuelle Meldung(en)", len(issues)), colBad)
	if len(issues) == 0 {
		paintCardParagraph(hdc, card, "Keine aktuellen Fehler aus Geräteerkennung, Input, HID, FFB, Output, Profiler oder Windows-Sicherheit. Historische Meldungen bleiben im Reiter Protokoll erhalten.", 75)
		return
	}
	y := card.Top + 68
	rowH := int32(70)
	rowGap := int32(7)
	available := card.Bottom - 12 - y
	maxRows := int(available / (rowH + rowGap))
	if maxRows < 1 {
		maxRows = 1
	}
	if maxRows > len(issues) {
		maxRows = len(issues)
	}
	for i := 0; i < maxRows; i++ {
		x := issues[i]
		rr := RECT{card.Left + 18, y, card.Right - 18, y + rowH}
		fillRoundRect(hdc, rr, 11, colPanel2)
		c := diagnosticsSeverityColor(x.Level)
		fillRoundRect(hdc, RECT{rr.Left + 10, rr.Top + 10, rr.Left + 16, rr.Bottom - 10}, 3, c)
		pSetTextColor.Call(hdc, readableTextColor(c, colPanel2))
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
		a := RECT{rr.Left + 26, rr.Top + 5, rr.Left + 190, rr.Top + 28}
		drawText(hdc, diagnosticsSeverityLabel(x.Level)+" · "+x.Code, &a, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
		b := RECT{rr.Left + 198, rr.Top + 5, rr.Right - 10, rr.Top + 28}
		drawText(hdc, x.Title, &b, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		d := RECT{rr.Left + 26, rr.Top + 31, rr.Right - 10, rr.Bottom - 6}
		drawFittedParagraph(hdc, x.Detail, d, colMuted, fontSmall, fontSmall)
		y += rowH + rowGap
	}
}

func paintDiagnosticsTestsV2(hdc uintptr, r RECT, s system.State) {
	card := RECT{r.Left, r.Top, r.Right, r.Bottom}
	modernCard(hdc, card, colGood)
	cardHeader(hdc, card, "✓", "Tests & Recovery", "Gezielte Prüfungen statt verstreuter Dialoge", colGood)
	cert := system.HardwareCertificationProgress(s)
	certDetail := "G25/G27/DFGT Evidenzmatrix pro physischem Wheel"
	if cert.Required > 0 {
		certDetail = fmt.Sprintf("%s · %d/%d PASS · %d offen · %d fehlgeschlagen", cert.Model, cert.Passed, cert.Required, cert.Pending, cert.Failed)
	}
	labels := []struct {
		t, d string
		c    uintptr
	}{{"Engine Health", "Invarianten, Recovery und Native-Engine-Selbsttests", colAccent}, {"HID Stress", "Reopen/Write, Timeout, Poison und USB-Yank-Assistent", colWarning}, {"Hardware Certification", certDetail, colGood}, {"Status aktualisieren", "Geräte, Treiber und Laufzeitstatus neu erfassen", rgb(174, 112, 255)}}
	y := card.Top + 72
	rowH := int32(64)
	for i, x := range labels {
		rr := RECT{card.Left + 18, y, card.Right - 18, y + rowH}
		drawRoundRect(hdc, rr, 12, colPanel2, colBorder)
		textRight := rr.Right - 190
		pSetTextColor.Call(hdc, colText)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
		tr := RECT{rr.Left + 14, rr.Top + 5, textRight, rr.Top + 28}
		drawText(hdc, x.t, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		dr := RECT{rr.Left + 14, rr.Top + 29, textRight, rr.Bottom - 5}
		drawFittedParagraph(hdc, x.d, dr, colMuted, fontSmall, fontSmall)
		br := RECT{rr.Right - 170, rr.Top + 14, rr.Right - 12, rr.Bottom - 14}
		diagnosticsButton(hdc, i, br, "Starten", x.c)
		y += rowH + 8
	}
}

func paintDiagnosticsLogV2(hdc uintptr, r RECT, events []diagnosticUIEvent) {
	card := RECT{r.Left, r.Top, r.Right, r.Bottom}
	modernCard(hdc, card, rgb(174, 112, 255))
	cardHeader(hdc, card, "≡", "Sitzungsprotokoll", fmt.Sprintf("%d UI-Ereignis(se) · max. 200", len(events)), rgb(174, 112, 255))
	y := card.Top + 68
	rowH := int32(62)
	rowGap := int32(7)
	listBottom := card.Bottom - 56
	maxRows := int((listBottom - y) / (rowH + rowGap))
	if maxRows < 0 {
		maxRows = 0
	}
	start := len(events) - maxRows
	if start < 0 {
		start = 0
	}
	for i := len(events) - 1; i >= start; i-- {
		e := events[i]
		rr := RECT{card.Left + 18, y, card.Right - 18, y + rowH}
		fillRoundRect(hdc, rr, 10, colPanel2)
		severity := diagnosticsSeverityColor(e.Level)
		pSetTextColor.Call(hdc, readableTextColor(severity, colPanel2))
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
		a := RECT{rr.Left + 12, rr.Top + 4, rr.Left + 178, rr.Top + 24}
		drawText(hdc, e.At.Format("15:04:05")+" · "+e.Code, &a, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
		b := RECT{rr.Left + 186, rr.Top + 4, rr.Right - 10, rr.Top + 24}
		drawText(hdc, e.Title, &b, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		d := RECT{rr.Left + 12, rr.Top + 27, rr.Right - 10, rr.Bottom - 5}
		drawFittedParagraph(hdc, e.Detail, d, colMuted, fontSmall, fontSmall)
		y += rowH + rowGap
	}
	br := RECT{card.Right - 185, card.Bottom - 44, card.Right - 18, card.Bottom - 12}
	diagnosticsButton(hdc, 0, br, "Protokoll leeren", colWarning)
}

func paintDiagnosticsExportV2(hdc uintptr, r RECT, s system.State) {
	card := RECT{r.Left, r.Top, r.Right, r.Bottom}
	modernCard(hdc, card, colAccent)
	cardHeader(hdc, card, "↓", "Export & Support", "Daten bleiben lokal, bis du sie selbst weitergibst", colAccent)
	paintInfoRows(hdc, RECT{card.Left + 18, card.Top + 68, card.Right - 18, card.Top + 230}, []infoRow{{"Diagnose-ZIP", "System-, Wheel-, Treiber- und Laufzeitstatus für Support", colAccent}, {"Bericht kopieren", "Textversion für GitHub oder Chat", colGood}, {"Diagnoseordner", s.DataDir + "\\Diagnostics", rgb(174, 112, 255)}})
	labels := []string{"Diagnose-ZIP erstellen", "Bericht kopieren", "Ordner öffnen"}
	for i, l := range labels {
		w := (card.Right - card.Left - 52) / 3
		br := RECT{card.Left + 18 + int32(i)*(w+8), card.Bottom - 48, card.Left + 18 + int32(i)*(w+8) + w, card.Bottom - 12}
		diagnosticsButton(hdc, i, br, l, colAccent)
	}
}
func paintDiagnosticsReleaseV2(hdc uintptr, r RECT, s system.State) {
	st := system.InspectReleaseTrust(Version)
	card := RECT{r.Left, r.Top, r.Right, r.Bottom}
	accent := colAccent
	if st.StableRequired && !st.Valid {
		accent = colBad
	}
	modernCard(hdc, card, accent)
	cardHeader(hdc, card, "◆", "Release Trust", displayVersion(Version)+" · "+buildLabel(), accent)
	paintInfoRows(hdc, RECT{card.Left + 18, card.Top + 68, card.Right - 18, card.Bottom - 65}, []infoRow{{"Authenticode", emptyFallback(st.Status, "Unknown"), map[bool]uintptr{true: colGood, false: colWarning}[st.Valid]}, {"Publisher", emptyFallback(st.Subject, "Pre-release / nicht signiert"), colAccent}, {"Thumbprint", emptyFallback(st.Thumbprint, "—"), colMuted}, {"Stable Gate", map[bool]string{true: "Signatur Pflicht", false: "Pre-release · Signing optional"}[st.StableRequired], accent}})
	br := RECT{card.Right - 210, card.Bottom - 48, card.Right - 18, card.Bottom - 12}
	diagnosticsButton(hdc, 0, br, "Release Trust prüfen", accent)
}

func diagnosticsActivateFocus(s system.State, focusID int) bool {
	if currentPage != pageDiagnostics {
		return false
	}
	if focusID >= focusDiagnosticsTabBase && focusID < focusDiagnosticsTabBase+diagnosticsTabCount {
		i := focusID - focusDiagnosticsTabBase
		diagnosticsTab = i
		contentScroll = 0
		invalidate(mainWnd)
		return true
	}
	if focusID >= focusDiagnosticsActionBase && focusID < focusDiagnosticsActionBase+len(diagnosticsActionRects) {
		i := focusID - focusDiagnosticsActionBase
		r := diagnosticsActionRects[i]
		if r.Right <= r.Left || r.Bottom <= r.Top {
			return true
		}
		return diagnosticsHandleClick((r.Left+r.Right)/2, (r.Top+r.Bottom)/2, s)
	}
	return false
}

func diagnosticsHitTest(x, y int32) int {
	if currentPage != pageDiagnostics {
		return 0
	}
	for i, r := range diagnosticsTabRects {
		if pointIn(r, x, y) {
			return 100 + i
		}
	}
	for i, r := range diagnosticsActionRects {
		if pointIn(r, x, y) {
			return 200 + i
		}
	}
	return 0
}
func diagnosticsHoverChanged(x, y int32) bool {
	ok, oi := diagnosticsHoverKind, diagnosticsHoverIndex
	diagnosticsHoverKind = ""
	diagnosticsHoverIndex = -1
	code := diagnosticsHitTest(x, y)
	if code >= 100 && code < 100+diagnosticsTabCount {
		diagnosticsHoverKind = "tab"
		diagnosticsHoverIndex = code - 100
	} else if code >= 200 {
		diagnosticsHoverKind = "action"
		diagnosticsHoverIndex = code - 200
	}
	return ok != diagnosticsHoverKind || oi != diagnosticsHoverIndex
}
func diagnosticsHandleClick(x, y int32, s system.State) bool {
	code := diagnosticsHitTest(x, y)
	if code == 0 {
		return false
	}
	if code >= 100 && code < 100+diagnosticsTabCount {
		diagnosticsTab = code - 100
		contentScroll = 0
		invalidate(mainWnd)
		return true
	}
	idx := code - 200
	switch diagnosticsTab {
	case diagnosticsTabTests:
		switch idx {
		case 0:
			showEngineHealth(s)
		case 1:
			showHIDStressAssistant(s)
		case 2:
			showHardwareCertificationAssistant(s)
		case 3:
			manualRefreshPending = true
			refreshAsync()
		}
	case diagnosticsTabLog:
		if idx == 0 {
			diagnosticsEventMu.Lock()
			diagnosticsEvents = nil
			diagnosticsEventMu.Unlock()
		}
	case diagnosticsTabExport:
		switch idx {
		case 0:
			exportDiag(s)
		case 1:
			if setClipboardText(mainWnd, diagnosticReportWithAccessibility(s)) {
				setActionFeedback("Diagnosebericht kopiert.")
			}
		case 2:
			_ = system.OpenFolder(s.DataDir + "\\Diagnostics")
		}
	case diagnosticsTabRelease:
		if idx == 0 {
			showReleaseTrust()
		}
	}
	invalidate(mainWnd)
	return true
}
