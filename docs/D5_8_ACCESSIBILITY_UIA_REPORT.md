# D5.8 Accessibility / UI Automation Foundation

**Release:** 0.5.8-alpha  
**Purpose:** close the main-window software gap in the Stable accessibility gate without replacing LogiMate's custom renderer or fabricating Windows validation evidence.

## Architecture

LogiMate's main window is custom-painted. D5.8 creates transparent native Windows child controls over its interactive regions. Windows therefore supplies the standard accessibility provider for BUTTON, CheckBox and RadioButton semantics. The children do not paint the product UI and are pointer-transparent; mouse interaction remains on the existing custom surface.

UI Automation/native activation is routed into the same existing LogiMate action functions used by keyboard/mouse input. There is no independent accessibility state model.

## Covered main-window surfaces

- Sidebar navigation
- Four page action buttons
- Wheel `Erweiterte Ansicht` toggle
- Settings theme selection
- Settings toggles
- Current page title/subtitle/live body through a native status surface

The bridge synchronizes visibility, enabled state, names and checked/selected state on every painted frame. Keyboard focus can move to the same native peer and Settings auto-scrolls focused controls into view.

## Diagnostics

Diagnostics and Engine Health now report whether the D5.8 native accessibility peers were created, the expected/actual control count, how many are currently visible, whether the status surface exists and the current keyboard focus ID.

## Stable boundary

This release deliberately keeps `uiAccessibilityValidated=false`. The software can prove that the bridge compiles and that its IDs/focus model are internally coherent; it cannot prove screen-reader behavior on a Linux build host. Real Windows evidence remains required for Narrator/UI Automation, keyboard traversal, High Contrast, reduced motion, mixed DPI and secondary custom dialogs/setup/changelog.
