# D5.7 Hardware Certification Assistant

**Release:** 0.5.7-alpha  
**Purpose:** turn the post-D5 Stable hardware matrix into evidence-driven, repeatable tests inside LogiMate itself.

## Why this phase exists

D0–D5 made Modern mode software-independent. D5.1–D5.6 then used a real G27 to improve detection and observability. The remaining Stable blockers are physical: real-wheel input/output behavior, reconnect/crash/suspend safety, migration rollback and per-model evidence.

D5.7 does **not** declare those items passed. It adds the tool that records them without confusing successful compilation with physical certification.

## What was added

- `internal/system/hardware_certification_windows.go`
  - per-wheel evidence store keyed by stable wheel identity + model;
  - required checks come from `wheelengine.RequiredHardwareChecks`;
  - PASS/FAIL with timestamp, app version, session/model/native PID and input source/layout;
  - strict/corruption-protected JSON persistence;
  - Markdown + JSON report export;
  - automatic evidence only for facts LogiMate can prove (currently native PID/PnP/model state).
- `internal/app/hardware_certification_windows.go`
  - guided physical certification assistant;
  - safe bounded effect tests reuse the existing Output Lease/Watchdog/Emergency Stop paths;
  - destructive/restart-spanning checks remain user-confirmed evidence instead of fake automation.
- Existing Wheel page Advanced View
  - live certification progress card with PASS/FAIL/PENDING for every required model check.

## G27 workflow

Recommended order:

1. Native Mode / PID
2. Steering
3. Pedals
4. Buttons & paddles
5. H-shifter
6. Range
7. Constant Force
8. Spring
9. Damper
10. Friction
11. Autocenter
12. Game FFB / Telemetry
13. Reconnect
14. USB removal
15. Process-kill / recovery
16. Suspend / resume
17. Modern ↔ Legacy ↔ Modern

Motor tests remain bounded by the existing experimental output safety profile. The assistant explicitly warns before a motor-driving test and neutralizes through the central Emergency Stop afterward.

## Stable boundary

Local evidence does not automatically set `docs/HARDWARE_CERTIFICATION.json` to validated. A Stable release still requires review and explicit repository evidence for G25, G27 and DFGT plus the global crash/migration/UI/HID-stress/Authenticode gates.
