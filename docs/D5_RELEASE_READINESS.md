# D5 Release Readiness — 0.5.0-alpha

## Software release decision

**READY FOR ALPHA / PREVIEW.**

D5 completes the planned Modern-mode independence work for the integrated G25/G27/Driving Force GT software stack.

## Required D5 gates

| Gate | Result |
|---|---|
| Production dependency graph excludes `internal/openg27port` | PASS |
| OpenG27 executable discovery/download/install/launch removed | PASS |
| External OpenG27 config no longer affects live native input | PASS |
| Legacy profile import remains offline/read-only compatibility | PASS |
| Native output ownership remains LogiMate lease/HID based | PASS |
| Legacy LGS/Profiler remains explicit alternative mode | PASS |
| D0–D4 software regression compile/tests | PASS in release build environment |
| Real G25/G27/DFGT physical certification | PENDING — Stable gate |
| Crash/USB/suspend/HID-stress hardware validation | PENDING — Stable gate |
| Stable Authenticode and full UI/accessibility certification | PENDING — Stable gate |

## Standalone dependency statement

For supported **Modern** workflows, the required runtime components are Windows plus LogiMate itself. No OpenG27 executable, Logitech Profiler/LGS installation, external calibration utility or separate FFB manager is required.

Legacy LGS/Profiler is needed only if the user intentionally switches to **Original Logitech / Legacy**.

## Packaging rule

Alpha/Preview may ship with pending hardware evidence. Stable must remain blocked until `HARDWARE_CERTIFICATION.json` and signing/UI/HID-stress gates are complete.
