# LogiMate Native / OpenG27 Fusion Audit

> **C8 / 0.2.9-alpha update:** the C7 native cutover remains the runtime baseline, but C8 centralizes output ownership/recovery, removes external OpenG27 config from native G27 input behavior, hardens identity/migration/telemetry/update safety, and makes D0 Native Engine Foundation the next architecture step. External OpenG27 remains explicit fallback/parity tooling only.


## Executive decision

Yes. The strongest end-state is one LogiMate application with the existing LogiMate Win32/Fluent UI and a native Wheel Engine that reaches feature parity with OpenG27 and then extends it. OpenG27 is MIT-licensed, so code may be used, modified, translated and redistributed provided its copyright and MIT permission notice are retained for copied or substantial derived portions.

The recommended strategy is **port/reference first, then native consolidation** — not embedding OpenG27's WPF UI and not permanently shipping two control panels.


## Fusion execution status

- **C1 / 0.2.1-alpha:** implemented. Pure hardware-independent Core parity layer, provenance ledger and self-tests. No translated HID output.
- **C2 / 0.2.2-alpha:** implemented. `Lg4ffReports` translation, upstream golden vectors and in-app Protocol Shadow comparison. Known LogiMate/OpenG27 packet differences are visible but not auto-promoted to hardware.
- **0.2.3-alpha UI checkpoint:** dialog reliability/live setup progress hardening only; C2 protocol behavior unchanged.
- **C3 / 0.2.4-alpha:** implemented. `FfbEngine`/`WheelOutput` scheduler port, deterministic dry-run and a separate bounded G27 live validation path behind LogiMate safety ownership.
- **C4 / 0.2.5-alpha:** implemented. G27 raw parser parity, lifecycle-policy comparison and live Engine Health parity counters; production input ownership remains LogiMate-native.
- **C5 / 0.2.6-alpha:** implemented. OpenG27 GameProfile/import parity translated into LogiMate profiles without touching hard safety.
- **C6 / 0.2.7-alpha:** implemented. Wreckfest 2 Pino adapter, explicit G27 Game-FFB and RPM LEDs through the Fusion scheduler.
- **C7 / 0.2.8-alpha:** implemented. LogiMate Native is the normal G27 Modern runtime; external OpenG27 is explicit fallback only. A live PASS/WARN/BLOCK report keeps physical validation gaps visible.
- **Path C:** functionally complete.
- **Path D:** next; generalize, consolidate and improve the native engine while continuing the physical certification matrix.

## Target architecture

`G25 / G27 / DFGT → Microsoft Generic HID → LogiMate Device Core → Input + Calibration → Native FFB Engine → Game/Telemetry Adapters → LogiMate UI`

Legacy remains a parallel supported branch:

`Wheel → Logitech LGS/WingMan → Profiler/DirectInput → LogiMate legacy management/diagnostics`

## Four possible integration paths

### A. External OpenG27 process
Keep the current downloaded OpenG27 application and launch/control it from LogiMate.

**Pros:** lowest risk; upstream remains independently updateable.  
**Cons:** duplicated UI, duplicated state, output-ownership conflicts, weakest end-user experience.  
**Use:** fallback during migration only.

### B. Embedded .NET sidecar/core service
Reference or adapt OpenG27 Core in a small .NET worker controlled by native LogiMate IPC.

**Pros:** fastest path to exact OpenG27 behavior; minimal translation errors.  
**Cons:** mixed Go/.NET runtime, IPC lifecycle, two crash domains, packaging complexity.  
**Use:** optional parity oracle / migration bridge, not preferred final architecture.

### C. Port OpenG27 Core C# → Go with provenance
Translate the proven pure/core components to LogiMate-native Go while retaining MIT attribution for substantial derived code.

**Pros:** reuses proven behavior and tests, one process/runtime, fits LogiMate architecture.  
**Cons:** translation must be validated byte-for-byte/numerically.  
**Use:** recommended near-term route.

### D. Clean native reimplementation using OpenG27 as behavioral reference
Reimplement protocols/algorithms from specifications and observed tests while using OpenG27 only as a reference/oracle.

**Pros:** cleanest architecture; easiest multi-wheel generalization.  
**Cons:** most engineering and hardware-validation work.  
**Use:** recommended long-term consolidation after parity is proven.

## Recommended C → D migration

