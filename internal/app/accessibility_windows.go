//go:build windows

package app

var keyboardFocus = -1

const (
	focusActionBase                 = 100
	focusThemeBase                  = 200
	focusSettingBase                = 300
	focusWheelCalibrationActionBase = 800
	focusWheelProfileActionBase     = 810
	focusWheelDeviceActionBase      = 820
	focusDiagnosticsTabBase         = 840
	focusDiagnosticsActionBase      = 850
	focusMemoryIntegrity            = 860
)

func focusOrder() []int {
	out := make([]int, 0, 32)
	for i := range navItems {
		out = append(out, i)
	}
	if pageShowsStatusBand() && rectUsable(memoryIntegrityRect) {
		out = append(out, focusMemoryIntegrity)
	}
	if currentPage == pageWheel {
		out = append(out, d6FocusOrder()...)
		if wheelSubtab == wheelSubtabLive {
			out = append(out, focusWheelAdvanced)
		}
		if !wheelCalibrationWizard.Active {
			switch wheelSubtab {
			case wheelSubtabCalibration:
				for i, r := range wheelCalibrationActionRects {
					if r.Right > r.Left {
						out = append(out, focusWheelCalibrationActionBase+i)
					}
				}
			case wheelSubtabProfiles:
				for i, r := range wheelProfileActionRects {
					if r.Right > r.Left {
						out = append(out, focusWheelProfileActionBase+i)
					}
				}
			case wheelSubtabDevice:
				for i, r := range wheelDeviceActionRects {
					if r.Right > r.Left {
						out = append(out, focusWheelDeviceActionBase+i)
					}
				}
			}
		}
	}
	if currentPage == pageDiagnostics {
		for i := 0; i < diagnosticsTabCount; i++ {
			out = append(out, focusDiagnosticsTabBase+i)
		}
		for i, r := range diagnosticsActionRects {
			if r.Right > r.Left {
				out = append(out, focusDiagnosticsActionBase+i)
			}
		}
	}
	if currentPage == pageSettings {
		for i := range themeChoices {
			out = append(out, focusThemeBase+i)
		}
		for i := range settingDefs {
			out = append(out, focusSettingBase+i)
		}
	}
	for i, b := range actionButtons {
		if b.visible && b.enabled {
			out = append(out, focusActionBase+i)
		}
	}
	return out
}

func focusNext(reverse bool) {
	order := focusOrder()
	if len(order) == 0 {
		keyboardFocus = -1
		return
	}
	pos := -1
	for i, v := range order {
		if v == keyboardFocus {
			pos = i
			break
		}
	}
	if reverse {
		if pos <= 0 {
			pos = len(order)
		}
		pos--
	} else {
		pos++
		if pos >= len(order) {
			pos = 0
		}
	}
	setKeyboardFocusID(order[pos])
}

func activateKeyboardFocus() {
	f := keyboardFocus
	stateMu.RLock()
	s := appStateSnapshot()
	stateMu.RUnlock()
	if d6ActivateFocus(s) {
		return
	}
	if wheelReworkActivateFocus(s, f) {
		return
	}
	if diagnosticsActivateFocus(s, f) {
		return
	}
	switch {
	case f >= 0 && f < len(navItems):
		setPage(navItems[f].page)
	case f >= focusActionBase && f < focusActionBase+len(actionButtons):
		i := f - focusActionBase
		if actionButtons[i].visible && actionButtons[i].enabled {
			action(currentPage, i)
		}
	case f == focusWheelAdvanced:
		if currentPage == pageWheel {
			wheelAdvancedView = !wheelAdvancedView
			contentScroll = 0
			setActionFeedback(map[bool]string{true: "Erweiterte Lenkradansicht aktiv.", false: "Standardansicht aktiv."}[wheelAdvancedView])
			invalidate(mainWnd)
		}
	case f == focusMemoryIntegrity:
		if pageShowsStatusBand() && rectUsable(memoryIntegrityRect) {
			activateMemoryIntegritySettings()
		}
	case f >= focusThemeBase && f < focusThemeBase+len(themeChoices):
		setThemeMode(themeChoices[f-focusThemeBase].id)
	case f >= focusSettingBase && f < focusSettingBase+len(settingDefs):
		toggleSetting(f - focusSettingBase)
	}
}

func onKeyDown(vk uint32) bool {
	switch vk {
	case VK_TAB:
		shift, _, _ := pGetKeyState.Call(VK_SHIFT)
		focusNext(int16(shift&0xffff) < 0)
		return true
	case VK_RETURN, VK_SPACE:
		if keyboardFocus < 0 {
			focusNext(false)
		} else {
			activateKeyboardFocus()
		}
		return true
	case VK_UP, VK_LEFT:
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		if d6AdjustFocusedSlider(-1, s) {
			return true
		}
		focusNext(true)
		return true
	case VK_DOWN, VK_RIGHT:
		stateMu.RLock()
		s := appStateSnapshot()
		stateMu.RUnlock()
		if d6AdjustFocusedSlider(1, s) {
			return true
		}
		focusNext(false)
		return true
	case VK_ESCAPE:
		if projectPanelOpen {
			projectPanelOpen = false
		}
		keyboardFocus = -1
		if mainWnd != 0 {
			pSetFocus.Call(uintptr(mainWnd))
		}
		invalidate(mainWnd)
		return true
	}
	return false
}

func animationsAllowed() bool {
	p := getUISettings()
	return p.Animations && !highContrastEnabled() && clientAnimationsEnabled()
}
