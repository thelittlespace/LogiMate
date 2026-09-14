//go:build windows

package app

import (
	"fmt"
	"strings"

	"github.com/thelittlespace/LogiMate/internal/system"
)

// The modern dashboard layer intentionally keeps page-specific composition out
// of the Win32 message pump. Each page owns its information hierarchy, while
// common cards use the helpers below. This mirrors the profile-hub visual
// language across the complete application instead of pouring every page into
// one generic text box.

var (
	aboutIndicanaRect RECT
	aboutPayPalRect   RECT
	aboutGitHubRect   RECT
	projectPanelOpen  bool
	projectPanelClose RECT
	projectPanelRect  RECT
	projectPanelRects [5]RECT
)

func modernCard(hdc uintptr, r RECT, accent uintptr) {
	fill := blendColor(colPanel, themeBackground, 10)
	border := colBorder
	if accent != 0 {
		border = blendColor(accent, colBorder, 58)
	}
	drawRoundRect(hdc, r, 18, fill, border)
	if accent != 0 {
		// Paint after the card surface; the old order painted the card over its
		// own accent stripe, so the visual hierarchy disappeared.
		fillRoundRect(hdc, RECT{r.Left + 1, r.Top + 15, r.Left + 4, r.Bottom - 15}, 2, accent)
	}
}

