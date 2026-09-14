# LogiMate Native Wheel Engine — OpenG27 integration plan

> **D0 / 0.3.0-alpha update:** LogiMate now has one typed Native Engine foundation for G25, G27 and Driving Force GT. Normal Modern setup/runtime does not require an external OpenG27 process. This document remains the provenance/history bridge explaining which internal algorithms were translated or parity-checked against upstream OpenG27.

Status: Path C and C8 are historical foundations. D0 is the current runtime architecture. External OpenG27 is optional import/A-B compatibility only; physical model certification continues in D1.

- C5: OpenG27 game-profile matching/import is translated into LogiMate Game Profiles + StableWheelID engine profiles; hard safety remains independent.
- C6: Wreckfest 2 Pino telemetry is implemented on loopback, with explicit G27 Game-FFB + RPM LED output through the Fusion scheduler and LogiMate safety policy.
- C7: LogiMate Native cutover is implemented. The Modern G27 migration runs C1–C6 core validation before destructive work and no longer downloads/launches OpenG27 automatically. External OpenG27 remains explicit fallback only.

## Current implementation status (0.3.0-alpha / Native D0)

- C1 pure Core port is present under `internal/openg27port` with MIT provenance and hardware-independent parity tests.
- Ported in C1: ForceScaling, ForceFrame/ForceMixer, AxisCalibration, PedalCalibration and RangeTracker.
- C1 remains isolated pure logic. C2 ports `Lg4ffReports` and performs byte-for-byte Protocol Shadow comparisons. C3 ports the OpenG27 scheduler/WheelOutput layer with deterministic dry-run and a G27-only bounded live test. C4 now ports `G27ReportParser` and mirrors `G27Device` lifecycle policy while LogiMate StableWheelID remains authoritative.
- C2 remains a legacy-builder shadow comparison. C3/C6 are the controlled translated live-output paths. C4 is read-only parser/device-lifecycle parity. C7 promotes the LogiMate Native stack to the normal G27 Modern runtime while keeping hardware warnings explicit. Engine Health refreshes live every 500 ms for parity/cutover counters and safety states.

- Stage A is implemented as an explicit Experimental opt-in: guarded range, G27 LED and momentary autocenter output plus single-owner OpenG27 interlock.
- Stage C is implemented: the translated scheduler, per-wheel safety config, hard force clamps, slew limiter, heartbeat/watchdog, zero-force ownership handoff and game-profile/Pino source integration are present.
- Spring/damper/friction now have separate watchdog-bounded hardware-test sessions on dedicated slots 1/2/3, still behind the Experimental hardware-validation gate.
- A deterministic four-channel force mixer and per-wheel Wheel-Engine profiles (Gentle/Balanced/Direct/Custom) now sit above the hard safety profile.
- Generation-safe worker ownership prevents stale FFB sessions from overwriting a newer effect handoff.
- Game process activation and Wreckfest 2 telemetry-driven force are implemented. Universal arbitrary-game DirectInput FFB compatibility remains a separate future research track.

## Goal

For the integrated G25/G27/Driving Force GT Modern path, LogiMate provides the core user-facing feature set itself: native HID connection, calibration, rotation range, force-feedback output, autocenter/damper/friction, rev LEDs, pedal tuning/remapping, per-game profiles, supported telemetry adapters and live monitoring.

The target runtime is:

```text
G25/G27/DFGT -> Microsoft Generic HID stack -> LogiMate Native Wheel Engine -> games/profiles/telemetry
```

No separate OpenG27 GUI/process is required for the normal D0 Modern runtime. Legacy Logitech/WingMan remains an optional reversible compatibility path; the preferred Modern path should keep Windows Memory Integrity enabled.

## Why this is feasible

OpenG27 is itself a user-mode G27 implementation. Its public MIT-licensed source separates device transport/configuration, the lg4ff command layer, calibration, a force engine, profiles, process detection, LEDs and telemetry. LogiMate already has Win32 HID discovery, native G27 input decoding, C294/C29B handling, per-device identity, migration safety and an established configuration/recovery layer.