1. Freeze LogiMate engine interfaces: WheelTransport, InputSource, FFBSource, TelemetryAdapter, GameProfileProvider.
2. Create a provenance table for every OpenG27-derived or translated component.
3. Port pure logic first: axis/pedal calibration, force scaling/mixer, range tracking and profile matching.
4. Port/verify `Lg4ffReports` using golden byte vectors; LogiMate's current builders become the comparison target rather than being replaced blindly.
5. Upgrade the LogiMate FFB scheduler toward OpenG27's high-rate model while retaining LogiMate generation ownership, safety caps, watchdog, crash marker and emergency-stop semantics.
6. Port Wreckfest 2 Pino decoding/listening as the first real TelemetryAdapter.
7. Port RPM LED source and per-car/telemetry auto-range behavior behind game-profile opt-in.
8. Build an OpenG27 profile/config import assistant into LogiMate; never silently overwrite existing LogiMate calibration.
9. Add a shadow/parity mode that compares generated reports/force frames without sending duplicate hardware output.
10. C7 removes external OpenG27 from the normal Modern setup path after software parity gates; it remains an explicit fallback while the physical certification matrix is completed.

## OpenG27 components to reuse/port

High value pure/core areas:
- AxisCalibration / PedalCalibration / PedalRoleMapping
- ForceFrame / ForceMixer / force scaling
- Lg4ffReports
- FfbEngine scheduling concepts
- WheelOutput safety concepts
- GameDetector / GameProfile / GameProfileStore
- RangeTracker
- RpmLedSource
- PinoTelemetry / Wf2Telemetry / TelemetryListener

## Components not to copy as architecture

- WPF `MainWindow` and profile editor: LogiMate's existing native Fluent UI remains canonical.
- G27-only device assumptions: LogiMate's Device Core is model-aware for G25, G27 and DFGT.
- OpenG27 persistence layout: data maps into LogiMate StableWheelID + GameProfile schema.
- Blind PID/path selection: LogiMate retains ContainerID/session grouping plus StableWheelID persistence.
- Any force output path that bypasses LogiMate's hard safety caps, generation ownership or recovery marker.

## What LogiMate should become better at than OpenG27

- G25 + G27 + DFGT in one engine.
- Legacy Logitech and Modern Generic-HID management in one application.
- Stable physical identity across C294/native re-enumeration.
- Integrated driver backup, rollback and LGS migration.
- Per-wheel calibration + per-wheel engine profile + per-game overrides.
- Stronger output ownership, generation-safe workers, watchdog and unclean-crash recovery.
- Raw HID validation and learning workflows.
- Adapter registry supporting multiple games rather than a single hard-coded telemetry path.
- Rich diagnostics/readiness/self-test layer.
- Future adapter ABI so game integrations can evolve without modifying the motor core.

## Licensing / provenance rules

When the first substantial OpenG27 code is copied or translated:
1. Preserve the OpenG27 MIT copyright + permission notice in `THIRD_PARTY_NOTICES.md`.
2. Add source-level provenance comments to translated/derived modules.
3. Keep a `docs/OPENG27_PROVENANCE.md` mapping upstream file/commit → LogiMate module.
4. Audit licenses of OpenG27 dependencies separately; do not assume MIT for HidSharp or any other dependency merely because OpenG27 itself is MIT.
5. Prefer copying protocol-independent tests/golden vectors only when their licensing provenance is recorded.

## Compatibility reality

Generic HID does not make arbitrary games send their DirectInput FFB stream to a user-mode LogiMate process. Games with telemetry/SDK output can be adapted directly. Broad legacy game compatibility may eventually require a separate signed virtual/DirectInput FFB bridge, which is a larger driver/security project and should not be mixed into the initial OpenG27-core port.

## Test strategy

- Golden lg4ff report byte vectors.
- OpenG27 ↔ LogiMate numeric force-mixer vectors.
- Golden Wreckfest Pino packet fixtures.
- Parser fuzzing and malformed UDP packets.
- G27 C294/C29B power-cycle and re-enumeration matrix.
- G25 C294/C299 and DFGT C294/C29A matrices.
- USB disconnect during active force; watchdog and startup recovery.
- OpenG27/LogiMate ownership-conflict injection.
- Shadow mode before any new adapter is allowed to drive hardware.

## Suggested product roadmap after 0.2.0

- **0.2.x:** stabilize game profiles, telemetry adapter API and hardware test matrices.
- **0.3.x:** port OpenG27 pure Core + lg4ff parity + Wreckfest Pino.
- **0.4.x:** native telemetry-driven FFB/LED/auto-range and OpenG27 config import.
- **0.2.8-alpha:** OpenG27 external app is now optional fallback only for the G27 Modern path.
- **0.3.x:** Path D generalization/consolidation and broader game adapters.
- **1.0:** complete supported-wheel hardware matrix, signed release, full UI Automation, external OpenG27 no longer required for normal Modern operation.
