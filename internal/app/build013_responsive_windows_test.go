//go:build windows

package app

import "testing"

func TestBuild013ResponsiveMinimumNeverExceedsScreen(t *testing.T) {
	w, h := responsiveMinimumTrackSize(192, 1920, 1080)
	if w > 1920-64 || h > 1080-96 {
		t.Fatalf("minimum exceeds usable screen: %dx%d", w, h)
	}
	if w < 760 || h < 560 {
		t.Fatalf("minimum fell below compact usability floor: %dx%d", w, h)
	}
}

func TestBuild013ResponsiveMinimumKeepsPreferredAt96DPI(t *testing.T) {
	w, h := responsiveMinimumTrackSize(96, 2560, 1440)
	if w != 1080 || h != 740 {
		t.Fatalf("96-DPI preferred size changed: %dx%d", w, h)
	}
}

func TestBuild013ContentRectStaysOnscreen(t *testing.T) {
	oldWidth, oldPage := sidebarWidth, currentPage
	defer func() { sidebarWidth, currentPage = oldWidth, oldPage }()
	sidebarWidth = sidebarCollapsed
	currentPage = pageOverview
	r := contentRectFor(RECT{Left: 0, Top: 0, Right: 800, Bottom: 650})
	if r.Right > 800 || r.Left < 0 || r.Right <= r.Left {
		t.Fatalf("invalid compact content rect: %+v", r)
	}
}