func cardHeader(hdc uintptr, r RECT, icon, title, subtitle string, accent uintptr) {
	iconR := RECT{r.Left + 16, r.Top + 14, r.Left + 52, r.Top + 50}
	iconFill := blendColor(accent, colPanel2, 72)
	fillRoundRect(hdc, iconR, 11, iconFill)
	pSetTextColor.Call(hdc, readableTextColor(accent, iconFill))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSection))
	drawText(hdc, icon, &iconR, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)

	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSection))
	titleR := RECT{iconR.Right + 12, r.Top + 11, r.Right - 14, r.Top + 36}
	drawText(hdc, title, &titleR, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	if strings.TrimSpace(subtitle) != "" {
		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		subR := RECT{iconR.Right + 12, r.Top + 34, r.Right - 14, r.Top + 55}
		drawText(hdc, subtitle, &subR, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func metricCard(hdc uintptr, r RECT, icon, label, value, detail string, accent uintptr) {
	modernCard(hdc, r, accent)
	iconR := RECT{r.Left + 14, r.Top + 15, r.Left + 48, r.Top + 49}
	iconFill := blendColor(accent, colPanel2, 68)
	fillRoundRect(hdc, iconR, 10, iconFill)
	pSetTextColor.Call(hdc, readableTextColor(accent, iconFill))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontBody))
	drawText(hdc, icon, &iconR, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)

	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	lr := RECT{iconR.Right + 10, r.Top + 12, r.Right - 10, r.Top + 31}
	drawText(hdc, label, &lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBrand))
	vr := RECT{r.Left + 14, r.Top + 52, r.Right - 12, r.Top + 82}
	drawText(hdc, value, &vr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	if detail != "" {
		pSetTextColor.Call(hdc, colMuted)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		dr := RECT{r.Left + 14, r.Top + 84, r.Right - 12, r.Bottom - 10}
		drawText(hdc, detail, &dr, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}
}

func modernPageSurface(hdc uintptr, content RECT, s system.State) {
	switch currentPage {
	case pageOverview:
		paintOverviewHub(hdc, content, s)
	case pageSystem:
		paintSystemHub(hdc, content, s)
	case pageDiagnostics:
		paintDiagnosticsHub(hdc, content, s)
	case pageAbout:
		paintAboutHub(hdc, content, s)
	default:
		paintRichBody(hdc, content, getBody())
	}
}

func paintOverviewHub(hdc uintptr, content RECT, s system.State) {
	left, right := content.Left, content.Right
	top := content.Top
	gap := int32(12)
	width := right - left

	// Hero / recommendation card.
	hero := RECT{left, top, right, top + 150}
	modernCard(hdc, hero, colAccent)
	chip := RECT{hero.Left + 20, hero.Top + 20, hero.Left + 68, hero.Top + 68}
	heroChipFill := blendColor(colAccent, colPanel2, 68)
	fillRoundRect(hdc, chip, 15, heroChipFill)
	pSetTextColor.Call(hdc, readableTextColor(colAccent, heroChipFill))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontMetric))
	drawText(hdc, "✓", &chip, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)

	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	kicker := RECT{chip.Right + 16, hero.Top + 18, hero.Right - 24, hero.Top + 40}
	drawText(hdc, "DEIN NÄCHSTER SINNVOLLER SCHRITT", &kicker, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBrand))
	tr := RECT{chip.Right + 16, hero.Top + 42, hero.Right - 24, hero.Top + 72}
	drawText(hdc, overviewPrimaryAction(s), &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	rr := RECT{chip.Right + 16, hero.Top + 76, hero.Right - 30, hero.Bottom - 18}
	drawFittedParagraph(hdc, setupRecommendation(s), rr, colMuted, fontBody, fontSmall)

	// Four state cards.
	cardsTop := hero.Bottom + gap
	cardW := (width - gap*3) / 4
	backupValue, backupColor := backupStatus(s)
	model := compactWheelName(s.WheelModel)
	if model == "" {
		model = "Nicht erkannt"
	}
	mode := compactMode(s.ActiveMode)
	if mode == "" {
		mode = "Unbekannt"
	}
	metrics := []struct {
		icon, label, value, detail string
		accent                     uintptr
	}{
		{"◉", "Lenkrad", model, s.SelectionStatus, statusColor(system.HasActionableSelectedWheel(s))},
		{"↔", "Betriebsmodus", mode, detectionEvidenceLabel(s.DetectionEvidence), colAccent},
		{"◆", "Speicherschutz", map[bool]string{true: "Aktiv", false: "Aus"}[s.HVCI], "Windows Memory Integrity / HVCI", hvciStatusColor(s)},
		{"▣", "Treiber-Backup", backupValue, "Sicherer Rückweg vor Änderungen", backupColor},
	}
	for i, m := range metrics {
		l := left + int32(i)*(cardW+gap)
		metricCard(hdc, RECT{l, cardsTop, l + cardW, cardsTop + 126}, m.icon, m.label, m.value, m.detail, m.accent)
	}

	lowerTop := cardsTop + 126 + gap
	colW := (width - gap) / 2
	quick := RECT{left, lowerTop, left + colW, content.Bottom}
	health := RECT{quick.Right + gap, lowerTop, right, content.Bottom}
	modernCard(hdc, quick, colAccent)
	modernCard(hdc, health, colGood)
	cardHeader(hdc, quick, "⚡", "Schnellzugriff", "Häufige Aufgaben ohne Umwege", colAccent)
	cardHeader(hdc, health, "✓", "Systemzustand", "Was LogiMate gerade sicher weiß", colGood)

	paintInfoRows(hdc, RECT{quick.Left + 18, quick.Top + 64, quick.Right - 18, quick.Bottom - 14}, []infoRow{
		{"Geräte verwalten", "Lenkrad auswählen, neu erkennen oder Auswahl zurücksetzen", colAccent},
		{"Hardware testen", "Lenkung, Pedale, Tasten und H-Schaltung live prüfen", colGood},
		{"Treiber & Modus", "Backups, Generic HID und Legacy sauber verwalten", colWarning},
	})
	det := "Erkennung ohne Fehler"
	detColor := colGood
	if s.DeviceDetectionError != "" {
		det, detColor = "Erkennung braucht Aufmerksamkeit", colBad
	}
	driver := "Treiberinventur verfügbar"
	driverColor := colGood
	if s.LegacyDriverError != "" {
		driver, driverColor = "Treiberinventur erst mit Admin sicher", colWarning
	}
	paintInfoRows(hdc, RECT{health.Left + 18, health.Top + 64, health.Right - 18, health.Bottom - 14}, []infoRow{
		{det, "PnP/SetupAPI und Geräteauswahl", detColor},
		{driver, "Vor Änderungen wird erhöht erneut strikt geprüft", driverColor},
		{map[bool]string{true: "Acrylic aktiv", false: "Opaker Fallback"}[glassActive], rendererStatus, colAccent},
	})
	contentScrollMax = 0
}

type infoRow struct {
	title, detail string
	accent        uintptr
}

