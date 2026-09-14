//go:build windows

package app

import "testing"

func TestProfileHubSidebarGeometry(t *testing.T) {
	if sidebarExpanded < 220 {
		t.Fatalf("expanded sidebar too narrow for profile-hub design: %d", sidebarExpanded)
	}
	if sidebarCollapsed >= sidebarExpanded {
		t.Fatalf("compact sidebar must remain narrower: compact=%d expanded=%d", sidebarCollapsed, sidebarExpanded)
	}
	old := sidebarWidth
	defer func() { sidebarWidth = old }()
	sidebarWidth = sidebarExpanded
	rc := RECT{Right: 1400, Bottom: 900}
	settings := -1
	about := -1
	for i, n := range navItems {
		if n.page == pageSettings {
			settings = i
		}
		if n.page == pageAbout {
			about = i
		}
	}
	if settings < 0 || about < 0 {
		t.Fatal("profile navigation is missing Settings or About")
	}
	sr := navItemRect(settings, rc)
	ar := navItemRect(about, rc)
	if sr.Top <= ar.Bottom+40 {
		t.Fatalf("Settings should be separated at the bottom: settings=%+v about=%+v", sr, ar)
	}
}

func TestIndicanaHubHitTargets(t *testing.T) {
	oldPage, oldOpen := currentPage, projectPanelOpen
	defer func() { currentPage, projectPanelOpen = oldPage, oldOpen }()
	currentPage = pageAbout
	projectPanelOpen = false
	aboutIndicanaRect = RECT{10, 10, 110, 60}
	if got := aboutHubHitTest(30, 30); got != 1 {
		t.Fatalf("Indicana tile hit=%d want 1", got)
	}
	projectPanelOpen = true
	projectPanelRect = RECT{100, 100, 500, 500}
	projectPanelClose = RECT{450, 110, 490, 150}
	projectPanelRects[0] = RECT{120, 180, 460, 230}
	if got := aboutHubHitTest(470, 130); got != 100 {
		t.Fatalf("panel close hit=%d want 100", got)
	}
	if got := aboutHubHitTest(200, 200); got != 110 {
		t.Fatalf("project hit=%d want 110", got)
	}
	if got := aboutHubHitTest(20, 20); got != 0 {
		t.Fatalf("navigation area must remain reachable while panel is open, hit=%d", got)
	}
}
