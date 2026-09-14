# LogiMate D0 Native Engine Foundation — 0.3.0-alpha

**Status:** software foundation complete; physical hardware certification remains open.
**Date:** 2026-09-12
**Previous baseline:** 0.2.9-alpha / Fusion C8.

## Goal

D0 turns LogiMate from a G27-centered collection of proven native paths into one capability-driven native engine for the currently supported classic Logitech families: **G25, G27 and Driving Force GT**. The normal Modern workflow must not require Logitech Gaming Software/Profiler or an external OpenG27 process.

## What changed

### 1. Canonical native wheel engine

A new platform-independent `internal/wheelengine` package owns the typed runtime model:

- `ModelID`: `g25`, `g27`, `dfgt`, `compatibility`, `unknown`;
- `OperatingMode`: Modern/Legacy/Unknown;
- model descriptors and control/output capabilities;
- canonical `Device`, `InputState`, `OutputState` and `Health` state;
- a runtime `Manager` with generation/session tracking;
- a transport command boundary.

A PnP session change invalidates stale input/output state rather than silently carrying it to the new device generation.

### 2. Capability-driven model behavior

G25, G27 and DFGT now obtain native PID, C294 mode selector and supported features from one descriptor table:

| Model | Native PID | C294 selector | Clutch | H-shifter | Rev LEDs | Native FFB |
|---|---|---:|---:|---:|---:|---:|
| G25 | C299 | 0x02 | Yes | Yes | No | Yes* |
| G27 | C29B | 0x04 | Yes | Yes | Yes | Yes* |
| Driving Force GT | C29A | 0x03 | No | No | No | Yes* |

`*` Alpha/code support; physical output certification is still required before Stable.

### 3. One Modern migration path

The guided Modern migration is no longer split into a G27 special route and a generic route. All supported models use one journaled transaction:

1. verify the selected physical target;
2. run the D0 Native Engine core gate;
3. back up Profiler state and relevant legacy drivers;
4. optionally remove Profiler/LGS application components;
5. remove only verified bound legacy wheel packages;
6. re-enumerate through Windows Generic HID;
7. switch C294 to the model-specific native PID when needed;
8. verify the terminal target state;
9. commit only after postconditions pass, otherwise verify rollback.

### 4. One output ownership boundary

C8's central output lease and Windows mutex remain authoritative. D0 additionally funnels productive Logitech HID output through `nativeHIDTransport` instead of keeping separate FFB/range/LED/game writers.

The transport:

- serializes writes;
- opens the selected HID target without `FILE_SHARE_WRITE`;
- has a deadline/cancellation boundary;
- attempts `CancelIoEx` when a call exceeds its deadline;
- poisons/closes the handle and keeps higher-level ownership fail-closed when write completion cannot be proven.

The underlying classic Logitech compatibility call is still `HidD_SetOutputReport`. A future transport may promote overlapped `WriteFile` after real hardware validation; D0 deliberately does not replace a proven classic-wheel write primitive blindly.

### 5. Canonical input state

Direct-HID/WinMM samples are projected into the wheel engine with:

- wheel/session identity;
- source/layout;
- sample generation/time/validity;
- steering and pedal values;
- button/D-pad/gear/paddle state;
- connection and error state.

G25/G27/DFGT Direct-HID selection is correlated to the selected PnP target and stale samples do not survive a session change.

### 6. Native game/telemetry adapter identity

The Wreckfest 2 Pino runtime ID is now `wreckfest2-pino`. Existing saved profiles using `openg27-pino` are accepted as a legacy alias and normalized automatically.

The parser implementation remains legally attributed where code/behavior derives from OpenG27, but **runtime configuration no longer needs an OpenG27-named adapter**.

The native game-output path is capability-driven:

- G25/G27/DFGT may use the common guarded game-FFB path when Native FFB is enabled;
- RPM LEDs are emitted only when the model descriptor reports them (currently G27);
- LED-only mode never acquires a motor lease;
- Pino `FFBEnabled`, physics and player-control flags remain authoritative gates.

### 7. UI/setup reflects standalone Modern mode

The normal System page and first-run setup now describe **Modern / LogiMate Native** for all three supported models. The normal Modern action no longer presents OpenG27 as part of the workflow.

Historical OpenG27 import/fallback code remains for migration, provenance and A/B diagnostics, but it is optional and not required to operate a supported wheel in Modern mode.

## Standalone status after D0

### Normal Modern mode

Required runtime components:

- Windows;
- LogiMate;
- the connected supported Logitech wheel.

Not required:

- external OpenG27 application;
- Logitech Profiler/LGS application;
- separate calibration program;
- separate FFB utility;
- separate game-profile utility for the built-in adapter paths.

### Legacy mode

Legacy is intentionally preserved as an optional compatibility path. It can require the old Logitech driver/LGS stack because that is the point of Legacy mode. This does not make it a dependency of Modern mode.

## Validation completed in this build environment

- `go test ./internal/wheelengine/...` — PASS.
- `go test ./internal/openg27port/...` — PASS.
- Windows x64 `internal/system` test binary compile — PASS.
- Windows x64 `internal/app` test binary compile — PASS.
- Windows x64 application build — PASS.
- Windows x64 `go vet -unsafeptr=false ./...` — PASS.

The final release packaging reruns these gates plus installer/ARM64/package-integrity checks.

## What D0 does not claim

D0 cannot certify physical motor behavior from a Linux build/audit host. Stable support still needs the documented real-hardware matrix for:

- G25 C294↔C299 transition, controls, shifter and all native effects;
- G27 C294↔C29B, output/reconnect/crash matrix and LEDs;
- DFGT C294↔C29A, controls and native effects;
- USB removal, process kill, shutdown and sleep/resume while output is active;
- repeated/adverse HID I/O;
- Modern↔Legacy↔Modern rollback on real Windows;
- Authenticode Stable packaging.

## Exact next step — D1

D1 is no longer “invent a G25/DFGT engine”. The software-side generalization is already largely present. D1 is now **model certification and model-specific refinement**:

1. run the G25 hardware matrix and record protocol/input deviations;
2. run the DFGT hardware matrix and record deviations;
3. run the same G27 matrix against the unified transport;
4. encode any proven per-model differences in `wheelengine.Descriptor`/model adapters, not scattered UI branches;
5. add captured hardware fixtures/regression tests;
6. only then change a descriptor from `hardware-gate` to `validated`.

After D1, D2 can remove historical duplicate/reference runtime layers where equivalence is proven, while keeping mandatory OpenG27 provenance/license records.