That work is now implemented for the C7 G27 runtime; Path D focuses on consolidation, broader wheel capabilities and deeper hardware certification rather than starting from zero.

## Important limitation to keep explicit

Matching the current OpenG27 feature set is feasible without a custom kernel driver. That does **not** automatically mean every arbitrary DirectInput game can deliver its normal FFB to a driverless G27.

OpenG27's own documentation notes that its Wreckfest 2 game FFB is sourced from the game's UDP telemetry because the driverless wheel does not receive that game's normal DirectInput FFB path. OpenG27 also contains a Logitech SDK compatibility shim, but a game-side shim/bridge is a different compatibility mechanism from normal Generic HID.

Therefore LogiMate should split the promise:

1. **Native Wheel Engine parity:** all settings/output/profile/telemetry features OpenG27 currently exposes.
2. **Universal game FFB compatibility:** a separate later research track. It may need a game-specific telemetry adapter, explicit SDK shim, virtual device/FFB bridge, or eventually a signed driver solution. Never imply that Generic HID alone guarantees DirectInput FFB in every title.

## Architecture

### 1. `internal/wheelengine/transport`

Own one selected physical G27. Reuse LogiMate's stable physical `SelectedWheelID` and native HID discovery.

Responsibilities:
- C294 compatibility-mode detection and explicitly authorized C294 -> C29B native switch.
- Shared/exclusive handle policy.
- Serialized input/output access.
- reconnect/backoff and device-change handling.
- output write timeouts and cancellation.
- hard stop of effects on disconnect/shutdown.

No UI code sends raw reports directly.

### 2. `internal/wheelengine/g27`

Pure protocol layer with small tested functions for reports and decoding.

Initial output surface:
- rotation range (40/90..900 as hardware-valid range is confirmed),
- constant force,
- stop-all,
- autocenter spring set/enable/off,
- damper,
- friction,
- five-bit rev LED mask,
- native-mode switch.

Keep packet builders pure and unit-testable. Do not copy GPL source code into the MIT tree; protocol behavior can be independently implemented. If MIT OpenG27 code is ported or adapted, retain its required copyright/license notice in `THIRD_PARTY_NOTICES.md`.

### 3. `internal/wheelengine/config`

Per-physical-device configuration, never global-by-model:
- steering min/center/max calibration,
- throttle/brake/clutch min/max/inversion,
- pedal role mapping,
- steering/pedal deadzones,
- pedal sensitivity/curves,
- rotation range,
- master gain,
- autocenter/damper/friction,
- LED source/test level and blink interval,
- auto-connect preference.

Use LogiMate atomic writes and schema versioning. Add one-time import from `%APPDATA%\\OpenG27\\config.json` with a preview/diff before committing.

### 4. `internal/wheelengine/output`

A single owner of force and LED output.

Safety invariants:
- one writer per physical wheel,
- explicit max torque/gain clamp,
- slew/rate limiter,
- watchdog that stops force if source updates stall,
- zero-force on source/profile switch,
- zero-force and LEDs-off on disconnect/exit,
- no FFB output while device identity is ambiguous,
- hardware-test controls have short bounded duration and a prominent Stop button.

A dedicated worker can target roughly 150 Hz, matching the current OpenG27 architecture, but timing must be measured on real hardware before becoming a stable guarantee.

### 5. `internal/wheelengine/profiles`

Per-game profiles keyed by executable/process identity:
- profile name and executable matcher,
- rotation,
- autocenter/damper/friction,
- master gain,
- LED behavior,
- telemetry provider settings,
- optional game-FFB gain/inversion when that telemetry provider supplies force.

Profile activation must be visible in the UI and journaled in diagnostics. Manual override always wins until released.

### 6. `internal/wheelengine/telemetry`

