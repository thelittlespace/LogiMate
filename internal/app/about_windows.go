//go:build windows

package app

const (
	aboutPayPalEmail = "indicana@tutanota.de"
	// PayPal's classic donation endpoint accepts a confirmed account e-mail in
	// the business parameter. Keep this value centralized so it can later be
	// replaced by a dedicated PayPal payment/donation link without touching UI.
	aboutPayPalURL        = "https://www.paypal.com/cgi-bin/webscr?cmd=_donations&business=indicana%40tutanota.de&item_name=LogiMate+und+Indicana+Projekte&currency_code=EUR"
	aboutGitHubURL        = "https://github.com/thelittlespace/LogiMate"
	aboutIndicanaToolsURL = "https://github.com/thelittlespace/indicana-tools"
)

func aboutPageText() string {
	return `Markus Kleine
Ich bin Markus Kleine – Entwickler, Tüftler und der Kopf hinter LogiMate. Mich reizen Projekte, bei denen ältere Hardware durch moderne, verständliche Software wieder richtig gut nutzbar wird. LogiMate ist genau daraus entstanden: alte Logitech-Lenkräder sollen sich unter aktuellem Windows nicht wie Legacy-Hardware anfühlen.

LogiMate
Mein modernes Windows-Werkzeug für Logitech G25, G27 und Driving Force GT: Erkennung, Live-Diagnose, sichere Treiberwechsel, Backups, Recovery und eine klare Brücke zwischen alter Logitech-Software und modernen Community-Lösungen. Sicherheit vor blindem Automatisieren ist dabei Absicht.

Indicana Tools
Eine wachsende Sammlung praktischer Web- und Alltagstools. Der Fokus liegt auf einer schnellen, übersichtlichen Oberfläche, Dark Mode und Funktionen, die ohne unnötigen Ballast direkt helfen.

StromPilot
Ein Energie- und Stromprojekt rund um Home Assistant: Verbrauch verständlicher machen, Daten sinnvoll visualisieren und aus Messwerten echte Entscheidungen ableiten.

Green-ITea · IT-Service
PC-Service, Reparatur, Systembau, Windows-Optimierung und Softwarehilfe mit dem Ziel, Technik länger sinnvoll nutzbar zu halten statt sie vorschnell zu ersetzen.

TheLittleCyclist & weitere Projekte
E-MTB, Touren, Technik, DIY, Self-Hosting, Raspberry Pi und Windows-Tools gehören ebenfalls zu meinen Projekten. Vieles beginnt als eigenes Problem – und wird dann zu etwas, das auch anderen helfen kann.

Open Source & Credits
LogiMate respektiert die Arbeit anderer Projekte. OpenG27 ist ein eigenständiges MIT-lizenziertes Open-Source-Projekt von Jabelius. Ausgewählte Core-Ideen und -Komponenten wurden mit dokumentierter Provenance nach Go portiert und werden in LogiMates eigener Native Wheel Engine weiterentwickelt; Copyright- und Lizenzhinweise bleiben erhalten.

Privatsphäre
LogiMate braucht kein Benutzerkonto und enthält keine eigene Nutzungs-Telemetrie. Onlinezugriffe dienen ausschließlich dem LogiMate-Updateweg; Diagnoseexporte bleiben lokal, bis du sie selbst weitergibst. OpenG27 wird von LogiMate weder gesucht noch heruntergeladen oder gestartet.

Sharing is caring ♥
Wenn dir LogiMate oder eines meiner Projekte Arbeit erspart hat, kannst du die Weiterentwicklung freiwillig über PayPal unterstützen. PayPal: indicana@tutanota.de. Das ist eine freiwillige Unterstützung meiner Projekte und keine steuerlich absetzbare Spende an eine gemeinnützige Organisation.`
}