func paintInfoRows(hdc uintptr, r RECT, rows []infoRow) {
	if len(rows) == 0 || r.Right <= r.Left || r.Bottom <= r.Top {
		return
	}
	gap := int32(8)
	available := r.Bottom - r.Top
	h := (available - gap*int32(len(rows)-1)) / int32(len(rows))
	compact := h < 42
	if compact {
		// Compact key/value mode prevents the old two-line row layout from
		// overlapping itself in small cards (Profile/Release/identity summaries).
		gap = 3
		h = (available - gap*int32(len(rows)-1)) / int32(len(rows))
		if h < 16 {
			h = 16
		}
	}
	for i, row := range rows {
		y := r.Top + int32(i)*(h+gap)
		rr := RECT{r.Left, y, r.Right, y + h}
		if rr.Top >= r.Bottom {
			break
		}
		if rr.Bottom > r.Bottom {
			rr.Bottom = r.Bottom
		}
		fillRoundRect(hdc, rr, 10, colPanel2)
		dotTop := rr.Top + max32(3, (rr.Bottom-rr.Top-8)/2)
		dot := RECT{rr.Left + 12, dotTop, rr.Left + 20, dotTop + 8}
		fillRoundRect(hdc, dot, 4, row.accent)
		if compact {
			split := rr.Left + (rr.Right-rr.Left)*46/100
			pSetTextColor.Call(hdc, colText)
			old, _, _ := pSelectObject.Call(hdc, uintptr(fontSmall))
			tr := RECT{rr.Left + 29, rr.Top, split - 8, rr.Bottom}
			drawText(hdc, row.title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
			pSetTextColor.Call(hdc, readableTextColor(row.accent, colPanel2))
			old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
			dr := RECT{split, rr.Top, rr.Right - 9, rr.Bottom}
			drawText(hdc, row.detail, &dr, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			pSelectObject.Call(hdc, old)
			continue
		}
		pSetTextColor.Call(hdc, colText)
		old, _, _ := pSelectObject.Call(hdc, uintptr(fontLabel))
		tr := RECT{rr.Left + 30, rr.Top + 5, rr.Right - 10, rr.Top + 27}
		drawText(hdc, row.title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		dr := RECT{rr.Left + 30, rr.Top + 27, rr.Right - 10, rr.Bottom - 5}
		drawFittedParagraph(hdc, row.detail, dr, colMuted2, fontSmall, fontSmall)
	}
}

func paintSystemHub(hdc uintptr, content RECT, s system.State) {
	left, right, top := content.Left, content.Right, content.Top
	gap := int32(12)
	w := right - left
	half := (w - gap) / 2
	cards := []RECT{
		{left, top, left + half, top + 160},
		{left + half + gap, top, right, top + 160},
		{left, top + 172, left + half, top + 332},
		{left + half + gap, top + 172, right, top + 332},
	}
	titles := []struct {
		icon, title, sub string
		accent           uintptr
	}{
		{"▣", "Treiber & Backup", "Erst sichern, dann verändern", colAccent},
		{"↔", "Betriebsmodus", "Legacy und modernes Generic HID", colGood},
		{"◆", "Windows-Sicherheit", "HVCI / Memory Integrity", colWarning},
		{"⚙", "Wheel Engine", "LogiMate Native Engine", colAccent},
	}
	for i, c := range cards {
		modernCard(hdc, c, titles[i].accent)
		cardHeader(hdc, c, titles[i].icon, titles[i].title, titles[i].sub, titles[i].accent)
	}

	paintCardParagraph(hdc, cards[0], fmt.Sprintf("Legacy-Pakete: %d\nVerifizierte Backups: %d\nInventur: %s", len(s.LegacyDrivers), s.BackupCount, errorStateLabel(s.LegacyDriverError)), 67)
	paintCardParagraph(hdc, cards[1], fmt.Sprintf("Aktiv: %s\nGewünscht: %s\nAuswahl: %s", compactMode(s.ActiveMode), emptyFallback(s.OperatingPreference, "Automatisch"), emptyFallback(s.SelectionStatus, "Noch offen")), 67)
	paintCardParagraph(hdc, cards[2], fmt.Sprintf("Speicherschutz: %s\nLegacy-Hinweis: %s", map[bool]string{true: "Aktiv", false: "Aus"}[s.HVCI], hvciExplain(s)), 67)
	engine := "READY"
	if err := system.NativeEngineCoreGate(); err != nil {
		engine = "BLOCKED"
	}
	snap := system.NativeWheelEngineSnapshot()
	modelReady := system.HasActionableSelectedWheel(s)
	paintCardParagraph(hdc, cards[3], fmt.Sprintf("Engine: %s\nZielmodell bereit: %s\nGeneration: %d", engine, yesNo(modelReady), snap.Generation), 67)

	note := RECT{left, top + 344, right, content.Bottom}
	modernCard(hdc, note, colWarning)
	cardHeader(hdc, note, "!", "Sicherer Änderungsweg", "LogiMate blockiert riskante Abkürzungen", colWarning)
	paintCardParagraph(hdc, note, systemActionExplanation(s), 67)
	contentScrollMax = 0
}

func paintDiagnosticsHubLegacy(hdc uintptr, content RECT, s system.State) {
	left, right, top := content.Left, content.Right, content.Top
	gap := int32(12)
	w := right - left
	third := (w - gap*2) / 3
	rendererColor := colGood
	if strings.Contains(strings.ToLower(rendererStatus), "recovery") {
		rendererColor = colWarning
	}
	items := []struct {
		icon, label, value, detail string
		accent                     uintptr
	}{
		{"◉", "Geräteerkennung", errorStateLabel(s.DeviceDetectionError), fmt.Sprintf("%d Logitech-Wheel(s)", len(s.Wheels)), map[bool]uintptr{true: colBad, false: colGood}[s.DeviceDetectionError != ""]},
		{"▣", "Renderer", rendererStatus, rendererDetail, rendererColor},
		{"◆", "Windows", fmt.Sprintf("Build %d", windowsBuildNumber()), fmt.Sprintf("DPI %d · %d Monitor(e)", currentDPI, monitorCount()), colAccent},
	}
	for i, it := range items {
		l := left + int32(i)*(third+gap)
		metricCard(hdc, RECT{l, top, l + third, top + 135}, it.icon, it.label, it.value, it.detail, it.accent)
	}
	big := RECT{left, top + 147, right, content.Bottom}
	modernCard(hdc, big, colAccent)
	cardHeader(hdc, big, "?", "Support-Diagnose", "Datenschutzfreundlich exportieren oder kopieren", colAccent)
	lines := []infoRow{
		{"Diagnose-ZIP", "Sammelt relevante LogiMate-Informationen ohne das komplette Benutzerprofil preiszugeben", colAccent},
		{"Bericht kopieren", "Ideal für GitHub-Issues oder Support ohne zusätzliche Datei", colGood},
		{"Safe UI", "Bei Grafikproblemen mit --safe-ui starten; Acrylic wird dann bewusst deaktiviert", colWarning},
	}
	paintInfoRows(hdc, RECT{big.Left + 18, big.Top + 66, big.Right - 18, big.Bottom - 16}, lines)
	contentScrollMax = 0
}

func paintCardParagraph(hdc uintptr, r RECT, text string, topOffset int32) {
	rr := RECT{r.Left + 18, r.Top + topOffset, r.Right - 18, r.Bottom - 14}
	drawFittedParagraph(hdc, text, rr, colMuted, fontBody, fontSmall)
}

func errorStateLabel(err string) string {
	if strings.TrimSpace(err) != "" {
		return "Fehler"
	}
	return "OK"
}
func emptyFallback(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
func hvciExplain(s system.State) string {
	if s.HVCI && s.ActiveMode == "Logitech Legacy" {
		return "Kann alte Logitech-Treiber blockieren"
	}
	if s.HVCI {
		return "Passt zum modernen Pfad"
	}
	return "Bei Legacy ggf. erforderlich"
}
func systemActionExplanation(s system.State) string {
	if !system.HasActionableSelectedWheel(s) {
		return "Wähle zuerst unter Lenkrad → Geräte verwalten ein eindeutiges Ziel. Ohne eindeutige Hardware bleibt jeder Treiberwechsel gesperrt."
	}
	if len(s.Wheels) > 1 {
		return "Mehrere unterstützte Räder sind gleichzeitig verbunden. Live-Diagnose funktioniert, aber Treiberwechsel bleiben gesperrt, weil Driver-Store-Pakete nicht sicher nur einem einzelnen Rad zugeordnet werden können."
	}
	if s.LegacyDriverError != "" {
		return "Die Treiberinventur konnte nicht zuverlässig gelesen werden. LogiMate interpretiert diesen Fehler niemals als leeren Driver Store und erlaubt daher keine destruktive Aktion."
	}
	if len(s.LegacyDrivers) > 0 && s.BackupCount == 0 {
		return "Vor dem Wechsel auf Generic HID zuerst ein vollständiges, SHA-256-verifiziertes Treiber-Backup erstellen."
	}
	return "Das Zielgerät ist eindeutig und der Sicherheitsstatus ist plausibel. Mode-Wechsel bleiben trotzdem bestätigungspflichtig und laufen über den erhöhten, protokollierten Migrationspfad."
}

func paintAboutHubLegacy(hdc uintptr, content RECT, s system.State) {
	// Profile page follows the approved personal-hub reference: profile + quick
	// facts, biography + current work, then projects / favorite areas / support.
	left, right := content.Left, content.Right
	visibleTop, visibleBottom := content.Top, content.Bottom
	visibleH := visibleBottom - visibleTop
	virtualH := int32(820)
	contentScrollMax = virtualH - visibleH
	if contentScrollMax < 0 {
		contentScrollMax = 0
	}
	if contentScroll > contentScrollMax {
		contentScroll = contentScrollMax
	}
	y := content.Top - contentScroll
	gap := int32(12)
	width := right - left

	saved, _, _ := pSaveDC.Call(hdc)
	pIntersectClipRect.Call(hdc, uintptr(left), uintptr(visibleTop), uintptr(right), uintptr(visibleBottom))

	// Row 1: personal hero + quick facts.
	factsW := width * 31 / 100
	hero := RECT{left, y, right - factsW - gap, y + 220}
	facts := RECT{hero.Right + gap, y, right, y + 220}
	modernCard(hdc, hero, colAccent)
	modernCard(hdc, facts, rgb(115, 177, 255))

	avatar := RECT{hero.Left + 24, hero.Top + 28, hero.Left + 174, hero.Top + 178}
	ellipse(hdc, avatar, blendColor(colAccentSoft, colPanel2, 24), blendColor(colAccent, colBorder, 12))
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontMetric))
	drawText(hdc, "MK", &avatar, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)
	online := RECT{avatar.Right - 18, avatar.Bottom - 20, avatar.Right - 5, avatar.Bottom - 7}
	fillRoundRect(hdc, online, 7, colGood)

	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	kr := RECT{avatar.Right + 24, hero.Top + 27, hero.Right - 22, hero.Top + 49}
	drawText(hdc, "HALLO, ICH BIN", &kr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontMetric))
	nr := RECT{avatar.Right + 24, hero.Top + 49, hero.Right - 22, hero.Top + 88}
	drawText(hdc, "Markus Kleine", &nr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, readableAccentOnPanel2(colAccent))
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSection))
	role := RECT{avatar.Right + 24, hero.Top + 88, hero.Right - 22, hero.Top + 114}
	drawText(hdc, "Entwickler · Tüftler · Zukunftsdenker", &role, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	bio := RECT{avatar.Right + 24, hero.Top + 120, hero.Right - 24, hero.Bottom - 24}
	drawFittedParagraph(hdc, "Ich entwickle digitale Lösungen, die ältere Technik, Windows und den Alltag ein Stück verständlicher und besser machen. LogiMate ist mein größtes aktuelles Projekt – mit dem Ziel, klassische Logitech-Wheels modern, sicher und langfristig nutzbar zu halten.", bio, colMuted, fontBody, fontSmall)

	cardHeader(hdc, facts, "○", "Auf einen Blick", "Der Mensch hinter LogiMate", rgb(115, 177, 255))
	paintInfoRows(hdc, RECT{facts.Left + 16, facts.Top + 65, facts.Right - 16, facts.Bottom - 14}, []infoRow{
		{"Name", "Markus Kleine", colAccent},
		{"Fokus", "Technologie · Nachhaltigkeit · Freiheit", colGood},
		{"Status", "Immer an neuen Ideen", rgb(174, 112, 255)},
	})

	y = hero.Bottom + gap
	// Row 2: biography and current work.
	bioW := width * 52 / 100
	about := RECT{left, y, left + bioW, y + 215}
	current := RECT{about.Right + gap, y, right, y + 215}
	modernCard(hdc, about, rgb(95, 180, 255))
	modernCard(hdc, current, colGood)
	cardHeader(hdc, about, "▤", "Über Markus", "Ideen werden zu echten Projekten", rgb(95, 180, 255))
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
	ar := RECT{about.Left + 18, about.Top + 68, about.Right - 18, about.Bottom - 50}
	drawText(hdc, "Mich begeistern Projekte, bei denen man Probleme nicht nur beschreibt, sondern löst: vom Homelab über Windows-Tools bis zu Hardware, die Hersteller längst abgeschrieben haben. Dabei zählen für mich klare Bedienung, nachvollziehbare Sicherheit und praktische Lösungen mehr als unnötiger Ballast.", &ar, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)
	// Three small values at the bottom.
	vals := []struct {
		icon, text string
		color      uintptr
	}{{"♣", "Nachhaltiger denken", colGood}, {"⚡", "Effizienter arbeiten", rgb(255, 196, 96)}, {"○", "Wissen teilen", colAccent}}
	vw := (about.Right - about.Left - 44) / 3
	for i, v := range vals {
		l := about.Left + 16 + int32(i)*vw
		r := RECT{l, about.Bottom - 45, l + vw - 6, about.Bottom - 12}
		pSetTextColor.Call(hdc, v.color)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		drawText(hdc, v.icon+"  "+v.text, &r, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
	}

	cardHeader(hdc, current, "↗", "Aktuell", "Woran ich gerade arbeite", colGood)
	paintInfoRows(hdc, RECT{current.Left + 16, current.Top + 65, current.Right - 16, current.Bottom - 14}, []infoRow{
		{"LogiMate UI & Stabilität", "Moderne Nutzerführung und sichere Geräteverwaltung", colAccent},
		{"Native Wheel Engine", displayVersion(Version) + " · " + buildLabel(), rgb(174, 112, 255)},
		{"Indicana ausbauen", "Weitere praktische Tools und Dienste bündeln", colGood},
	})

	y = about.Bottom + gap
	// Row 3: projects, favorite areas and support.
	projectsW := width * 44 / 100
	favoriteW := width * 25 / 100
	projects := RECT{left, y, left + projectsW, y + 255}
	favorite := RECT{projects.Right + gap, y, projects.Right + gap + favoriteW, y + 255}
	support := RECT{favorite.Right + gap, y, right, y + 255}
	modernCard(hdc, projects, colAccent)
	modernCard(hdc, favorite, rgb(174, 112, 255))
	modernCard(hdc, support, rgb(255, 103, 157))
	cardHeader(hdc, projects, "◆", "Projekte", "Ein gemeinsames Zuhause für viele Ideen", colAccent)
	cardHeader(hdc, favorite, "♥", "Lieblingsbereiche", "Was mich antreibt", rgb(174, 112, 255))
	cardHeader(hdc, support, "♥", "Support", "Sharing is caring", rgb(255, 103, 157))

	// Two visible project rows + Indicana hub.
	projTop := projects.Top + 64
	projH := int32(48)
	logi := RECT{projects.Left + 14, projTop, projects.Right - 14, projTop + projH}
	indicana := RECT{projects.Left + 14, projTop + 56, projects.Right - 14, projTop + 56 + projH}
	cycle := RECT{projects.Left + 14, projTop + 112, projects.Right - 14, projTop + 112 + projH}
	aboutGitHubRect = logi
	aboutIndicanaRect = indicana
	projectMiniCard(hdc, logi, "◉", "LogiMate", "Wheel-Manager · Open Source", colAccent)
	projectMiniCard(hdc, indicana, "◆", "Indicana Projekte", "Alle Projekte im Hub öffnen", rgb(174, 112, 255))
	projectMiniCard(hdc, cycle, "△", "TheLittleCyclist", "MTB · Technik · Outdoor", rgb(255, 145, 95))

	paintInfoRows(hdc, RECT{favorite.Left + 14, favorite.Top + 64, favorite.Right - 14, favorite.Bottom - 14}, []infoRow{
		{"Technologie", "Windows · Apps · Automatisierung", colAccent},
		{"Nachhaltigkeit", "Technik länger sinnvoll nutzen", colGood},
		{"Outdoor & Natur", "MTB · Touren · neue Perspektiven", rgb(255, 196, 96)},
		{"Homelab", "Self-Hosted · Raspberry Pi · Dienste", rgb(174, 112, 255)},
	})

	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBrand))
	st := RECT{support.Left + 18, support.Top + 67, support.Right - 18, support.Top + 99}
	drawText(hdc, "Sharing is caring ♥", &st, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	sd := RECT{support.Left + 18, support.Top + 101, support.Right - 18, support.Top + 151}
	drawText(hdc, "Wenn dir meine Projekte helfen, kannst du ihre Weiterentwicklung freiwillig unterstützen.", &sd, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)
	aboutPayPalRect = RECT{support.Left + 18, support.Top + 160, support.Right - 18, support.Top + 204}
	drawRoundRect(hdc, aboutPayPalRect, 15, colAccent, colAccent)
	pSetTextColor.Call(hdc, colOnAccent)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	drawText(hdc, "Mit PayPal unterstützen  →", &aboutPayPalRect, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	email := RECT{support.Left + 18, support.Top + 207, support.Right - 18, support.Bottom - 10}
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	drawText(hdc, aboutPayPalEmail, &email, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	pRestoreDC.Call(hdc, saved)
	paintScrollBar(hdc, content, visibleH, virtualH)

	if projectPanelOpen {
		paintIndicanaProjectPanel(hdc, content)
	}
}

func projectMiniCard(hdc uintptr, r RECT, icon, title, detail string, accent uintptr) {
	drawRoundRect(hdc, r, 12, colPanel2, blendColor(accent, colBorder, 66))
	iconR := RECT{r.Left + 9, r.Top + 8, r.Left + 41, r.Bottom - 8}
	iconFill := blendColor(accent, colPanel2, 68)
	fillRoundRect(hdc, iconR, 9, iconFill)
	pSetTextColor.Call(hdc, readableTextColor(accent, iconFill))
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontBody))
	drawText(hdc, icon, &iconR, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	tr := RECT{iconR.Right + 10, r.Top + 5, r.Right - 30, r.Top + 26}
	drawText(hdc, title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted2)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	dr := RECT{iconR.Right + 10, r.Top + 24, r.Right - 30, r.Bottom - 5}
	drawText(hdc, detail, &dr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
	ar := RECT{r.Right - 28, r.Top, r.Right - 6, r.Bottom}
	drawText(hdc, "→", &ar, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)
}

func paintIndicanaProjectPanel(hdc uintptr, content RECT) {
	// In-app project hub. It is intentionally modal only inside the content
	// area; navigation remains visible so the user never feels trapped.
	panel := RECT{content.Left + 38, content.Top + 34, content.Right - 38, content.Bottom - 34}
	projectPanelRect = panel
	drawRoundRect(hdc, panel, 22, blendColor(colPanel, themeBackground, 5), colAccent)
	projectPanelClose = RECT{panel.Right - 52, panel.Top + 16, panel.Right - 16, panel.Top + 52}
	drawRoundRect(hdc, projectPanelClose, 10, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontSection))
	drawText(hdc, "×", &projectPanelClose, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colText)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontBrand))
	title := RECT{panel.Left + 24, panel.Top + 18, panel.Right - 70, panel.Top + 49}
	drawText(hdc, "Indicana · Projekte", &title, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	pSetTextColor.Call(hdc, colMuted)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSubtitle))
	sub := RECT{panel.Left + 24, panel.Top + 50, panel.Right - 70, panel.Top + 72}
	drawText(hdc, "Werkzeuge, Technik und Ideen von Markus Kleine", &sub, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	projects := []struct {
		icon, title, desc string
		accent            uintptr
	}{
		{"◉", "LogiMate", "Klassische Logitech-Lenkräder unter modernem Windows verwalten.", colAccent},
		{"◆", "Indicana Tools", "Praktische Web- und Alltagstools in einer gemeinsamen Sammlung.", rgb(174, 112, 255)},
		{"⚡", "StromPilot", "Stromverbrauch in Home Assistant sichtbar und verständlich machen.", rgb(116, 224, 126)},
		{"♣", "Green-ITea", "IT-Service, Nachhaltigkeit und Technik länger sinnvoll nutzen.", rgb(82, 214, 161)},
		{"△", "TheLittleCyclist", "E-MTB, Touren, Technik, Outdoor und Community.", rgb(255, 145, 95)},
	}
	top := panel.Top + 90
	gap := int32(10)
	cardH := int32(68)
	for i, p := range projects {
		r := RECT{panel.Left + 22, top + int32(i)*(cardH+gap), panel.Right - 22, top + int32(i)*(cardH+gap) + cardH}
		projectPanelRects[i] = r
		drawRoundRect(hdc, r, 14, colPanel2, blendColor(p.accent, colBorder, 60))
		iconR := RECT{r.Left + 12, r.Top + 13, r.Left + 54, r.Bottom - 13}
		iconFill := blendColor(p.accent, colPanel2, 68)
		fillRoundRect(hdc, iconR, 11, iconFill)
		pSetTextColor.Call(hdc, readableTextColor(p.accent, iconFill))
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSection))
		drawText(hdc, p.icon, &iconR, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colText)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
		tr := RECT{iconR.Right + 12, r.Top + 8, r.Right - 80, r.Top + 31}
		drawText(hdc, p.title, &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colMuted2)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
		dr := RECT{iconR.Right + 12, r.Top + 31, r.Right - 80, r.Bottom - 7}
		drawText(hdc, p.desc, &dr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		pSelectObject.Call(hdc, old)
		pSetTextColor.Call(hdc, colMuted)
		old, _, _ = pSelectObject.Call(hdc, uintptr(fontBody))
		ar := RECT{r.Right - 52, r.Top, r.Right - 16, r.Bottom}
		drawText(hdc, "→", &ar, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(hdc, old)
	}
}