Provider interface rather than hard-coding one game into the engine.

First provider: Wreckfest 2 Pino UDP telemetry (default localhost:23123), covering the current OpenG27 use case:
- live RPM -> shift LEDs,
- telemetry force -> constant-force source,
- optional auto-range only where the game/provider exposes trustworthy range data.

Future games get separate adapters and must not contaminate the generic wheel engine.

### 7. Optional Logitech SDK compatibility bridge

Treat the OpenG27 `LogiSdkShim` idea as an **optional per-game compatibility module**, not as a hidden injection mechanism. Anti-cheat-sensitive games and protected install folders make automatic DLL replacement unsafe and confusing.

If implemented:
- explicit opt-in per game,
- backup/restore every touched file,
- hash/version tracking,
- clear anti-cheat warning,
- one-click removal,
- no silent installation.

This module is not required before declaring parity with OpenG27's normal control-panel/settings workflow.

## UI destination

The Wheel page should eventually gain subviews/cards instead of launching another app:

- **Monitor** — current dashboard, raw/decoded HID, connection health.
- **Setup & calibration** — steering/pedals with guided capture and live graphs.
- **Force Feedback** — master gain, autocenter, damper, friction and bounded test actions.
- **Rotation & LEDs** — range, rev LED test/source/blink behavior.
- **Profiles** — process picker, profile list, active profile, import/export.
- **Telemetry** — provider, port/status, live RPM/force for supported games.

Advanced raw protocol diagnostics stay behind an Advanced section.

## Migration stages

### Stage A — native output foundation — complete
- Protocol builders and guarded write transport exist behind the Experimental output gate.
- Range/LED/autocenter and bounded effect tests exist.
- Historical stage only; OpenG27 is no longer the normal production runtime after C7.

### Stage B — native settings parity
- Bring calibration, deadzones, pedal roles/sensitivity, range, spring/damper/friction and LED settings into LogiMate.
- Import OpenG27 config with preview.
- External OpenG27 becomes optional for users who only need device settings.

### Stage C — native FFB worker and profiles
- Add watchdog/slew-limited force engine.
- Add process-based profiles and profile activation state.
- Hardware-validate constant force, spring, damper and friction on a real G27.

### Stage D — Wreckfest 2 telemetry parity
- Add Pino UDP adapter, telemetry force and RPM LEDs.
- Validate reconnect, game exit, race/menu transitions and safe force stop.

### Stage E — cutover — complete in C7
- OpenG27 profile import exists and the pinned upstream version remains the parity reference.
- A live PASS/WARN/BLOCK checklist separates software blockers from physical-validation warnings.
- External OpenG27 is no longer installed/launched by default and remains an explicit compatibility option.

### Stage F — runtime independence — complete for the normal G27 Modern path
- Default Modern setup has no external OpenG27 download/start requirement.
- Upstream attribution, provenance, import support and explicit fallback remain.
- Generic HID + LogiMate Native is the normal G27 Modern stack.

## Hardware gates before cutover

At minimum on a physical G27:
- cold boot, hotplug and USB re-enumeration,
- C294 -> C29B transition and reconnect,
- steering/pedal/button/H-shifter decode,
- 900/540/360-degree range changes,
- spring on/off and safe strength limits,
- damper/friction on/off,
- constant force both directions plus emergency stop,
- LEDs each bit and cumulative bar,
- per-device calibration persistence,
- profile activation/deactivation while games start/exit,
- Wreckfest 2 telemetry loss/recovery,
- HVCI on throughout Modern path,
- sleep/resume and LogiMate crash/restart with no stuck force.

No feature leaves Experimental until its physical test is recorded.

## Dependency policy

Stage E/C7 has passed the software cutover gate. LogiMate must still credit OpenG27 as the upstream MIT project for translated/derived components and must not overstate physical certification. External integration remains a fallback. Native and external engines must never drive the same wheel output simultaneously.
