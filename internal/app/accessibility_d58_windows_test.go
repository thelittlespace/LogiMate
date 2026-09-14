//go:build windows

package app

import "testing"

func TestD58WheelFocusOrderIncludesAdvancedView(t *testing.T) {
	oldPage, oldButtons, oldTab := currentPage, actionButtons, wheelSubtab
	defer func() { currentPage, actionButtons, wheelSubtab = oldPage, oldButtons, oldTab }()
	currentPage = pageWheel
	wheelSubtab = wheelSubtabLive
	for i := range actionButtons {
		actionButtons[i].visible = false
		actionButtons[i].enabled = false
	}
	order := focusOrder()
	found := false
	for _, id := range order {
		if id == focusWheelAdvanced {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("D5.8 advanced wheel view missing from keyboard focus order: %v", order)
	}
}

func TestD58AccessibilityControlIDsDoNotOverlap(t *testing.T) {
	ids := map[int]string{}
	add := func(id int, label string) {
		if prev, ok := ids[id]; ok {
			t.Fatalf("accessibility control id %d overlaps: %s / %s", id, prev, label)
		}
		ids[id] = label
	}
	for i := range navItems {
		add(accessibilityIDNavBase+i, "nav")
	}
	add(accessibilityIDAdvanced, "advanced")
	add(accessibilityIDMemoryIntegrity, "memory-integrity")
	for i := range themeChoices {
		add(accessibilityIDThemeBase+i, "theme")
	}
	for i := range settingDefs {
		add(accessibilityIDSettingBase+i, "setting")
	}
	for i := range actionButtons {
		add(accessibilityIDActionBase+i, "action")
	}
	for i := 0; i < 5; i++ {
		add(accessibilityIDD6TabBase+i, "wheel-tab")
	}
	add(accessibilityIDD6Toggle, "d6-toggle")
	for i := 0; i < int(d6SliderCount); i++ {
		add(accessibilityIDD6SliderBase+i, "d6-slider")
	}
	for i := 0; i < 3; i++ {
		add(accessibilityIDD6ProfileBase+i, "d6-profile")
	}
	for i := 0; i < 4; i++ {
		add(accessibilityIDD6ShapeBase+i, "d6-shape")
	}
	for i := 0; i < 6; i++ {
		add(accessibilityIDD6TestBase+i, "d6-test")
	}
	for i := 0; i < 4; i++ {
		add(accessibilityIDWheelCalBase+i, "wheel-cal-action")
		add(accessibilityIDWheelProfBase+i, "wheel-profile-action")
		add(accessibilityIDWheelDevBase+i, "wheel-device-action")
		add(accessibilityIDDiagActionBase+i, "diagnostics-action")
	}
	for i := 0; i < diagnosticsTabCount; i++ {
		add(accessibilityIDDiagTabBase+i, "diagnostics-tab")
	}
}

func TestD58FocusNamespacesDoNotOverlap(t *testing.T) {
	ids := map[int]string{}
	add := func(id int, label string) {
		if prev, ok := ids[id]; ok {
			t.Fatalf("focus id %d overlaps: %s / %s", id, prev, label)
		}
		ids[id] = label
	}
	for i := range navItems {
		add(i, "nav")
	}
	add(focusWheelAdvanced, "advanced")
	add(focusMemoryIntegrity, "memory-integrity")
	for i := range actionButtons {
		add(focusActionBase+i, "action")
	}
	for i := range themeChoices {
		add(focusThemeBase+i, "theme")
	}
	for i := range settingDefs {
		add(focusSettingBase+i, "setting")
	}
	add(d6FocusTabLive, "d6-live-tab")
	add(d6FocusTabFFB, "d6-ffb-tab")
	add(d6FocusToggle, "d6-toggle")
	for i := 0; i < int(d6SliderCount); i++ {
		add(d6FocusSliderBase+i, "d6-slider")
	}
	for i := 0; i < 3; i++ {
		add(d6FocusProfileBase+i, "d6-profile")
	}
	for i := 0; i < 4; i++ {
		add(d6FocusShapeBase+i, "d6-shape")
	}
	for i := 0; i < 6; i++ {
		add(d6FocusTestBase+i, "d6-test")
	}
	for i := 0; i < 4; i++ {
		add(focusWheelCalibrationActionBase+i, "wheel-cal-action")
		add(focusWheelProfileActionBase+i, "wheel-profile-action")
		add(focusWheelDeviceActionBase+i, "wheel-device-action")
		add(focusDiagnosticsActionBase+i, "diagnostics-action")
	}
	for i := 0; i < diagnosticsTabCount; i++ {
		add(focusDiagnosticsTabBase+i, "diagnostics-tab")
	}
}

func TestBuild018MemoryIntegrityStatusIsKeyboardReachable(t *testing.T) {
	oldPage, oldRect, oldButtons := currentPage, memoryIntegrityRect, actionButtons
	defer func() { currentPage, memoryIntegrityRect, actionButtons = oldPage, oldRect, oldButtons }()
	currentPage = pageWheel
	memoryIntegrityRect = RECT{10, 10, 180, 40}
	for i := range actionButtons {
		actionButtons[i].visible = false
		actionButtons[i].enabled = false
	}
	for _, id := range focusOrder() {
		if id == focusMemoryIntegrity {
			return
		}
	}
	t.Fatalf("Build 018 memory-integrity status missing from keyboard focus order: %v", focusOrder())
}
