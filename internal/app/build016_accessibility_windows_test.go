//go:build windows

package app

import "testing"

func TestBuild016AllWheelTabsHaveAccessibilityCommands(t *testing.T) {
	for i := 0; i < 5; i++ {
		id := accessibilityIDD6TabBase + i
		if id >= accessibilityIDD6Toggle {
			t.Fatalf("wheel-tab accessibility id overlaps controls: %d", id)
		}
	}
}

func TestBuild016DiagnosticsFocusNamespaces(t *testing.T) {
	if focusDiagnosticsTabBase+diagnosticsTabCount >= focusDiagnosticsActionBase {
		t.Fatalf("diagnostics tab/action focus ranges overlap")
	}
}
