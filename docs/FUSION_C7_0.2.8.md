# OpenG27 Fusion C7 — LogiMate 0.2.8-alpha

## Goal

C7 is the Path-C cutover checkpoint. The normal Logitech G27 Modern workflow no longer depends on a separately downloaded/running OpenG27 application. Windows Generic HID + LogiMate Native is the default runtime. External OpenG27 remains an explicit compatibility fallback and A/B oracle.

## What changed

- Added `FusionC7CoreGate`, a hardware-free pre-migration gate covering C1 force/calibration, C2 lg4ff golden bytes, C3 scheduler dry-run, C4 G27 parser vector, C5 profile matching and C6 Pino/player-control decoding.
- Added a live C7 PASS/WARN/BLOCK report in Engine Health and diagnostic exports.
- G27 Modern setup now validates LogiMate Native before destructive driver changes.
- G27 Modern setup no longer downloads, installs or auto-starts OpenG27.
- Existing OpenG27 installs are preserved untouched as optional fallback.
- System now exposes an explicit **OpenG27 Fallback** manager for status, optional install/update and A/B start.
- Starting the fallback retains the single-owner handoff and stops LogiMate native motor output first.
- Removed the user-facing OpenG27 auto-start toggle. Its persisted field remains load-compatible and defaults to false.

## C7 readiness gates

### PASS/BLOCK gates

1. C1–C6 hardware-free core self-test.
2. Exactly one actionable, model-confirmed and PnP-verified G27 target.
3. Current Generic HID / Modern mode.
4. No external OpenG27 process owning output.
5. No observed C4 live parser mismatch or parse error.
6. C3 scheduler dry-run.

### WARN gates

- No live C29B input report observed yet in the current session.
- Direct HID not connected yet while the target is otherwise valid.
- Known differences in old LogiMate C2 compatibility/test builders. C3/C6 use the translated OpenG27/lg4ff report bytes and are not blocked by those old builders.
- Physical G27 C3/C6 force, LEDs, reconnect/power-cycle, sleep/resume and crash-recovery matrix is not yet recorded as complete.

Warnings do not pretend to be stable hardware certification. `READY` in C7 means the current software/runtime cutover has no blocking condition; it does not mean LogiMate 1.0 certification is complete.

## Why external OpenG27 remains

OpenG27 remains useful as:

- a compatibility fallback,
- a hardware A/B comparison oracle,
- an import source for existing OpenG27 profiles/configuration,
- a regression reference while Path D consolidates translated and legacy LogiMate paths.

It is no longer a normal runtime dependency.

## Safety

- Native Wheel Output remains an explicit Experimental opt-in for motor-driving tests.
- External OpenG27 and LogiMate Native never intentionally own motor output simultaneously.
- Migration core validation happens before driver-package removal.
- C7 does not silently upgrade WARN hardware evidence to PASS.
- StableWheelID, PnP verification, model confirmation, generation ownership, watchdog, emergency stop and crash recovery remain authoritative LogiMate safeguards.

## Next step — Path D

Path C is functionally complete. D1 begins generalization beyond G27: move translated G27 assumptions behind wheel capability descriptors, validate G25/DFGT differences, and then consolidate duplicate translated/legacy implementations into one LogiMate-native wheel engine.
