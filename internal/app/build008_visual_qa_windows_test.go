//go:build windows

package app

import "testing"

func TestBuild008ThemeSemanticTextContrast(t *testing.T) {
	for _, mode := range []string{"dark", "gray", "light"} {
		p := paletteFor(mode)
		backgrounds := []struct {
			name string
			c    uintptr
		}{{"background", p.background}, {"panel", p.panel}, {"panel2", p.panel2}}
		foregrounds := []struct {
			name string
			c    uintptr
		}{{"text", p.text}, {"muted", p.muted}, {"muted2", p.muted2}, {"accent", p.accent}, {"warning", p.warning}, {"good", p.good}, {"bad", p.bad}}
		for _, fg := range foregrounds {
			for _, bg := range backgrounds {
				if got := colorContrastRatio(fg.c, bg.c); got < 4.5 {
					t.Fatalf("%s: %s on %s contrast %.2f < 4.5", mode, fg.name, bg.name, got)
				}
			}
		}
		if got := colorContrastRatio(p.onAccent, p.accent); got < 4.5 {
			t.Fatalf("%s: onAccent/accent contrast %.2f < 4.5", mode, got)
		}
	}
}

func TestBuild008SettingsRowsLeaveTwoLineDescriptionSpace(t *testing.T) {
	if settingRowH < 60 {
		t.Fatalf("setting row height %d is too small for title + two-line description", settingRowH)
	}
	if settingGroupH < 46 {
		t.Fatalf("setting group height %d is too small for title + description", settingGroupH)
	}
}