func aboutHubHitTest(x, y int32) int {
	if currentPage != pageAbout {
		return 0
	}
	if projectPanelOpen {
		if pointIn(projectPanelClose, x, y) {
			return 100
		}
		for i, r := range projectPanelRects {
			if pointIn(r, x, y) {
				return 110 + i
			}
		}
		if pointIn(projectPanelRect, x, y) {
			return 101 // consume empty clicks only inside the panel
		}
		return 0
	}
	if pointIn(aboutIndicanaRect, x, y) {
		return 1
	}
	if pointIn(aboutPayPalRect, x, y) {
		return 2
	}
	if pointIn(aboutGitHubRect, x, y) {
		return 3
	}
	return 0
}

func pointIn(r RECT, x, y int32) bool {
	return r.Right > r.Left && x >= r.Left && x <= r.Right && y >= r.Top && y <= r.Bottom
}

func handleAboutHubClick(code int) bool {
	switch code {
	case 1:
		projectPanelOpen = true
		invalidate(mainWnd)
		return true
	case 2:
		if err := openExternalURL(aboutPayPalURL); err != nil {
			messageBox(mainWnd, err.Error(), "PayPal", MB_OK|MB_ICONERROR)
		} else {
			setActionFeedback("PayPal im Browser geöffnet. Danke ♥")
		}
		return true
	case 3, 110:
		if err := openExternalURL(aboutGitHubURL); err != nil {
			messageBox(mainWnd, err.Error(), "GitHub", MB_OK|MB_ICONERROR)
		}
		return true
	case 100:
		projectPanelOpen = false
		invalidate(mainWnd)
		return true
	case 111:
		if err := openExternalURL(aboutIndicanaToolsURL); err != nil {
			messageBox(mainWnd, err.Error(), "Indicana Tools", MB_OK|MB_ICONERROR)
		}
		return true
	case 112, 113, 114:
		// These projects currently have no canonical external URL in LogiMate.
		setActionFeedback("Projekt ist im Indicana-Hub dokumentiert; externer Link folgt später.")
		return true
	case 101:
		return true
	}
	return false
}
