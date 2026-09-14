# D5.3 Release Readiness — 0.5.3-alpha

## Software gates

- D5.2 HID-first detection retained.
- First-run confirmed C294 G27 auto-promotion no longer requires a saved Modern preference.
- Explicit Legacy opt-out retained.
- G27 native switch bytes match OpenG27 exactly.
- Regression tests cover first-run auto-promotion, Legacy opt-out and G27 switch bytes.
- Windows x64 system/app test compilation and x64/ARM64 builds must pass before packaging.
- Production dependency on `internal/openg27port` remains forbidden.

## External gate

Physical G27 C294→C29B validation is still required. This release remains Alpha.
