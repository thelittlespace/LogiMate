//go:build windows

package app

import (
	"math"
	"testing"
)

func TestDefaultUISettings(t *testing.T) {
	old := Version
	Version = "0.2.0-alpha"
	defer func() { Version = old }()
	p := defaultUISettings()
	if p.ThemeMode != "dark" {
		t.Fatalf("default theme = %q, want dark", p.ThemeMode)
	}
	if !p.Acrylic || !p.Animations || !p.SidebarAutoExpand || !p.AutoRefresh || !p.CheckUpdates || p.AutoDownloadUpdate || !p.PreviewUpdates || !p.ShowWhatsNew || !p.OfferSetup {
		t.Fatalf("recommended visual/runtime/update defaults changed unexpectedly: %+v", p)
	}
	if p.StartWithWindows {
		t.Fatal("Windows autostart must remain opt-in")
	}
	if p.NativeWheelOutput {
		t.Fatal("native wheel output must remain opt-in")
	}
}

func TestSettingsValuesOrder(t *testing.T) {
	p := uiSettings{Acrylic: true, Animations: false, SidebarAutoExpand: true, AutoRefresh: false, NativeWheelOutput: false, CheckUpdates: false, AutoDownloadUpdate: true, PreviewUpdates: false, ShowWhatsNew: true, StartWithWindows: false, OfferSetup: true}
	got := settingsValues(p)
	want := []bool{true, false, true, false, false, false, true, false, true, false, true}
	if len(got) != len(want) {
		t.Fatalf("settingsValues length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("settingsValues[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestCompactWheelNames(t *testing.T) {
	cases := map[string]string{
		"":                                     "Nicht erkannt",
		"Logitech G25":                         "Logitech G25",
		"Logitech G27":                         "Logitech G27",
		"Logitech C294 (Kompatibilitätsmodus)": "C294 • Modell offen",
		"Logitech Driving Force GT":            "Driving Force GT",
		"Logitech G27 (manuell bestätigt / C294)": "Logitech G27",
	}
	for in, want := range cases {
		if got := compactWheelName(in); got != want {
			t.Fatalf("compactWheelName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestActionLayoutNoOverlap(t *testing.T) {
	oldButtons := actionButtons
	oldSidebar := sidebarWidth
	oldAnim := pageAnim
	defer func() {
		actionButtons = oldButtons
		sidebarWidth = oldSidebar
		pageAnim = oldAnim
	}()

	for i := range actionButtons {
		actionButtons[i] = actionButton{label: "Action", visible: true}
	}
	sidebarWidth = sidebarExpanded
	pageAnim = 1
	actionLayout(RECT{Left: 0, Top: 0, Right: 960, Bottom: 650})

	for i := 0; i < len(actionRects); i++ {
		r := actionRects[i]
		if r.Right <= r.Left || r.Bottom <= r.Top {
			t.Fatalf("action rect %d is empty: %+v", i, r)
		}
		if i > 0 && actionRects[i-1].Right >= r.Left {
			t.Fatalf("action rects overlap: prev=%+v current=%+v", actionRects[i-1], r)
		}
	}
}

func TestActionIconsAreStable(t *testing.T) {
	cases := map[string]string{
		"Generic HID":             "↗",
		"Modern / Generic HID":    "↗",
		"Logitech Legacy":         "↶",
		"Treiber sichern":         "↓",
		"Aktualisieren":           "↻",
		"Profilordner":            "⌂",
		"Modell wählen":           "✓",
		"Bericht kopieren":        "⧉",
		"Was ist neu?":            "i",
		"Zurücksetzen":            "↺",
		"Legacy wiederherstellen": "↶",
		"Original Logitech":       "↶",
	}
	for label, want := range cases {
		if got := actionIcon(label); got != want {
			t.Fatalf("actionIcon(%q) = %q, want %q", label, got, want)
		}
	}
}

func TestThemeModes(t *testing.T) {
	cases := map[string]string{
		"dark": "dark", "DUNKEL": "dark", "gray": "gray", "grey": "gray", "Grau": "gray", "light": "light", "Hell": "light", "system": "system", "Windows": "system", "": "dark",
	}
	for in, want := range cases {
		if got := normalizeThemeMode(in); got != want {
			t.Fatalf("normalizeThemeMode(%q) = %q, want %q", in, got, want)
		}
	}
	if len(themeChoices) != 4 {
		t.Fatalf("theme choice count = %d, want 4", len(themeChoices))
	}
}

func TestThemeMaterialBackdrops(t *testing.T) {
	if got, _ := materialBackdropForTheme("dark"); got != 3 {
		t.Fatalf("dark backdrop=%d, want Acrylic(3)", got)
	}
	if got, _ := materialBackdropForTheme("gray"); got != 4 {
		t.Fatalf("gray backdrop=%d, want Mica Alt(4)", got)
	}
	if got, _ := materialBackdropForTheme("light"); got != 2 {
		t.Fatalf("light backdrop=%d, want Mica(2)", got)
	}
}

func TestRepairAlphaPixels(t *testing.T) {
	pixels := []byte{
		0, 0, 0, 255, // glass clear must become transparent
		10, 20, 30, 0, // painted GDI pixel must become opaque
		40, 50, 60, 127, // icon/composited alpha must be preserved
	}
	repairAlphaPixels(pixels)
	if pixels[3] != 0 {
		t.Fatalf("black glass pixel alpha=%d, want 0", pixels[3])
	}
	if pixels[7] != 255 {
		t.Fatalf("painted GDI pixel alpha=%d, want 255", pixels[7])
	}
	if pixels[11] != 127 {
		t.Fatalf("existing non-zero alpha=%d, want 127", pixels[11])
	}
}

func TestAxisNormalization(t *testing.T) {
	if got := axisNorm(32767, 0, 65535); got < 0.49 || got > 0.51 {
		t.Fatalf("axisNorm midpoint = %f", got)
	}
	if got := axisNorm(0, 0, 65535); got != 0 {
		t.Fatalf("axisNorm min = %f", got)
	}
	if got := axisNorm(65535, 0, 65535); got != 1 {
		t.Fatalf("axisNorm max = %f", got)
	}
}

func TestRichBlockSplitting(t *testing.T) {
	got := splitRichBlocks("Windows\r\n11\r\n\r\nBackup\r\n3 INF")
	if len(got) != 2 {
		t.Fatalf("splitRichBlocks = %d blocks, want 2", len(got))
	}
}

func colorChannels(c uintptr) (float64, float64, float64) {
	return float64(c&0xff) / 255, float64((c>>8)&0xff) / 255, float64((c>>16)&0xff) / 255
}

func srgbLinear(x float64) float64 {
	if x <= 0.04045 {
		return x / 12.92
	}
	return pow((x+0.055)/1.055, 2.4)
}

func pow(x, y float64) float64 {
	// Small test-only exponentiation wrapper avoids adding production dependencies.
	return math.Pow(x, y)
}

func relativeLuminance(c uintptr) float64 {
	r, g, b := colorChannels(c)
	return 0.2126*srgbLinear(r) + 0.7152*srgbLinear(g) + 0.0722*srgbLinear(b)
}

func contrastRatio(a, b uintptr) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestThemePalettesAreDistinctAndReadable(t *testing.T) {
	dark := paletteFor("dark")
	gray := paletteFor("gray")
	light := paletteFor("light")
	if dark.background == gray.background || gray.background == light.background || dark.background == light.background {
		t.Fatal("theme backgrounds must be visually distinct")
	}
	if dark.accent == gray.accent || gray.accent == light.accent {
		t.Fatal("theme accents must be visually distinct")
	}
	for name, p := range map[string]themePalette{"dark": dark, "gray": gray, "light": light} {
		if got := contrastRatio(p.text, p.panel); got < 7.0 {
			t.Fatalf("%s text/panel contrast = %.2f, want >= 7.0", name, got)
		}
		if got := contrastRatio(p.muted, p.panel); got < 4.5 {
			t.Fatalf("%s muted/panel contrast = %.2f, want >= 4.5", name, got)
		}
		if got := contrastRatio(p.onAccent, p.accent); got < 4.5 {
			t.Fatalf("%s onAccent/accent contrast = %.2f, want >= 4.5", name, got)
		}
	}
	// Gray must remain materially lighter and less blue-biased than Dark.
	dr, dg, db := colorChannels(dark.background)
	gr, gg, gb := colorChannels(gray.background)
	if gr <= dr || gg <= dg || gb <= db {
		t.Fatal("gray background should be clearly lighter than dark")
	}
}

func TestSettingsAreGroupedAndComplete(t *testing.T) {
	wantGroups := map[string]bool{"appearance": false, "wheel": false, "updates": false, "startup": false}
	for _, d := range settingDefs {
		if _, ok := wantGroups[d.group]; !ok {
			t.Fatalf("unexpected settings group %q", d.group)
		}
		wantGroups[d.group] = true
	}
	for group, seen := range wantGroups {
		if !seen {
			t.Fatalf("settings group %q has no options", group)
		}
	}
	if len(settingsValues(defaultUISettings())) != len(settingDefs) {
		t.Fatalf("settings values/definitions length mismatch: %d vs %d", len(settingsValues(defaultUISettings())), len(settingDefs))
	}
}
