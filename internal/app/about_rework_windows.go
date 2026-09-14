//go:build windows

package app

import (
	"fmt"

	"github.com/thelittlespace/LogiMate/internal/system"
)

// Build 008: product-first About page with fitted text and WCAG-aware accents. Project history stays in CHANGELOG/docs;
// the UI only shows durable facts so it cannot become stale after each audit.
func paintAboutHub(hdc uintptr, content RECT, s system.State) {
	left, right := content.Left, content.Right
	visibleTop, visibleBottom := content.Top, content.Bottom
	visibleH := visibleBottom - visibleTop
	virtualH := int32(780)
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
	aboutGitHubRect, aboutIndicanaRect, aboutPayPalRect = RECT{}, RECT{}, RECT{}
	saved, _, _ := pSaveDC.Call(hdc)
	pIntersectClipRect.Call(hdc, uintptr(left), uintptr(visibleTop), uintptr(right), uintptr(visibleBottom))

	// Product hero: version/build and purpose are visible before personal/project details.
	hero := RECT{left, y, right, y + 168}
	modernCard(hdc, hero, colAccent)
	logo := RECT{hero.Left + 24, hero.Top + 28, hero.Left + 112, hero.Top + 116}
	drawBrandIcon(hdc, logo)
	pSetTextColor.Call(hdc, colText)
	old, _, _ := pSelectObject.Call(hdc, uintptr(fontMetric))
	tr := RECT{logo.Right + 22, hero.Top + 24, hero.Right - 260, hero.Top + 67}
	drawText(hdc, "LogiMate", &tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	heroAccent := readableAccentOnPanel2(colAccent)
	pSetTextColor.Call(hdc, heroAccent)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSection))
	sr := RECT{logo.Right + 22, hero.Top + 67, hero.Right - 260, hero.Top + 94}
	drawText(hdc, "Modernes Control Center für klassische Logitech Wheels", &sr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	br := RECT{logo.Right + 22, hero.Top + 98, hero.Right - 270, hero.Bottom - 20}
	drawFittedParagraph(hdc, "G25, G27 und Driving Force GT: Erkennung, Kalibrierung, Profile, Force Feedback, Diagnose, sichere Moduswechsel und Recovery in einer Oberfläche.", br, colMuted, fontBody, fontSmall)
	chip1 := RECT{hero.Right - 235, hero.Top + 30, hero.Right - 24, hero.Top + 68}
	drawRoundRect(hdc, chip1, 14, colSelected, colAccent)
	pSetTextColor.Call(hdc, readableTextColor(colAccent, colSelected))
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	drawText(hdc, displayVersion(Version)+" · "+buildLabel(), &chip1, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)
	chip2 := RECT{hero.Right - 235, hero.Top + 79, hero.Right - 24, hero.Top + 117}
	drawRoundRect(hdc, chip2, 14, colPanel2, colBorder)
	pSetTextColor.Call(hdc, colGood)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontSmall))
	drawText(hdc, "●  Alpha · lokale Daten", &chip2, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	y = hero.Bottom + gap
	aboutW := width * 58 / 100
	product := RECT{left, y, left + aboutW, y + 190}
	developer := RECT{product.Right + gap, y, right, y + 190}
	modernCard(hdc, product, colAccent)
	cardHeader(hdc, product, "◉", "Über LogiMate", "Sicherheit und Transparenz vor stiller Magie", colAccent)
	paintCardParagraph(hdc, product, "LogiMate hält Modern- und Legacy-Betrieb bewusst getrennt. Motorbefehle laufen über einen zentralen OutputLease, Watchdog und Emergency Neutralize; Profile dürfen harte Safety-Caps nicht übersteuern. OpenG27 bleibt Referenz/Import-Provenance, nicht Modern-Runtime.", 70)
	aboutGitHubRect = RECT{product.Left + 18, product.Bottom - 44, product.Right - 18, product.Bottom - 12}
	wheelPanelButton(hdc, aboutGitHubRect, "Open Source auf GitHub  →", colAccent, false)
	modernCard(hdc, developer, rgb(115, 177, 255))
	cardHeader(hdc, developer, "MK", "Markus Kleine", "Entwickler & Projektidee", rgb(115, 177, 255))
	paintCardParagraph(hdc, developer, "Ich entwickle LogiMate aus einem praktischen Ziel heraus: ältere, gute Hardware soll unter aktuellem Windows verständlich, kontrollierbar und langfristig nutzbar bleiben. Entscheidend sind klare Bedienung, nachvollziehbare Zustände und reproduzierbare Fehlerdiagnose.", 70)

	y = product.Bottom + gap
	third := (width - gap*2) / 3
	oss := RECT{left, y, left + third, y + 160}
	privacy := RECT{oss.Right + gap, y, oss.Right + gap + third, y + 160}
	status := RECT{privacy.Right + gap, y, right, y + 160}
	modernCard(hdc, oss, rgb(174, 112, 255))
	cardHeader(hdc, oss, "◆", "Open Source & Credits", "Provenance bleibt sichtbar", rgb(174, 112, 255))
	paintCardParagraph(hdc, oss, "OpenG27 © Jabelius · MIT. Portierte Referenzideen und Testvektoren bleiben dokumentiert; produktiver Modern-Betrieb ist eigenständig.", 70)
	modernCard(hdc, privacy, colGood)
	cardHeader(hdc, privacy, "⌂", "Privatsphäre", "Local-first", colGood)
	paintCardParagraph(hdc, privacy, "Kein Benutzerkonto, keine eigene Nutzungstelemetrie. Diagnoseexporte bleiben lokal, bis du sie selbst weitergibst. Onlinezugriffe dienen dem Updateweg.", 70)
	modernCard(hdc, status, colWarning)
	cardHeader(hdc, status, "!", "Alpha-Status", "Ehrliche Grenzen", colWarning)
	paintCardParagraph(hdc, status, fmt.Sprintf("%s bleibt bewusst Alpha. Stable benötigt weiterhin reale Hardware-, Recovery-, Accessibility-, HID-Stress- und Signing-Evidenz.", displayVersion(Version)), 70)

	y = oss.Bottom + gap
	projects := RECT{left, y, left + width*62/100, y + 220}
	support := RECT{projects.Right + gap, y, right, y + 220}
	modernCard(hdc, projects, colAccent)
	cardHeader(hdc, projects, "◇", "Weitere Projekte", "Werkzeuge, Technik und Outdoor", colAccent)
	p1 := RECT{projects.Left + 18, projects.Top + 70, projects.Right - 18, projects.Top + 119}
	p2 := RECT{projects.Left + 18, projects.Top + 128, projects.Right - 18, projects.Top + 177}
	aboutIndicanaRect = p1
	projectMiniCard(hdc, p1, "◆", "Indicana Projekte", "Tools, Dienste und weitere Softwareprojekte", rgb(174, 112, 255))
	projectMiniCard(hdc, p2, "△", "TheLittleCyclist", "E-MTB · Technik · Outdoor", rgb(255, 145, 95))
	modernCard(hdc, support, rgb(255, 103, 157))
	cardHeader(hdc, support, "♥", "Support", "Freiwillig · kein Zwang", rgb(255, 103, 157))
	paintCardParagraph(hdc, support, "Wenn LogiMate oder ein anderes Projekt hilfreich ist, kannst du die Weiterentwicklung freiwillig unterstützen. Das ändert weder Funktionen noch Antworten oder Prioritäten.", 70)
	aboutPayPalRect = RECT{support.Left + 18, support.Bottom - 58, support.Right - 18, support.Bottom - 18}
	drawRoundRect(hdc, aboutPayPalRect, 14, colAccent, colAccent)
	pSetTextColor.Call(hdc, colOnAccent)
	old, _, _ = pSelectObject.Call(hdc, uintptr(fontLabel))
	drawText(hdc, "PayPal unterstützen  →", &aboutPayPalRect, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	pSelectObject.Call(hdc, old)

	pRestoreDC.Call(hdc, saved)
	paintScrollBar(hdc, content, visibleH, virtualH)
	if projectPanelOpen {
		paintIndicanaProjectPanel(hdc, content)
	}
}
