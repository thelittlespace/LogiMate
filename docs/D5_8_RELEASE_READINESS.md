# D5.8 Release Readiness

**Release:** 0.5.8-alpha  
**State:** software foundation complete; Windows accessibility validation intentionally pending.

## Software acceptance

- Main-window custom controls have native Windows accessibility peers.
- Navigation/actions/settings/themes/Advanced View expose semantic roles/names/states through standard native controls.
- UI Automation/keyboard/mouse converge on the same LogiMate action paths.
- Advanced Wheel View participates in keyboard focus order.
- Settings keyboard/accessibility focus auto-scrolls into view.
- Page context is exposed through a native status surface.
- Diagnostics and Engine Health expose the bridge runtime state.
- Windows app/system test binaries compile, Windows vet passes, amd64/arm64 GUI builds succeed and core host tests pass.
- Production runtime remains independent of `internal/openg27port`.

## Still pending before Stable

- Real Windows Narrator/UI Automation or Accessibility Insights validation.
- Full keyboard traversal on all pages.
- High Contrast and reduced-motion validation.
- 100% + scaled/mixed-DPI validation.
- Accessibility audit of secondary custom dialogs/setup/changelog.
- D5.7 real G25/G27/DFGT hardware evidence.
- HID stress/adverse-I/O gate.
- Authenticode signing and verification.

`docs/HARDWARE_CERTIFICATION.json` therefore keeps `uiAccessibilityValidated=false`.
