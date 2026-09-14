# LogiMate Master Audit & Plan-D Tracker

**Project:** LogiMate  
**Historical audit baseline:** 0.2.8-alpha — Fusion C7 / Native Modern Cutover
**Current development release:** 0.0.1-alpha · Build 011 — FFB Slot Activation & Adjustable Test Strength  
**Audit start:** 2026-09-12  
**Purpose:** This file is the authoritative hand-off document for future chats/releases. It records what is already implemented, what was verified, what is still open, and the exact next development step.

> **Rule for future work:** Before changing LogiMate, read this file first. Do not restart the roadmap from memory. Update this file in every release that changes audit status, architecture, safety behavior, or Plan-D progress.

---

## 1. Current project state

Path C, C8 and D0–D5 are complete on the software architecture path. **Current state: 0.0.1-alpha · Build 011 — real lg4ff slot activation on the Build-010 unified G27 HID session**. Modern mode is standalone; D5.3 keeps D5.2 HID-first discovery and closes the real-hardware first-run gap where an already confirmed C294 G27 could remain in compatibility mode solely because no `modern` preference had been persisted yet.

Implemented checkpoints:

- **C1 — Pure Core parity / 0.2.1-alpha:** force scaling, calibration, pedal calibration, range tracking.
- **C2 — lg4ff report parity / 0.2.2-alpha:** OpenG27 reference report builders and shadow comparisons.
- **C3 — FFB scheduler parity / 0.2.4-alpha:** ported scheduler/output behavior, dry-run, bounded G27 live test.
- **C4 — Device/report parser parity / 0.2.5-alpha:** G27 report parser shadowing and lifecycle policy.
- **C5 — Profiles/import / 0.2.6-alpha:** OpenG27 profile/config import and matching semantics.
- **C6 — Wreckfest 2 / 0.2.7-alpha:** Pino telemetry, RPM LEDs and player-control gating.
- **C7 — Native cutover / 0.2.8-alpha:** normal G27 Modern runtime uses Windows Generic HID + LogiMate Native; external OpenG27 is fallback/A-B oracle only.

The historical 100-point product audit in `docs/AUDIT.md` remains valid as project history. The audit started in this file is a **new Plan-D engineering audit**, not a continuation of the old numbering.

---

## 2. Development order from C7 onward

Do **not** jump directly from C7 to adding more wheel models.

The agreed sequence is:

1. **Freeze C7 as the audit baseline.**
2. **Plan-D engineering audit 1–100.**
3. **C8 — Reliability & Architecture Cleanup:** fix BLOCK findings and architecture defects that would make D0 unsafe.
4. **D0 — Native Engine Foundation:** create one typed wheel/device/capability/transport/input/output/safety architecture.
5. **D1 — Generalize beyond G27:** G25 and DFGT move onto the same capability-driven engine; physical validation remains mandatory.
6. **D2 — Unified Native Wheel Engine:** remove duplicate translated/legacy implementations after equivalence is proven.
7. **D3 — Adapter Ecosystem:** stable game/telemetry adapter interface with per-adapter health.
8. **D4 — Advanced FFB:** filtering, clipping telemetry, response shaping, per-effect tuning and safe model presets.
9. **D5 — Modern-mode Independence:** supported Modern workflows no longer depend on external OpenG27 data/runtime; Legacy remains first-class.

### D0 target architecture

The intended direction is approximately:

```text
WheelManager
├── DeviceRegistry
├── WheelDescriptor / Capabilities
├── WheelTransport
├── InputEngine
├── OutputEngine
├── SafetyController
├── ProfileManager
└── AdapterManager
```

Per-device runtime state should be explicit rather than inferred from display strings:

```text
WheelDevice
├── StableID
├── SessionID
├── ModelID
├── Capabilities
├── ConnectionState
├── InputState
├── OutputLease
└── Health
```

The engine should ask for capabilities (for example `HasHShifter`, `HasClutch`, `HasRevLEDs`, supported rotation/effects) instead of scattering `if G27`/`if G25`/`if DFGT` branches across the codebase.

---

## 3. New Plan-D audit structure

The new audit is split into ten batches:

| Batch | Scope |
|---|---|
| 1–10 | Architecture, ownership, state model, boundaries |
| 11–20 | Device discovery, StableWheelID, hotplug, reconnect, multi-wheel |
| 21–30 | Input engine, axes, pedals, buttons, paddles, D-pad, H-shifter, calibration |
| 31–40 | Output/FFB engine, scheduler, mixer, range, LEDs, effect lifecycle |
| 41–50 | Safety: emergency stop, crash, shutdown, disconnect, stale generation |
| 51–60 | Modern/Legacy migration, transaction journal, rollback, HVCI, backups |
| 61–70 | Profiles, games, telemetry, adapters, process/foreground gates |
| 71–80 | UI/UX state truth, settings dependencies, diagnostics, setup flow |
| 81–90 | Reliability/security: races, handles, atomic state, privileges, updater |
| 91–100 | Release readiness: install/update/portable/uninstall/recovery/hardware matrix |

Each finding uses:

- **Status:** PASS / WARN / BLOCK / NOT TESTABLE WITHOUT HARDWARE
- **Severity:** BLOCK / HIGH / MEDIUM / LOW
- **Evidence:** concrete file/function/line region
- **Risk:** why it matters
- **Required fix:** intended solution
- **Acceptance:** how the fix is proven

---

## 4. Baseline verification performed on C7

The untouched uploaded C7 release was checked before audit changes.

### Release integrity

`SHA256SUMS.txt` matched all original packaged artifacts:

- `LogiMate.exe` — OK
- `LogiMate-Setup-x64.exe` — OK
- `LogiMate-Portable-x64.zip` — OK
- `LogiMate-Source.zip` — OK
- `LogiMate-GitHub-Ready.zip` — OK

### Source/build checks

Performed from the included C7 source:

- `go test ./internal/openg27port/...` — **PASS** on Linux host.
- Windows amd64 `cmd/logimate` cross-build — **PASS**.
- Windows amd64 installer build with `-tags installer` — **PASS**.
- Windows arm64 validation build — **PASS**.
- Windows amd64 `go vet -unsafeptr=false ./...` — **PASS**.
- Installer `go vet` with `-tags installer` — **PASS**.
- Windows `internal/system` test binary cross-compilation — **PASS**.
- Windows `internal/app` test binary cross-compilation — **PASS**.

Windows-only test binaries cannot be executed on the Linux audit host. Physical USB/FFB behavior is also **NOT TESTABLE WITHOUT HARDWARE** in this environment.

---

# 5. Plan-D Engineering Audit 1–10

## A-001 — Architecture documentation contradicts C7 runtime

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN

**Evidence**

- `docs/ARCHITECTURE.md` still describes OpenG27 as a separate upstream engine that LogiMate downloads and says OpenG27 remains responsible for FFB/LED/profile features.
- `docs/FUSION_C7_0.2.8.md` states the opposite current runtime: LogiMate Native is the normal G27 Modern stack and external OpenG27 is fallback only.

**Risk**

Future development can follow an obsolete ownership model, re-introduce external-runtime assumptions, or implement D0 against the wrong boundary.

**Required fix**

Rewrite `docs/ARCHITECTURE.md` for the C7 state and clearly separate:

1. native production/runtime behavior,
2. `internal/openg27port` provenance/reference code,
3. external OpenG27 fallback/A-B oracle,
4. Legacy LGS/Profiler path.

**Acceptance**

No current architecture document may describe external OpenG27 as the normal C7 FFB/profile runtime.

---

## A-002 — Capability/descriptor layer does not exist yet

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — D0 REQUIRED

**Evidence**

- There is no `WheelDescriptor`, capability set, or equivalent typed model abstraction in `internal/`.
- Model behavior is distributed through `IsG25Model`, `IsG27Model`, `IsDFGTModel`, PID switches and model-specific branches.
- The source contains hundreds of G27-specific references; Path-D UI itself states that a capability system is still future work.

**Risk**

Adding G25/DFGT features now would multiply branches and duplicate protocol/input/output logic instead of creating a unified engine.

**Required fix**

Introduce a model descriptor registry before broadening output support. At minimum the descriptor must own:

- model ID / display name,
- supported VID/PIDs,
- native-mode selector,
- input parser,
- supported controls,
- range limits,
- LED support,
- supported FFB/effect commands,
- hardware-validation state.

**Acceptance**

New engine code consumes capabilities/model IDs rather than parsing model display strings.

---

## A-003 — `internal/system` is a multi-responsibility monolith

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — D0 REQUIRED

**Evidence**

- `internal/system/system_windows.go` is about 2,400 lines and contains device discovery, identity/state reconciliation, persistence, driver backup/restore, OpenG27 download/launch support, diagnostics and WinMM input.
- The full `internal/system` package also owns migration, native HID, FFB, telemetry, updater, profiles and reliability behavior.
- `internal/app/app_windows.go` is similarly very large and mixes UI dispatch, refresh lifecycle, device-change safety and many feature actions.

**Risk**

Ownership boundaries are difficult to prove. A change for one concern can modify shared global behavior in another, making safety regression review harder.

**Required fix**

D0 should split by responsibility rather than by historical release checkpoint. Suggested boundaries:

- `device` / registry + identity,
- `transport/hid`,
- `input`,
- `output/ffb`,
- `safety`,
- `migration`,
- `profiles`,
- `telemetry/adapters`.

**Acceptance**

Motor-output safety and device identity can be audited without reading updater, UI, telemetry and driver-management implementation in the same package/file.

---

## A-004 — Model/mode truth is stringly typed and can misclassify aggregate multi-wheel state

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN

**Evidence**

- `internal/system/wheels_windows.go` identifies models with display strings and `strings.HasPrefix` (`IsG25Model`, `IsG27Model`, `IsDFGTModel`).
- `aggregateLogicalWheelModel` can create combined strings such as `Logitech G25 + Logitech G27`.
- UI and several state decisions still call the model helpers on `State.WheelModel` instead of always consuming the selected `WheelDevice`.

**Risk**

A combined display summary beginning with `Logitech G25` satisfies `IsG25Model(...)`, even though the state actually represents multiple different wheels. Display text is therefore capable of becoming control logic.

**Required fix**

Add a typed `ModelID` enum/value and separate:

- selected-device model ID,
- aggregate display summary,
- human-readable label.

No safety or engine branch may use an aggregate display string.

**Acceptance**

A G25+G27 multi-wheel state has no single model ID and cannot pass G25- or G27-specific capability gates until a specific wheel is selected.

---

## A-005 — Native G27 input behavior still depends on external OpenG27 config when present

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — D5 BLOCKER

**Evidence**

- `internal/system/g27_shared_hid_windows.go` calls `loadOpenG27InputConfig()` inside the native G27 report parser.
- `internal/system/openg27_config_windows.go` reads `%APPDATA%\OpenG27\config.json` and uses OpenG27 pedal roles/calibration as live native LogiMate input semantics when the file exists.

**Risk**

Two systems with identical LogiMate settings can produce different native input values solely because an external fallback application's config file happens to exist. This weakens deterministic LogiMate ownership and D5 independence.

**Required fix**

Treat OpenG27 configuration as an explicit **import source only**. Imported values must be copied into LogiMate's own per-wheel schema and thereafter owned/versioned by LogiMate.

**Acceptance**

Deleting or changing `%APPDATA%\OpenG27\config.json` after import no longer changes normal LogiMate Native input behavior.

---

## A-006 — Multiple live output/protocol implementations remain active with known byte differences

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8/D0 REQUIRED

**Evidence**

- `internal/system/native_ffb_windows.go` builds and writes LogiMate's classic effect packets.
- `internal/system/fusion_c3_windows.go` and C6 route the translated `internal/openg27port` scheduler/report bytes to hardware.
- `internal/system/fusion_c2_windows.go` explicitly records known differences between the old LogiMate builders and OpenG27/lg4ff reference bytes, including LED, constant-force and autocenter sequence differences.
- Both old/native and translated paths are exposed as hardware tests.

**Risk**

There is not yet one authoritative protocol/output implementation. Fixing a packet, watchdog, effect slot or model rule in one path does not guarantee the other path changes with it.

**Required fix**

D0/D2 must converge on one protocol layer and one scheduler/output lifecycle. Reference vectors from `internal/openg27port` should remain parity tests/provenance, not a second independent production engine.

**Acceptance**

For each supported effect/command there is one production builder and one production output owner; parity tests compare it to reference vectors.

---

## A-007 — FFB and Autocenter do not share one exclusive motor-output lease

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — MUST FIX IN C8

**Evidence**

- FFB sessions are owned through the global `nativeFFB` generation/cancel state.
- `NativeOutputTestAutocenter()` uses the separate `nativeOutput` timer/status and does not acquire the `nativeFFB` generation or reject an active FFB session.
- `StartNativeConstantForceTest()` / condition tests stop prior `nativeFFB` sessions but do not neutralize/reject a currently active `nativeOutput` autocenter effect.

**Risk**

A user can start one motor-driving path and then start the other before its watchdog expires. Both can write motor-related commands to the same HID device under separate ownership models.

**Required fix**

Create one `OutputLease`/`MotorOwner` arbiter. Any motor-driving action must atomically acquire the same lease. Starting a new motor effect must either:

1. safely neutralize and release the old owner, then acquire, or
2. fail closed.

Range/LED commands should be classified separately from motor effects but still routed through the same device-session transport.

**Acceptance**

Automated tests prove that Autocenter + Constant/Spring/Damper/Friction/C3/C6 can never be active concurrently on the same wheel.

---

## A-008 — `NativeOutputRelease()` can cancel the Autocenter watchdog without sending a neutral command

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — MUST FIX IN C8

**Evidence**

- `internal/system/native_output_windows.go`: `NativeOutputTestAutocenter()` starts a two-second timer whose callback sends zero autocenter.
- `NativeOutputRelease()` calls `stopWatchdogLocked()`, marks output inactive, and does **not** send the zero-autocenter packet.
- `internal/app/app_windows.go`: disabling the Native Wheel Output setting calls `NativeOutputRelease()` directly.

**Risk**

Disabling the feature while Autocenter is active can remove LogiMate's scheduled neutralization while the wheel still retains the previously sent Autocenter command. The software state says inactive even though the device may still be applying the command.

**Required fix**

`Release` must never mean “forget state”. For a held motor-output lease it must be a fail-safe sequence:

1. neutralize/stop using the captured live transport/lease,
2. wait/bound completion,
3. then release timer/handle/owner state.

If neutralization fails, keep an explicit fault/crash marker and report that the hardware stop was not confirmed.

**Acceptance**

A test simulating `Autocenter -> NativeOutputRelease` must observe a zero/stop report before the lease/timer is discarded.

---

## A-009 — Emergency Stop is coupled to normal output validation and can be blocked by the condition it is supposed to recover from

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — MUST FIX IN C8

**Evidence**

- `NativeFFBEmergencyStop()` and `nativeOutputSafeStopSnapshot()` call `validateNativeOutputTarget(s)` before sending final stop packets.
- `validateNativeOutputTarget()` rejects output when external OpenG27 is running, when more than one supported wheel is attached, when current state/model/mode is no longer actionable, or when current Raw Input path correlation is not unique.

**Risk**

Those checks are correct for **starting a new output command**, but wrong for a best-effort **stop of an already-owned output session**. For example, if OpenG27 is started externally while a LogiMate motor command is active, the normal gate can refuse the emergency stop. Similar problems can occur during topology changes or stale UI state.

**Required fix**

Separate:

- `AcquireOutputTarget(...)` — strict validation for new output,
- `EmergencyNeutralize(lease)` — uses the already captured wheel ID/path/handle/generation and never refuses solely because ownership/topology became unsafe.

Emergency stop should invalidate all future writes first, then best-effort neutralize the captured lease.

**Acceptance**

Tests cover Emergency Stop after:

- OpenG27 appears,
- a second wheel appears,
- selected state becomes stale,
- device-change notification occurs,
- generation changes.

No stale worker may resume output afterward.

---

## A-010 — C7 source/release baseline is buildable and internally coherent enough to continue the audit

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED

**Evidence**

- Original packaged checksums verified.
- OpenG27-port unit tests pass on the audit host.
- Windows x64 application and installer cross-build.
- Windows ARM64 validation build succeeds.
- Windows-target vet succeeds.
- Windows system/app test suites compile successfully.

**Remaining limitation**

Compilation is not physical certification. Windows-only tests were not executed on the Linux audit host, and USB/FFB behavior requires real wheel hardware.

**Acceptance**

This is the accepted C7 baseline for Audit 11–20 and C8 remediation.

---

# 6. C8 mandatory scope from Audit 1–100

C8 is a **safety/reliability release**, not a feature release. The implementation order is:

1. **One OS-wide/per-wheel output owner:** single-instance/output lease, one serialized OutputEngine and no independent motor writers.
2. **Fail-safe neutralization:** Emergency Stop must bypass normal “new command” gates, use the captured lease target, and never clear recovery evidence unless neutralization + handle closure are confirmed.
3. **Bounded hardware lifecycle:** cancellable/bounded HID I/O, no 350 ms “assume stopped” hand-off, crash/shutdown/session-end/device-change recovery state machine.
4. **Recovery durability:** output markers are mandatory before motor force; startup retry is persistent; unresolved recovery blocks new output.
5. **Telemetry motor authorization:** LED-only means zero motor force; profile FFB enable + adapter FFB enable + fresh Physics + PlayerControl are all required.
6. **Device identity/discovery hardening:** replace port-only trust, fail closed on SetupAPI enumeration errors, never elevate synthetic Direct-HID evidence to PnP truth.
7. **Input correctness:** valid-first-sample contract, signed/clamped axis normalization, layout-bound pedal calibration, clean semantic button/D-pad model, malformed/stale health.
8. **Migration transactions:** durable journal errors are fatal, explicit final postconditions, verified rollback, startup discovery of unresolved migrations and real HVCI requested/effective state.
9. **Persistence/reliability:** config corruption is visible/recoverable, concurrent atomic writes are safe, future schema versions are write-blocked.
10. **UI/release truth:** remove false safety claims, make disable-output a verified neutralization action, align version/build/release pipeline, add signing/hardware attestations.

D0 starts only after these C8 blockers are closed and regression-tested.

# 7. Audit progress

| Range | Scope | Status |
|---|---|---|
| A-001–A-010 | Architecture / ownership | COMPLETE |
| A-011–A-020 | Device discovery / identity / multi-wheel | COMPLETE |
| A-021–A-030 | Input engine | COMPLETE |
| A-031–A-040 | Output / FFB engine | COMPLETE |
| A-041–A-050 | Safety / crash / recovery | COMPLETE |
| A-051–A-060 | Modern / Legacy migration | COMPLETE |
| A-061–A-070 | Profiles / games / telemetry / adapters | COMPLETE |
| A-071–A-080 | UI / UX state truth | COMPLETE |
| A-081–A-090 | Reliability / security | COMPLETE |
| A-091–A-100 | Release readiness | COMPLETE |

**Plan-D engineering audit:** **100 / 100 reviewed.**

**Development status:** C7 remains frozen. Next implementation target is **C8 Reliability & Architecture Cleanup**.

# 8. Plan-D Engineering Audit 11–20

## A-011 — Persisted “StableWheelID” is a USB-port identity, not a physical-device identity

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — MUST FIX IN C8

**Evidence**

- `internal/system/wheels_windows.go`: `stableWheelLocationID()` hashes only `DEVPKEY_Device_LocationPaths`.
- `DeriveStableWheelID()` falls back to the USB instance suffix/topology token.
- Neither path incorporates a device-unique hardware serial.
- `SaveWheelDevicePreference()` stores manual C294 model confirmation under that ID.
- `BuildWheelDevicesWithPreferences()` treats a preference found under that ID as `ModelConfirmed`.

**Risk**

The ID survives C294 ↔ native PID changes because it follows USB topology. But a *different* classic Logitech wheel plugged into the same physical USB port can receive the same ID. If both appear as ambiguous C294, the replacement wheel can inherit the previous G25/G27/DFGT confirmation and become authorized for the wrong model-specific native-mode selector.

**Required fix**

Introduce versioned identity with separate `PersistentID`, `PortID`, `SessionID/ContainerID`, optional serial/device-unique evidence, and `IdentityConfidence`. Manual C294 confirmation must not transfer solely because the USB port matches.

**Acceptance**

Confirm G27 as C294 on port X, remove it, attach a different C294-capable model on port X: old model confirmation must not authorize model-specific writes.

---

## A-012 — Native SetupAPI enumeration can return a partial topology as successful

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — MUST FIX IN C8

**Evidence**

`internal/system/native_windows.go`, `detectDevicesNative()`: an unexpected `SetupDiEnumDeviceInfo` error after at least one item causes the loop to `break`, then the function returns the already collected devices with `nil` error.

**Risk**

Two attached supported wheels can temporarily look like one trusted wheel if enumeration fails mid-scan. That can defeat the invariant that destructive mode changes require exactly one attached supported wheel.

**Required fix**

Unexpected enumeration errors must mark the topology incomplete/untrusted and block actionable operations. Partial records may be retained for diagnostics only.

**Acceptance**

Simulate an error after the first of two devices: scan becomes failed/incomplete, no wheel is actionable, native output/migration remain blocked.

---

## A-013 — Synthetic Direct-HID continuity device can become PnP-verified

**Status:** BLOCK  
**Severity:** HIGH  
**State:** OPEN — C8 SAFETY FIX

**Evidence**

`CollectState()` / `CollectStateFast()` synthesize `HID\\VID_046D&PID_C29B\\DIRECT` when normal discovery is temporarily empty but the Direct-HID handle is still open. `BuildWheelDevicesWithPreferences()` interprets native PID C29B as PnP verification, with no source marker distinguishing this synthetic record from a real SetupAPI node.

**Risk**

A continuity fallback intended for UI/input resilience can become proof for `CanChangeSelectedWheelMode()`. A stale/open HID handle is weaker evidence than a fresh PnP scan and must not authorize destructive operations.

**Required fix**

Add `DiscoverySource`/trust. Synthetic Direct-HID records may remain readable but must have `PnPVerified=false`; destructive actions require fresh trusted SetupAPI evidence.

**Acceptance**

A synthetic `...\\DIRECT` record alone can never make `CanChangeSelectedWheelMode()` true.

---

## A-014 — Raw-Input fallback identities can be persisted as stable selections

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 RELIABILITY FIX

**Evidence**

`devicesFromRawInput()` creates `PhysicalID = "raw:" + <path>` without stable parent/location identity. That can become `WheelDevice.ID`. `reconcileWheelSelection()` persists the only wheel unless the ID begins with `direct:`; there is no equivalent guard for `raw:`.

**Risk**

A transient PnP gap can write a session-only Raw Input path into `wheel.selected`. When canonical PnP identity returns, selection can fail closed and unnecessarily require manual reselection.

**Required fix**

Classify identity persistence quality. `raw:` and `direct:` are session-only and must never replace the authoritative persisted selection.

**Acceptance**

Raw-Input-only fallback provides diagnostics but never writes a transient ID to `wheel.selected`.

---

## A-015 — Raw Input discovery has no error/completeness channel

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN

**Evidence**

`EnumerateRawInputLogitechWheels()` returns only `[]string`: API failure returns `nil`, per-device query failures are skipped, and callers cannot distinguish healthy zero-device state from failed/partial enumeration.

**Risk**

Current output gates are mostly fail-closed, but reconnect diagnostics and future D0 identity decisions cannot reason about discovery confidence.

**Required fix**

Return paths plus error/completeness metadata and propagate discovery health into DeviceRegistry/state.

**Acceptance**

Raw Input API failure is visible in diagnostics and cannot be treated as authoritative absence.

---

## A-016 — Missing persisted target and unselected multi-wheel states fail closed

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE / WINDOWS TESTS PRESENT

**Evidence**

`reconcileWheelSelection()` preserves a missing stored target, refuses to guess among multiple wheels, auto-selects only a sole target, and keeps C294 confirmation per wheel. `multiwheel_windows_test.go` contains dedicated regression tests for these invariants.

**Remaining limitation**

Windows-tagged tests were cross-compiled in this audit environment, not executed here.

**Acceptance**

Keep these tests mandatory in C8 Windows validation.

---

## A-017 — G27 selected-wheel to Raw-HID correlation is exact and fail-closed

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE / WINDOWS TESTS PRESENT

**Evidence**

`rawPathInstanceID()` maps the Raw Input path to SetupAPI instance-ID form; `rawPathsForWheel()` matches selected interfaces exactly. With multiple G27s and no unique correlation, `ReadPreferredWheelInput()` returns an ambiguity error rather than choosing the first PID match. Native output similarly requires a unique HID path. `g27_shared_hid_windows_test.go` contains identical-wheel disambiguation coverage.

**Acceptance**

Preserve exact correlation first and never regress to “first PID match” in an ambiguous multi-wheel session.

---

## A-018 — G25/DFGT multi-wheel input discards available selected-device correlation

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — D0/D1 REQUIRED

**Evidence**

`ReadPreferredWheelInput()` lets G27 try exact selected-device correlation, but for G25/DFGT returns `Selection ambiguous` immediately when more than one wheel of the same family is present, before using the existing `rawPathsForWheel()` correlation helper.

**Risk**

D1 cannot claim unified same-model multi-wheel input. The classic shared reader is still a package-global singleton rather than a per-device session.

**Required fix**

Move G25/DFGT onto D0 per-device transport/input sessions and use exact selected-device HID correlation.

**Acceptance**

With two G25s/DFGTs, selecting A reads A only, selecting B reads B only; uncorrelatable state fails closed.

---

## A-019 — Native output remains intentionally machine/session single-wheel-only

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — D0 REQUIRED

**Evidence**

`validateNativeOutputTarget()` rejects all native output when `len(s.Wheels) != 1`, even if a selected wheel has an exact PnP/Raw-HID correlation.

**Assessment**

This is a correct conservative C7 safety restriction. It also proves output ownership is not yet truly per-device.

**Required fix**

Do not simply remove the one-wheel gate. D0 first needs a per-device OutputLease bound to selected identity + current session/path/generation. Driver-package migration may remain globally restricted.

**Acceptance**

Multi-wheel output is enabled only after stale sessions cannot write and exact device ownership is proven.

---

## A-020 — Physical identity/reconnect claims still require a real Windows hardware matrix

**Status:** NOT TESTABLE WITHOUT HARDWARE  
**Severity:** HIGH validation gate  
**State:** OPEN — REQUIRED BEFORE D1 RELEASE CLAIMS

**Evidence**

Synthetic unit coverage exists, but this audit host cannot physically verify actual Windows behavior across C294/native mode switching, unplug/replug, port moves, hubs, identical wheels, sleep/resume, rapid disconnect, ContainerID replacement and LocationPaths changes.

**Required validation matrix**

Capture SetupAPI instance IDs, ParentID, ContainerID, LocationPaths, Raw Input paths, derived LogiMate ID, selection before/after, model-confirmation trust and stale-handle behavior for each scenario.

**Acceptance**

D0/D1 identity design is not hardware-certified until this matrix demonstrates intended same-device and replacement-device behavior on real hardware.

---

# 9. Plan-D Engineering Audit 21–30

## A-021 — `JoyState` is not an authoritative per-device input sample

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — D0 FOUNDATION / C8 METADATA

**Evidence**

- `internal/system/system_windows.go`: `JoyState` contains axes, buttons and presentation/health fields, but no `WheelID`, `SessionID`, input-source enum, source generation, sample sequence, sample timestamp or explicit `SampleValid` flag.
- `ReadPreferredWheelInput()` can return Direct HID or WinMM and then mutates the returned state with `ApplyControlProfile()`.
- Pedal calibration is not applied into the same canonical state; consumers call `PedalPercent()` separately.

**Risk**

A caller cannot prove which physical/session device produced a stored sample, whether two samples came from the same input source/generation, or whether values are raw, parser-normalized or user-calibrated. D0 would otherwise keep layering semantics onto a presentation-oriented structure.

**Required fix**

Introduce a typed per-device `WheelInputState` (or equivalent) with at least:

- persistent wheel ID + current session ID,
- `InputSource` / `LayoutID`,
- source generation + monotonic sample sequence,
- sample timestamp + explicit validity/health,
- raw axes/buttons,
- canonical semantic controls,
- calibrated steering/pedals as derived values.

`JoyState` may remain a UI compatibility view generated from that authoritative state during migration.

**Acceptance**

Every live sample is traceable to one selected device/session/source/generation; calibration and UI cannot silently combine samples from different sources.

---

## A-022 — Connected Direct HID can be reported as `Found=true` before any real input sample exists

**Status:** BLOCK  
**Severity:** HIGH  
**State:** OPEN — MUST FIX IN C8

**Evidence**

- `readG27SharedInput()` returns `Found: true` with a fabricated centered steering/default-axis state when the HID handle is connected but no valid report has arrived yet.
- `readClassicSharedInput()` does the same for G25/DFGT.
- Button, steering, H-shifter and pedal learning workflows primarily gate captured samples on `j.Found`; they do not require “at least one real valid report from this current source generation”.
- A G27 state can also remain `Found=true` while marked `Malformed` after a later malformed report.

**Risk**

A user can enter calibration immediately after reconnect/open and save synthetic defaults or an unhealthy/stale sample as real hardware data. This can corrupt steering/pedal calibration or produce invalid learned controls without the workflow noticing why.

**Required fix**

Separate `Connected` from `SampleValid`. Learning/calibration must require a valid sample from the current source generation; preferably require a short stable sampling window for endpoint captures. Malformed state must not be accepted as a fresh calibration sample.

**Acceptance**

Tests prove that “handle connected, no report yet” and “latest report malformed” cannot be persisted by button/shifter/steering/pedal learning.

---

## A-023 — Pedal calibration is keyed to wheel/mode, not to the actual input layout/source used to learn it

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8/D0 REQUIRED

**Evidence**

- `ReadPreferredWheelInput()` prefers Direct HID but automatically falls back to WinMM when Direct HID is not available.
- `State.Pedals` is loaded through `ReadPedalMappingForWheel(dataDir, wheelID, model, mode)`; the key contains wheel/model/mode but no Direct-HID-vs-WinMM source/layout identifier.
- Direct G27 input exposes LogiMate-normalized semantic Y/Z/R pedal channels, while WinMM axis assignment can differ by driver generation/configuration.
- The UI then applies the same stored `PedalMapping` via `PedalPercent()` to whichever source `ReadPreferredWheelInput()` returned.

**Risk**

A mapping learned on Direct HID can be applied to WinMM after a fallback (or vice versa), causing Gas/Brake/Clutch to use the wrong axis or calibration range while still looking like a valid stored profile.

**Required fix**

Prefer one canonical semantic input layout before calibration. Until that exists, persist an explicit `InputLayoutID`/schema version with the calibration and invalidate/re-map it when the live source layout changes.

**Acceptance**

Force Direct-HID→WinMM fallback after calibration: semantic Gas/Brake/Clutch must remain correct or the old mapping must be rejected with a clear recalibration requirement.

---

## A-024 — `AxisValues()` has unsigned-underflow normalization that can turn below-minimum input into 100%

**Status:** BLOCK  
**Severity:** HIGH  
**State:** OPEN — MUST FIX IN C8

**Evidence**

`internal/system/pedals_windows.go`, `AxisValues()`:

```go
x := float64(v-min) / float64(max-min)
```

`v`, `min` and `max` are `uint32`. When `v < min`, `v-min` wraps before conversion to `float64`; the subsequent `x > 1` clamp produces `1.0`, not `0.0`. `StrongestMovedAxis()` uses this helper to decide which physical axis a pedal belongs to.

**Risk**

A controller reporting a value slightly below its advertised/min baseline can appear as a full-scale movement. Pedal learning can select the wrong axis or reject the intended one.

**Required fix**

Clamp before subtraction (or perform signed/float conversion before subtraction), matching the defensive behavior already used elsewhere in the UI/calibration code. Add below-min and above-max regression vectors.

**Acceptance**

For `min=100`, `max=900`: values 50/100/900/950 normalize to 0/0/1/1 respectively; `StrongestMovedAxis()` no longer sees unsigned wraparound as motion.

---

## A-025 — Steering calibration accepts impossible geometry and arbitrarily tiny travel

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 CALIBRATION HARDENING

**Evidence**

- `SaveSteeringCalibration()` validates only configured degree range and that Left/Center/Right are not exactly equal.
- It does not require Center to lie between the two endpoints.
- It does not enforce a sensible minimum raw span between center/endpoints or total travel.
- `learnSteering()` captures one snapshot per point rather than a stability/median window.

**Risk**

Three distinct jitter values can be saved as “900°”, or an incorrectly captured center can lie outside the endpoint interval. `steeringDegrees()` then clamps a malformed geometry into apparently valid degree output, magnifying noise and hiding a bad calibration.

**Required fix**

Validate ordering for both normal and reversed axes, minimum endpoint/half-span, center plausibility and capture stability. Report why a calibration is rejected instead of storing it.

**Acceptance**

Unit tests reject center-outside-range, tiny-span and unstable endpoint cases while preserving valid normal and reversed-axis calibrations.

---

## A-026 — Classic G25/DFGT `Buttons` mask includes encoded D-pad/payload bits as if they were logical buttons

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8/D1 INPUT SEMANTICS

**Evidence**

- `parseClassicNativeReport()` constructs `j.Buttons` directly from the first three payload bytes.
- The low nibble of `report[base]` is separately decoded as the D-pad direction code.
- Therefore the same D-pad encoding bits also appear inside the generic `Buttons` bitmask.
- Button learning uses `FirstPressedButton(before, after)` on that mask.

**Risk**

D-pad direction/neutral codes and other non-button payload fields can appear as pressed button bits, contaminate the generic button grid, and be accidentally learned as a wheel/shifter button. This is especially unsafe to generalize to D1 without a model-specific semantic map.

**Required fix**

Keep raw payload bytes separate from a semantic `ButtonMask`. Build model-specific button masks only from validated physical-button bits; represent D-pad solely as hat/POV semantics unless explicitly exposed as logical directional controls.

**Acceptance**

A neutral or changed D-pad does not create/clear unrelated button bits, and button-learning tests cannot learn a D-pad encoding bit as a normal button.

---

## A-027 — G25/DFGT malformed Direct-HID reports are silently dropped while the previous state remains apparently usable

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 INPUT HEALTH

**Evidence**

- `classicSharedReadLoop()` simply `continue`s when `parseClassicNativeReport()` rejects a report.
- Unlike the G27 reader, the classic reader has no malformed counter/timestamp/status.
- If an earlier valid sample exists and the handle remains open, `readClassicSharedInput()` can continue returning that previous sample and later label it only `Idle`.

**Risk**

A changed/invalid report layout, truncated traffic or parser mismatch can freeze G25/DFGT controls at the last valid state without exposing that incoming reports are malformed. Calibration/diagnostics cannot distinguish genuine idle from parser failure.

**Required fix**

Give every Direct-HID reader the same health contract: malformed count/time, current-generation valid-sample flag, last-good sequence/time and an explicit malformed/degraded state. Do not present stale semantic controls as healthy while malformed traffic continues.

**Acceptance**

After a valid classic sample followed by malformed reports, diagnostics show `Malformed/Degraded`; learning refuses the sample until a new valid report arrives.

---

## A-028 — G25 H-shifter decoding is threshold-based and not physically certified

**Status:** NOT TESTABLE WITHOUT HARDWARE  
**Severity:** HIGH validation gate  
**State:** OPEN — D1 HARDWARE GATE

**Evidence**

- `parseClassicNativeReport()` classifies G25 H-pattern positions using fixed X/Y byte thresholds.
- No hysteresis/debounce is applied at gear boundaries.
- `GearSignature()` reduces learned G25 analog X/Y to coarse 4-bit buckets.
- `docs/SUPPORTED_WHEELS.md` and `docs/TESTING.md` already state that G25 semantic shifter parsing requires physical validation.

**Risk**

Real potentiometer tolerance/wear can place a lever close to thresholds, causing neutral/gear chatter or model/revision-specific misclassification. Software-only vectors cannot certify those thresholds.

**Required fix**

Use real G25 raw-report captures across all gears, neutral transitions and worn/tolerant units. Add hysteresis/stability if the native semantic decoder remains enabled; learned per-wheel signatures should be preferred where appropriate.

**Acceptance**

Physical matrix records stable Neutral/1–6/R detection across repeated transitions without chatter; raw captures become regression fixtures.

---

## A-029 — WinMM input telemetry is process-global instead of per selected controller/session

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — D0

**Evidence**

- `winMMTelemetry` is one global structure containing last error, reconnect count and report-rate counters.
- `pollWinMMJoystick()` updates it for whichever WinMM ID is currently polled.
- `InvalidateInputCaches()` clears only the device-capability cache, not the telemetry owner/session.

**Risk**

After switching controllers, reconnecting, or changing fallback source, the displayed report rate/reconnect/error history can belong to a previous controller. Today this is mostly diagnostic contamination; D0 health decisions must not inherit process-global telemetry.

**Required fix**

Key telemetry by selected device/input session (or store it inside the per-device input session) and reset/rotate generation on source changes.

**Acceptance**

Switching from controller A to B cannot carry A's reconnect/error/rate counters into B's input state.

---

## A-030 — Input profile read failures silently degrade to empty calibration/mappings

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8 RELIABILITY

**Evidence**

- `readInputProfileFile()` ignores `json.Unmarshal` errors and returns the default/partially decoded structure.
- `ReadPedalMapping()` / `ReadPedalMappingForWheel()` return an empty mapping on JSON corruption without surfacing an error.
- Writes are atomic, which reduces corruption risk, but read-time corruption/schema damage is not represented in `State` or diagnostics as a profile fault.

**Risk**

A damaged or manually edited profile can make learned controls/pedals silently “disappear”. The user sees changed behavior rather than an actionable corruption/recovery message, and a subsequent save can overwrite useful data with a new partial state.

**Required fix**

Return structured load status/errors, keep last-known-good/backup recovery where practical, validate schema/version, and prevent silent overwrite of a corrupt source file until recovery is explicit.

**Acceptance**

Malformed `input-profiles.json`/`pedals.json` produces a visible diagnostic/recovery state; LogiMate does not silently treat corruption as “no calibration configured”.

---

# 10. Plan-D Engineering Audit 31–40 — Output / FFB Engine

## A-031 — FFB session hand-off can time out while the old hardware writer is still alive

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

`stopNativeFFBSession()` closes the cancel channel but waits only 350 ms for `done`. HID output calls are synchronous, so a blocked writer can survive that timeout. A new test/condition/C6 session may then create a new handle while the old goroutine still owns the previous one.

**Risk**

Generation counters prevent stale status updates, but they do not revoke an OS handle or prevent the old goroutine from writing to the motor. This can create two LogiMate writers even inside one process.

**Required fix**

Replace timeout-based ownership hand-off with one serialized OutputEngine/lease. A new motor session must not start until the previous lease is conclusively released; a stuck writer must put the engine into a hard fault state.

**Acceptance**

Fault-inject a blocked HID write. A second session must be refused, not started, until the first hardware lease is confirmed closed.

---

## A-032 — Motor-output start APIs report success before HID startup has actually succeeded

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8

**Evidence**

`StartNativeConstantForceTest()`, condition-effect starts and Fusion C6 mark runtime status active and return before the worker has opened the HID handle and completed its first output write.

**Risk**

The UI/caller can report “started” even though the worker immediately fails to open/write. Safety state, recovery markers and user expectations temporarily disagree.

**Required fix**

Add a bounded startup handshake: open target, establish lease, write a known-safe initial state/first command, then return success. Startup failure must be synchronous to the caller.

**Acceptance**

Simulated open/write failure is returned by the start call and no `Active=true` state remains.

---

## A-033 — Crash-recovery marker creation errors are ignored for active motor sessions

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

`MarkRuntimeOutputActive(...)` is called with `_ =` in constant-force, condition-effect and Fusion C6 output paths.

**Risk**

Motor output can become active even when `runtime-output.pending` was never durably written. A later crash then leaves startup recovery with no evidence that force may have been active.

**Required fix**

Recovery-marker persistence must be part of acquiring the motor-output lease. If durable marker creation fails, do not start force. Surface the error.

**Acceptance**

Make the data directory unwritable: every motor-output start must fail before any non-neutral motor command is sent.

---

## A-034 — Release can clear the recovery marker without proving the output worker stopped

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

`NativeFFBRelease()` waits via the bounded 350 ms stop path and then unconditionally clears the runtime marker. Worker-side safe-stop also clears it without requiring the stop report itself to succeed.

**Risk**

A blocked or failed HID stop can leave an output writer or residual effect alive while the only crash-recovery evidence is removed.

**Required fix**

Clear the marker only after confirmed neutralization + handle closure. If confirmation is impossible, preserve a fault/recovery marker and block new output.

**Acceptance**

Inject stop-write failure and blocked handle close: marker remains, engine enters faulted state, and new motor output is rejected.

---

## A-035 — Native engine profile application is not fully transactional

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8

**Evidence**

`ApplyNativeEngineProfile()` applies hardware rotation first and persists the selected profile second. If `SetActiveNativeEngineProfile()` fails, hardware was changed while the persistent active profile remains old.

**Risk**

After restart/diagnostics the stored profile can disagree with the physical wheel range that was last commanded.

**Required fix**

Use a small transaction: stage desired profile, apply hardware, persist commit; if commit fails, restore previous hardware range or retain an explicit pending/fault state.

**Acceptance**

Force persistence failure after a successful range command and verify hardware/state are rolled back or the operation is visibly incomplete.

---

## A-036 — Effect builders, slot allocation and mixer safety caps are well bounded

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN SOFTWARE

**Evidence**

Classic constant-force/condition builders validate slots and force ranges. `MixNativeEffects()` clamps per-effect requests and applies per-wheel hard ceilings. Existing tests cover packet builders/mixer behavior.

**Risk**

This materially reduces malformed-command and accidental-overdrive risk, although it does not replace physical protocol validation.

**Required fix**

Keep builders pure and capability-driven when moved into D0; extend golden vectors per model.

**Acceptance**

All existing packet/mixer unit tests remain green and D0 adds per-model vectors.

---

## A-037 — Synchronous HID writes have no hard I/O timeout or cancellation primitive

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 SAFETY

**Evidence**

`hidSetOutputReport()` is invoked synchronously from output workers. Cancellation channels/watchdogs are checked only when execution returns to the loop.

**Risk**

If a driver/device call blocks, a software watchdog cannot guarantee stop timing and shutdown can outlive the intended safety window.

**Required fix**

Move hardware I/O behind a transport with bounded overlapped I/O/cancellable operations where Windows supports it, plus lease faulting when the deadline is exceeded.

**Acceptance**

A deliberately stalled transport reaches a bounded fault deadline and prevents subsequent output ownership until recovery.

---

## A-038 — Native FFB/profile JSON corruption silently falls back to defaults

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8 RELIABILITY

**Evidence**

`ReadNativeFFBConfig()` returns defaults on unmarshal failure; native engine profile loading also ignores JSON errors. Save paths can then write new data over an unreadable source.

**Risk**

A corrupted safety/profile file changes behavior without a clear diagnostic and can destroy recoverable configuration.

**Required fix**

Return structured load health, preserve corrupt files, use last-known-good/snapshot recovery and refuse silent overwrite.

**Acceptance**

Malformed native FFB/profile files produce an explicit diagnostic and are not silently replaced on the next save.

---

## A-039 — Native output protocol still requires physical model certification

**Status:** NOT TESTABLE WITHOUT HARDWARE  
**Severity:** HIGH validation gate  
**State:** OPEN — HARDWARE GATE

**Evidence**

Software/golden-vector tests verify packet construction, but C7 itself documents that range, LEDs, constant/spring/damper/friction and reconnect behavior are not fully recorded on physical G25/G27/DFGT hardware.

**Risk**

A byte-perfect reference comparison cannot prove device revision behavior, motor direction, stop semantics or tolerance.

**Required fix**

Run the documented low-force hardware matrix and retain captures/results per model/revision.

**Acceptance**

Signed-off hardware evidence exists for every enabled output capability before it loses Experimental status.

---

## A-040 — Range, LEDs, autocenter and FFB still bypass one authoritative output command queue

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8/D0 REQUIRED

**Evidence**

`native_output_windows.go`, native FFB workers and Fusion C6 open/use output handles independently. There is no single command queue/lease covering every hardware-output class.

**Risk**

Ordering and ownership are only conventionally coordinated. This is the root architecture behind A-007/A-031 and will get worse with more models.

**Required fix**

D0 OutputEngine owns one per-device transport/lease and serializes range, LEDs, effect slots, autocenter and emergency-neutral commands.

**Acceptance**

Code search shows no production path can write wheel output except through the per-device OutputEngine.

---


# 11. Plan-D Engineering Audit 41–50 — Safety / Shutdown / Recovery

## A-041 — Startup recovery clears the pending-output marker even when emergency neutralization fails

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

In `msgStateReady`, `NativeOutputEmergencyStop(rs)` has its error discarded and `ClearRuntimeOutputMarker()` is then called unconditionally; the UI notice says slots were stopped.

**Risk**

A failed recovery attempt is recorded to the user as success and the evidence for retry is destroyed.

**Required fix**

Only clear after verified stop. On failure retain marker, show a blocking safety notice and retry when a valid target becomes available.

**Acceptance**

Fault-injected stop failure leaves marker intact and the UI reports recovery failure, not success.

---

## A-042 — Pending-output recovery is one-shot and is not retried when prerequisites are initially unavailable

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

`runtimeRecoveryChecked` is set on the first state-ready event. If a marker exists but the wheel is disconnected, mode is not Generic, or OpenG27 is running, no recovery is attempted later in that process.

**Risk**

The user can reconnect the wheel later while a previous residual effect may still exist, but LogiMate considers recovery already checked.

**Required fix**

Model recovery as a state machine tied to the marker/target generation. Retry on relevant PnP/mode transitions until confirmed neutralized or explicitly acknowledged as impossible.

**Acceptance**

Start with marker + wheel absent, then reconnect: recovery runs automatically before any new output is enabled.

---

## A-043 — A panic inside `WM_DESTROY` can skip the remainder of motor shutdown

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

`wndProc` wraps every message in one broad `recover()`. If any call during `WM_DESTROY` panics, recovery returns from the window procedure and later stop/release/timer/quit steps are skipped.

**Risk**

A UI panic must never be allowed to abort the safety-critical shutdown sequence.

**Required fix**

Separate fail-safe shutdown into a no-panic/idempotent routine, invoke it in a defer dedicated to process/window teardown, and recover individual non-critical cleanup steps rather than the whole sequence.

**Acceptance**

Injected panics before/inside cosmetic cleanup still execute hardware neutralization and output-lease release.

---

## A-044 — Windows session-end/power-shutdown messages do not have an explicit output-safety path

**Status:** BLOCK  
**Severity:** HIGH  
**State:** OPEN — C8 SAFETY

**Evidence**

The Win32 layer handles `WM_DESTROY`/`WM_CLOSE` but has no dedicated `WM_QUERYENDSESSION`, `WM_ENDSESSION` or power-transition output-neutralization handler.

**Risk**

Logoff/shutdown/sleep paths can differ from a normal user close; relying on eventual destruction is too weak for motor output.

**Required fix**

Handle session end and relevant power transitions by atomically blocking new output and invoking best-effort direct neutralization before acknowledging termination/suspend.

**Acceptance**

Windows shutdown/logoff/sleep tests demonstrate no new output after transition begins and confirmed neutralization where the OS permits.

---

## A-045 — PnP topology changes ignore emergency-stop failure and then release output state

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

`WM_DEVICECHANGE` calls `_ = NativeOutputEmergencyStop(stopState)` and immediately `NativeOutputRelease()`. Release itself can clear state/markers without confirmed stop.

**Risk**

Exactly when device identity is changing, LogiMate can lose both the error and the evidence that output may still be unsafe.

**Required fix**

On topology change revoke the output lease first, attempt direct neutralization against the captured leased path/fingerprint, preserve fault marker on failure, then refresh discovery.

**Acceptance**

Unplug/re-enumeration fault injection cannot transition to “safe/inactive” merely because stop returned an error.

---

## A-046 — Device-change handling proactively stops output before refreshing identity

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE

**Evidence**

`WM_DEVICECHANGE` checks both native output and native FFB activity before invalidating caches/refreshing the device model.

**Risk**

The ordering is directionally correct and should be retained when the stop semantics are hardened.

**Required fix**

Keep “revoke output before rediscovery” as a D0 invariant.

**Acceptance**

Regression test asserts output lease revocation is first action on topology-generation change.

---

## A-047 — Runtime output recovery marker lacks a strong target/session fingerprint

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8/D0

**Evidence**

The marker stores only `Active`, `Effect`, `WheelID` and timestamp. WheelID currently has the port-identity weakness from A-011; no raw path, native PID, session generation or descriptor fingerprint is retained.

**Risk**

Recovery can target the wrong physical device after re-enumeration/replacement or be unable to prove that it neutralized the same hardware session.

**Required fix**

Persist an output-lease fingerprint: strong device identity evidence, selected raw path/PID, model ID, session/generation and command class; validate it conservatively during recovery.

**Acceptance**

Replacing a wheel on the same USB topology cannot cause an old marker to authorize commands to the replacement device.

---

## A-048 — Native FFB has conservative gain/slew/watchdog limits and OpenG27 coexistence checks

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN SOFTWARE

**Evidence**

C7 caps test force/effect gains, applies slew limiting, uses a watchdog and periodically checks for the standard OpenG27 process name.

**Risk**

These controls reduce risk in normal execution, though they do not solve blocked I/O or ownership identity issues.

**Required fix**

Retain caps as policy above the D0 transport; make ownership independent of process-name heuristics.

**Acceptance**

Safety caps remain enforced by pure tests after engine consolidation.

---

## A-049 — State invariants do not detect all possible LogiMate-internal dual-output ownership

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8

**Evidence**

`ValidateStateInvariants()` checks important external/FFB combinations but there is no single lease state proving `nativeOutput` and native FFB/C6 cannot own hardware concurrently.

**Risk**

Diagnostics/readiness can show a healthy state while two internal output mechanisms are independently active.

**Required fix**

Replace inferred invariants with one authoritative per-device lease owner/mode; expose it in diagnostics and make impossible states unrepresentable.

**Acceptance**

A test attempting two output owners fails acquisition and readiness reports the actual lease holder.

---

## A-050 — Kill/crash/USB-yank/suspend stuck-force behavior is not physically certified

**Status:** NOT TESTABLE WITHOUT HARDWARE  
**Severity:** BLOCK validation gate  
**State:** OPEN — HARDWARE GATE

**Evidence**

Software inspection cannot prove device-side force decay when the process is terminated, Windows sleeps, USB is yanked, the driver stalls or power cycles.

**Risk**

This is the final safety property for a motor-driving application.

**Required fix**

Run a documented destructive/fault-injection hardware matrix at low force with an external emergency path and record results.

**Acceptance**

No stable/non-experimental motor-output claim until every crash/power/disconnect scenario is signed off.

---


# 12. Plan-D Engineering Audit 51–60 — Modern / Legacy Migration

## A-051 — Mode migrations can commit without verifying the requested final device state

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 MIGRATION

**Evidence**

`SetupModern*Tracked()` and `SetupLegacyTracked()` perform steps and invalidate caches but do not finish with a mandatory fresh state collection that proves the same selected physical wheel reached the requested driver/native mode.

**Risk**

A partially converged PnP/driver state can be reported as successful and persisted as the user preference.

**Required fix**

Define explicit transaction postconditions (target identity, driver binding, mode, required PID, profiler/HVCI expectations) and commit only after fresh enumeration proves them.

**Acceptance**

Fault-inject delayed/failed PnP convergence: migration remains incomplete/failed and does not persist success.

---

## A-052 — Migration journal durability errors are ignored after journal creation

**Status:** BLOCK  
**Severity:** HIGH  
**State:** OPEN — C8 MIGRATION

**Evidence**

`runTrackedSetupAdminAction()` ignores `MarkMigrationRunning()` errors. `AppendMigrationStep()`, `CompleteMigration()` and `CancelMigration()` discard write failures internally.

**Risk**

A destructive admin transaction can continue after its recovery/audit journal stops being durable, defeating the purpose of the transaction record.

**Required fix**

Make every journal mutation return/propagate an error. Before destructive steps, journal durability is a hard prerequisite; after a failure, enter safe abort/rollback.

**Acceptance**

Make the Migrations directory unwritable after `BeginMigration`: destructive driver operations must not proceed.

---

## A-053 — Rollback is best-effort and has no verified terminal “rollback succeeded” postcondition

**Status:** BLOCK  
**Severity:** HIGH  
**State:** OPEN — C8 MIGRATION

**Evidence**

`rollbackModern()` attempts driver/profiler/preference restoration and collects notes but does not fresh-enumerate and prove the original snapshot was restored. Preference restoration errors are ignored.

**Risk**

A failed migration may leave Windows in a third, unknown state while the rollback log sounds partially successful.

**Required fix**

Give rollback its own transaction result and postconditions; unresolved rollback must create a persistent recovery-required state that blocks further mode changes.

**Acceptance**

Injected restore failure produces `rollback_failed/recovery_required` and cannot be mistaken for the original state.

---

## A-054 — Operating-mode preference write failure is only a warning even though migration returns success

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 MIGRATION

**Evidence**

Both Modern and Legacy setup append a warning when `SaveOperatingPreference()` fails and then return nil success.

**Risk**

Hardware/driver state can be one mode while LogiMate persists/infers another on next launch, causing confusing or unsafe follow-up behavior.

**Required fix**

Treat persistent state commit as part of transaction commit, or derive the mode solely from freshly verified hardware and mark preference as non-authoritative.

**Acceptance**

Preference storage failure cannot yield a completed-success migration with contradictory persistent state.

---

## A-055 — HVCI/Memory Integrity handling models a registry request, not the effective security state

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 MIGRATION

**Evidence**

`HVCIEnabledNative()` reads one registry DWORD and setters write it. Policy enforcement, reboot-pending/effective state and enterprise controls are not distinguished.

**Risk**

UI/journal can say HVCI was disabled/re-enabled while the effective Windows security state has not changed yet or cannot change.

**Required fix**

Represent requested/effective/reboot-required/policy-locked states separately and verify after restart before claiming legacy-driver readiness or restored protection.

**Acceptance**

Policy-locked and reboot-pending test cases are shown explicitly and never reported as effective success.

---

## A-056 — Fresh Legacy install verifies the official Logitech installer signer before execution

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE

**Evidence**

`DownloadAndInstallOfficialLGS()` applies size bounds and Authenticode signer validation before starting the downloaded LGS installer.

**Risk**

This is a strong privilege-boundary practice for the legacy bootstrap path.

**Required fix**

Keep signer verification mandatory and pin expected publisher semantics in tests.

**Acceptance**

Tampered/unsigned/non-Logitech installer is rejected before execution.

---

## A-057 — Driver backup restore verifies recorded SHA-256 hashes

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE/TESTS

**Evidence**

Driver backup creates a manifest and restore validates backup hashes before adding drivers back to the Driver Store; reliability/model tests cover tamper rejection.

**Risk**

This reduces risk of restoring corrupted or modified INF payloads.

**Required fix**

Preserve manifest validation through C8/D0.

**Acceptance**

Tampered backup remains rejected by tests.

---

## A-058 — Profiler backup/restore has integrity and bounded-path checks

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE

**Evidence**

Profiler backup records hashes/metadata; restore path validates payloads and prevents unsafe relative/absolute archive paths before applying files/registry/import/install steps.

**Risk**

The legacy application backup is materially safer than an unvalidated file copy.

**Required fix**

Retain integrity checks and add end-to-end hardware/Windows tests.

**Acceptance**

Corrupt/tampered backup is rejected before restore actions.

---

## A-059 — Old migration cleanup can delete unresolved transaction journals solely by age

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8

**Evidence**

`cleanupOldMigrations()` removes every old `.json` journal based only on mtime; it does not preserve `running`, `pending`, `failed` or rollback-required records.

**Risk**

Forensics/recovery evidence for an unresolved destructive transaction can disappear after 30 days.

**Required fix**

Archive or retain unresolved journals until explicit resolution; age-prune only terminal success/cancelled records under a documented policy.

**Acceptance**

A 31-day-old unresolved journal survives cleanup while completed historical records may be archived/pruned.

---

## A-060 — Modern↔Legacy/HVCI rollback remains a real-Windows hardware gate

**Status:** NOT TESTABLE WITHOUT HARDWARE  
**Severity:** HIGH validation gate  
**State:** OPEN — HARDWARE GATE

**Evidence**

Cross-compilation and source review cannot validate PnP timing, Driver Store behavior, reboot transitions, LGS behavior and HVCI on actual supported Windows builds.

**Risk**

Driver migration is a destructive system operation and must be certified separately from application logic.

**Required fix**

Execute the documented fresh/legacy/modern/failure/reboot matrix on supported Windows 11 hardware with G25/G27/DFGT.

**Acceptance**

Recorded before/after device inventory and rollback evidence exists for every supported path.

---


# 13. Plan-D Engineering Audit 61–70 — Profiles / Games / Telemetry / Adapters

## A-061 — A telemetry-LED-only game profile can still drive the wheel motor

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

`StartFusionC6GameOutput()` explicitly permits a profile when either Game FFB is enabled OR telemetry LEDs are enabled. `fusionC6TelemetrySource.TryGetFrame()` then always converts telemetry force into Constant Force; it never gates on `gp.GameFFBEnabled`.

**Risk**

A configuration that appears to request LEDs only can unexpectedly generate motor torque.

**Required fix**

Motor source must return exact zero unless the profile explicitly enables Game FFB; LED output must be independently ownable without acquiring a motor-force source.

**Acceptance**

Profile `GameFFBEnabled=false, LEDPolicy=telemetry` drives LEDs while every motor report remains neutral/absent.

---

## A-062 — Pino telemetry `FFBEnabled=false` is ignored by the Fusion C6 motor source

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

Pino parsing stores `TelemetryFrame.FFBEnabled`, but `fusionC6TelemetrySource.TryGetFrame()` only checks telemetry freshness, Physics and PlayerControl before using `Force`.

**Risk**

The game/adapter can explicitly say FFB is disabled while LogiMate continues applying its force value.

**Required fix**

Require adapter-level FFB enable + profile-level FFB enable + valid player/physics state before non-zero motor force. Any false/unknown gate yields zero.

**Acceptance**

A live frame with force=1.0 and `FFBEnabled=false` produces 0 motor force.

---

## A-063 — Automatic telemetry startup errors are swallowed by game-session logic

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8/D3

**Evidence**

`UpdateGameSession()` calls `EnsureGameProfileTelemetry(p)` without an error result; listener bind/start failures can coexist with a profile reported as active.

**Risk**

Diagnostics/session UI can imply the adapter is running while no telemetry listener exists; later output behavior is harder to reason about.

**Required fix**

Adapter lifecycle functions return health/error and GameSession includes that state. Auto-apply/output start must require healthy adapter state when telemetry is required.

**Acceptance**

Occupied port/start failure surfaces as profile-degraded and prevents telemetry-dependent output.

---

## A-064 — Background game-profile selection is nondeterministic when multiple matching games run

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8/D3

**Evidence**

`ActiveGameProfile()` iterates a Go map returned by `ListRunningProcessNamesNative()` for non-foreground profiles. Map iteration order is intentionally unspecified.

**Risk**

With two matching background games, a different profile/engine configuration can win across refreshes/runs.

**Required fix**

Define deterministic priority: foreground, explicit priority/order, exact executable before substring, then stable tie-break; surface ambiguity instead of random choice.

**Acceptance**

Two simultaneously matching processes always select the documented winner or return an explicit ambiguity.

---

## A-065 — Corrupt game-profile storage silently becomes an empty/default profile set

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8 RELIABILITY

**Evidence**

`loadGameProfiles()` ignores JSON unmarshal errors and normalizes whatever default/partial structure remains.

**Risk**

Game behavior can silently disappear and the next save may overwrite recoverable data.

**Required fix**

Use the same structured config-health/last-known-good policy as input/native profiles.

**Acceptance**

Malformed game profile storage produces a visible recovery state and is not silently overwritten.

---

## A-066 — Pino telemetry packet diagnostics never increment `Packets`

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8

**Evidence**

The JSON listener increments `Packets`; the Pino listener contains `Frames += 0` before parsing and never increments `Packets`.

**Risk**

Diagnostic packet/invalid ratios are misleading, making telemetry fault analysis harder.

**Required fix**

Increment packet count for every accepted UDP datagram before parse, consistently across adapters.

**Acceptance**

Known valid+invalid Pino datagrams produce correct Packets/Frames/Invalid counters.

---

## A-067 — Telemetry listeners are loopback-only and parsing is bounded

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN SOFTWARE

**Evidence**

Both local JSON and Pino listeners bind `127.0.0.1`, reject non-loopback remotes, limit packet sizes and clamp/validate numeric telemetry values.

**Risk**

The network attack surface is intentionally local and malformed-number behavior is bounded.

**Required fix**

Preserve loopback default; any future remote adapter must be explicit and separately threat-modeled.

**Acceptance**

Existing parser/network tests remain green.

---

## A-068 — Telemetry freshness plus Physics/PlayerControl gates are present

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN SOFTWARE

**Evidence**

`TelemetrySnapshot()` marks samples stale after 750 ms and Fusion C6 zeros force when Physics or PlayerControl is false.

**Risk**

Normal game pause/control-loss cases already fail toward zero, aside from the missing FFBEnabled gates in A-061/A-062.

**Required fix**

Keep freshness/control gates in the common adapter contract.

**Acceptance**

Stale/paused/not-in-control frames always resolve to zero after the A-061/A-062 fix.

---

## A-069 — Game-profile import is size-bounded and normalized before persistence

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN SOFTWARE

**Evidence**

JSON import rejects payloads over 1 MiB, profile counts over 200 and invalid empty match rules; profile values are normalized/clamped.

**Risk**

This limits malformed import damage and memory/config abuse.

**Required fix**

Retain schema validation and add explicit versioning in D3.

**Acceptance**

Oversized/invalid imports remain rejected by tests.

---

## A-070 — Telemetry “adapter registry” is metadata, not yet an adapter lifecycle interface

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — D3 REQUIRED

**Evidence**

`RegisteredTelemetryAdapters()` returns descriptors, while startup/stop/parse behavior is hard-coded into branches such as `EnsureGameProfileTelemetry()` and Fusion C6.

**Risk**

Adding games will multiply special cases and make health/ownership/error semantics inconsistent.

**Required fix**

D3 defines a typed adapter interface: ID/capabilities/start/stop/health/frame schema/freshness/FFB authorization/LED data and deterministic lifecycle.

**Acceptance**

Wreckfest/Pino and local JSON run through the same registry/health contract with no hard-coded adapter branch in the output engine.

---


# 14. Plan-D Engineering Audit 71–80 — UI / UX State Truth

## A-071 — UI claims an exclusive output gate that C7 does not actually enforce

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 UI TRUTH

**Evidence**

The Native Wheel Engine dialog says hardware commands use “denselben exklusiven Output-Gate wie die FFB-Engine”, but A-007/A-031/A-040 show separate paths/handles with no common lease.

**Risk**

A safety claim in the UI is stronger than the backend guarantee.

**Required fix**

Until C8 implements the real lease, change wording to accurately describe Experimental behavior; after implementation derive ownership status from the backend lease.

**Acceptance**

Every UI safety statement is backed by an executable invariant/test.

---

## A-072 — Turning Experimental Native Output off can release bookkeeping without guaranteed neutralization

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY/UI

**Evidence**

`toggleSetting()` flips `NativeWheelOutput` off and calls `NativeOutputRelease()`, which has the A-008/A-034 stop/marker weaknesses and returns no error to the UI.

**Risk**

The user action that means “disable motor output” can visually switch off while force neutralization was not proven.

**Required fix**

Disabling must synchronously request emergency-neutral through the lease, report failure, preserve recovery state and only then persist the disabled setting.

**Acceptance**

Injected stop failure leaves the UI in blocked/fault state with Native Output disabled for new commands but recovery still required.

---

## A-073 — Experimental motor-output opt-in is a one-click settings toggle with no durable risk acknowledgement

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8 UX

**Evidence**

Native Wheel Output is default-off and clearly labeled Experimental, but enabling it is a normal toggle and does not present a one-time low-force/emergency-stop acknowledgement.

**Risk**

A user can enable motor-driving controls without seeing the hardware-validation status or emergency procedure.

**Required fix**

Add a one-time explicit experimental/hardware warning tied to current release/model certification state; keep default off.

**Acceptance**

First enable requires acknowledgement; future stable-certified models can use capability/certification policy to reduce friction.

---

## A-074 — Corrupt UI settings silently reset individual/all values to defaults

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8 RELIABILITY

**Evidence**

`loadUISettings()` ignores top-level/field JSON errors and simply retains defaults. No corruption health is surfaced.

**Risk**

Behavior such as update channel/setup offers/theme can change unexpectedly; future safety settings must never silently reset.

**Required fix**

Add settings schema/load health, preserve corrupt file and recover from last-known-good/snapshot. Safety-sensitive fields require explicit fallback semantics.

**Acceptance**

Malformed settings produce a visible warning and do not get silently overwritten.

---

## A-075 — Obsolete `AutoStartOpenG27` remains in the settings schema after the C7 cutover

**Status:** WARN  
**Severity:** LOW  
**State:** OPEN — C8 CLEANUP

**Evidence**

`uiSettings.AutoStartOpenG27` is persisted/read/tested but no longer appears as an active user setting/runtime behavior in the C7 native-default design.

**Risk**

Dead schema/state increases migration confusion and can tempt future code to restore obsolete ownership behavior.

**Required fix**

Migrate/remove the obsolete field or explicitly retain it as deprecated-read-only compatibility data with a removal version.

**Acceptance**

No production decision depends on AutoStartOpenG27 and schema migration is documented.

---

## A-076 — Setup can report completion even when its completion preference failed to persist

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8 UX/STATE

**Evidence**

`completeSetupGuide()` and “Nicht mehr automatisch” ignore `replaceUISettings()` errors before closing and showing success feedback.

**Risk**

The guide may reappear or state may diverge after the user was told the action completed.

**Required fix**

Handle persistence errors before success/close; keep the guide open or show an actionable failure.

**Acceptance**

Unwritable settings file cannot yield “Ersteinrichtung abgeschlossen”.

---

## A-077 — Destructive setup flow fails closed on ambiguous/no/multiple wheel selection

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE

**Evidence**

Setup/migration gates require an actionable selected supported wheel, reject multiple supported wheels and require model/PnP evidence before destructive mode changes.

**Risk**

This is an important protection against touching the wrong Logitech device.

**Required fix**

Retain and move the rule into DeviceRegistry transaction policy in D0.

**Acceptance**

Ambiguous/multi-wheel regression tests continue to block mode change.

---

## A-078 — Setup window cannot be casually closed while a tracked migration is running

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE

**Evidence**

Setup dialog close handling and live migration state keep the destructive operation visible until the tracked admin process reaches a result.

**Risk**

This reduces user-driven UI interruption during driver operations.

**Required fix**

Retain; pair with durable journal hardening from A-052/A-097.

**Acceptance**

WM_CLOSE/Escape tests remain blocked during migration.

---

## A-079 — Destructive mode changes have explicit user plan/confirmation before elevation

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE

**Evidence**

The guided setup builds the selected Modern/Legacy/HVCI/Profiler plan before launching the elevated admin action; legacy direct mode-change actions are refused.

**Risk**

This is good UAC/consent separation.

**Required fix**

Keep privileged worker parameter surface minimal and verified against the pre-authorized transaction token.

**Acceptance**

No unconfirmed direct driver switch path is introduced.

---

## A-080 — End-to-end Windows UI/accessibility/hardware workflow still requires external validation

**Status:** NOT TESTABLE WITHOUT HARDWARE  
**Severity:** MEDIUM validation gate  
**State:** OPEN — RELEASE GATE

**Evidence**

Unit tests cover selected UI logic and CI smoke starts the app, but the complete guided workflow, mixed DPI, accessibility, dialogs and physical wheel interactions cannot be validated on the Linux audit host.

**Risk**

UI truth is part of safety because users act on these statuses during driver/motor operations.

**Required fix**

Run the documented Windows UIA/manual matrix and retain results per release candidate.

**Acceptance**

No stable release until required UI automation/manual gates are signed off.

---


# 15. Plan-D Engineering Audit 81–90 — Reliability / Security

## A-081 — There is no OS-wide single-instance/output-owner lock for LogiMate itself

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — C8 SAFETY

**Evidence**

Source search finds no named mutex/file lock/single-instance mechanism. `validateNativeOutputTarget()` blocks the standard OpenG27 process name but cannot see another LogiMate process.

**Risk**

Two LogiMate instances can independently open the same HID target and drive conflicting effects, bypassing all in-process mutexes/generations.

**Required fix**

Create an OS-wide per-user application singleton and, more importantly, a per-physical-wheel named output lease/mutex held for the lifetime of any motor/output ownership.

**Acceptance**

Launching a second instance cannot acquire output; diagnostics identify the existing owner. Two-process integration test proves mutual exclusion.

---

## A-082 — Atomic file writes use a fixed `.tmp` name and are not safe under concurrent writers

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 RELIABILITY

**Evidence**

`AtomicWriteFile(path, ...)` uses `path + ".tmp"`. Concurrent goroutines/processes writing the same config can race on the same temporary file/rename.

**Risk**

A function designed as an atomic primitive can fail, lose updates or move another writer’s temp file under concurrency.

**Required fix**

Use unique same-directory temp files plus per-file serialization/compare-generation where lost updates matter; fsync directory/replace semantics as appropriate on Windows.

**Acceptance**

Concurrent stress test performs thousands of writes without corruption, cross-writer temp collisions or invalid JSON.

---

## A-083 — Updater verifies integrity but not independent publisher authenticity

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — STABLE RELEASE GATE

**Evidence**

Downloaded assets are SHA-256 checked against GitHub release digest or `SHA256SUMS.txt`, both obtained from the same release trust domain. Current binaries are explicitly unsigned.

**Risk**

If the repository/release publishing account is compromised, an attacker can replace both payload and checksum. SHA-256 alone proves consistency, not publisher identity.

**Required fix**

Authenticode-sign release binaries and verify expected publisher/signature before executing/installing an update; optionally add an independently pinned signing key/manifest.

**Acceptance**

Updater rejects an otherwise correctly hashed binary whose trusted publisher/signature is missing or wrong once stable signing is enabled.

---

## A-084 — ZIP extraction has traversal and decompression-size defenses

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE

**Evidence**

`unzip()`/update extraction bounds file count, individual and total uncompressed size, canonicalizes destination and rejects paths escaping the target directory.

**Risk**

This addresses common Zip Slip/decompression abuse in downloaded archives.

**Required fix**

Retain tests including mixed separators/case/`..`/large-entry vectors.

**Acceptance**

Traversal and oversized archive tests remain rejected.

---

## A-085 — Native updater helper accepts arbitrary apply paths from command-line arguments

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8 SECURITY HARDENING

**Evidence**

`runUpdateHelperIfRequested()` trusts `newExe`, `target`, `stage`, `targetDir` and log paths supplied on its command line; there is no transaction token/root allowlist binding them to a verified download started by the parent.

**Risk**

This is not an elevation bypass—the helper runs at the caller’s rights—but it unnecessarily exposes a same-user file replacement primitive and weakens update provenance.

**Required fix**

Issue an unguessable short-lived update transaction file/token and validate source hash, expected target/current exe root and stage root before replacement.

**Acceptance**

Manually invoking helper mode with unrelated paths/token fails before modifying files.

---

## A-086 — Update/uninstall lifecycle is not coordinated with another running LogiMate/output owner

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8

**Evidence**

Because there is no singleton (A-081), a separate `--uninstall` process can run while a normal LogiMate instance is active. Uninstall itself has no output-owner shutdown handshake; update relies on closing one known PID only.

**Risk**

Files/registration can be changed while another process still owns HID/output or writes configuration.

**Required fix**

Use the process/output lease for install/update/uninstall coordination: refuse or request verified shutdown of every active owner before mutating installation state.

**Acceptance**

With one active output instance, a second uninstall/update cannot proceed until the owner has neutralized and exited.

---

## A-087 — Diagnostic exports redact paths but still expose detailed device identifiers/topology

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — PRIVACY HARDENING

**Evidence**

`BuildDiagnosticReport()` redacts user paths but prints stable/session IDs, instance IDs, container/location/parent/physical IDs and driver inventory.

**Risk**

Those details are useful for support but can contain machine-specific identifiers users may not want to post publicly.

**Required fix**

Offer a “share-safe” diagnostic mode that hashes/redacts stable hardware identifiers while keeping correlation; keep full local report as an explicit option.

**Acceptance**

Public-support export contains no raw container/instance/location identifiers while remaining diagnostically correlatable.

---

## A-088 — Privileged driver/HVCI operations are explicitly UAC-gated rather than silently elevated

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN CODE

**Evidence**

Normal UI builds a migration plan and uses `runas`; the project security policy explicitly avoids bypassing UAC and HVCI changes require explicit user choice.

**Risk**

Privilege boundaries are visible to the user and destructive operations are not hidden in background startup.

**Required fix**

Retain the split between unprivileged UI and narrow privileged transaction worker.

**Acceptance**

No driver/HVCI mutation is reachable from the normal process without explicit elevation.

---

## A-089 — External OpenG27 ownership detection is executable-name based and can miss renamed instances

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8/D0

**Evidence**

Safety checks call `IsProcessRunning("OpenG27.App.exe")`. A copied/renamed external OpenG27 binary can hold the wheel while this name check returns false.

**Risk**

LogiMate can then acquire its own HID output path while another userspace owner is active.

**Required fix**

Do not infer hardware ownership from process names. Use an interoperable named device lease where possible and/or verify exclusive transport ownership. Treat external fallback launch as one participant in that lease.

**Acceptance**

Renaming the external executable cannot produce simultaneous hardware ownership.

---

## A-090 — Windows race/handle-leak stress and real HID concurrency remain unexecuted gates

**Status:** NOT TESTABLE WITHOUT HARDWARE  
**Severity:** HIGH validation gate  
**State:** OPEN — RELEASE GATE

**Evidence**

The audit can cross-compile Windows tests but cannot run Go race detection against Win32/HID code on this Linux host, nor measure handle/goroutine behavior through repeated physical reconnects.

**Risk**

Concurrency defects in long-lived USB/UI sessions may only appear under real scheduling/device churn.

**Required fix**

Add Windows CI/stress jobs plus physical reconnect loops; record process handle/goroutine counts and run race-enabled tests where build constraints permit.

**Acceptance**

Sustained refresh/reconnect/profile/output stress shows no race reports, unbounded handles/goroutines or stale owners.

---


# 16. Plan-D Engineering Audit 91–100 — Release Readiness

## A-091 — Main-branch CI artifacts are stamped with stale version `0.0.3-alpha`

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 RELEASE ENGINEERING

**Evidence**

`.github/workflows/build.yml` hardcodes `-X main.version=0.0.3-alpha` for app and installer while the audited source/release is 0.2.8-alpha.

**Risk**

CI artifacts from `main` can report the wrong version, affecting changelog/update/channel behavior and making test evidence ambiguous.

**Required fix**

Derive version from one source of truth (tag for releases; generated/dev version for branches) and assert binary/installer/package metadata agree.

**Acceptance**

CI fails if app-reported version, installer version and expected workflow version diverge.

---

## A-092 — Stable release signing is explicitly missing

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — STABLE RELEASE GATE

**Evidence**

`SECURITY.md` and `EvaluateStableReleaseGate()` acknowledge that current preview binaries are not Authenticode signed and require signed binaries for stable readiness.

**Risk**

Users/UAC/Updater cannot verify publisher identity; this also prevents satisfying A-083 strongly.

**Required fix**

Introduce reproducible signing stage with protected certificate/key, timestamping and signature verification in release validation.

**Acceptance**

Both `LogiMate.exe` and installer pass Authenticode verification for the expected publisher before stable publication.

---

## A-093 — Tag release workflow already has a solid software build/test/package baseline

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN SOURCE

**Evidence**

`.github/workflows/release.yml` builds Windows x64, runs `go vet` and `go test ./...`, performs a smoke start, builds installer, validates ARM64 compile, creates portable/source archives and SHA-256 sums.

**Risk**

The software-only release pipeline is materially better than ad-hoc binary packaging.

**Required fix**

Keep it as the canonical pipeline and add the missing signing/hardware attestation/gates rather than creating parallel release logic.

**Acceptance**

Release artifacts are produced only after all software CI gates pass.

---

## A-094 — Local `build.ps1` and CI release packaging are not equivalent

**Status:** WARN  
**Severity:** MEDIUM  
**State:** OPEN — C8 RELEASE ENGINEERING

**Evidence**

`build.ps1` builds/tests/setup/portable/checksums but does not produce the same source archive/release asset set as `release.yml`; build workflow also differs in version source.

**Risk**

A manually produced release can differ from CI even when both are called “release-ready”.

**Required fix**

Create one canonical release script invoked by CI and developers, parameterized only by version/signing context.

**Acceptance**

Running canonical packaging locally and in CI yields the same asset names/layout and validation steps.

---

## A-095 — Required G25/G27/DFGT hardware matrix is documented but not an enforceable release artifact

**Status:** NOT TESTABLE WITHOUT HARDWARE  
**Severity:** BLOCK validation gate  
**State:** OPEN — STABLE RELEASE GATE

**Evidence**

`docs/TESTING.md` has extensive hardware matrices, but the automated release workflow has no signed/manual test-result artifact or gate proving they were run for the candidate.

**Risk**

A tag can be published after software CI even if no physical wheel validation occurred.

**Required fix**

Add a release checklist/attestation artifact tied to commit/version, with model/Windows/build/tester/result and required gates for features enabled in that release.

**Acceptance**

Stable publication requires all model/capability hardware gates applicable to the release to be PASS.

---

## A-096 — Crash-recovery/no-stuck-force certification has no automated or recorded release gate

**Status:** NOT TESTABLE WITHOUT HARDWARE  
**Severity:** BLOCK validation gate  
**State:** OPEN — STABLE RELEASE GATE

**Evidence**

The test docs call out crash/reconnect/power scenarios, but release automation cannot prove them and the current release contains the A-041–A-050 blockers.

**Risk**

This is the most important motor-safety acceptance criterion.

**Required fix**

After C8 fixes, execute process kill, panic, USB yank/replug, game exit, telemetry loss, sleep/resume and Windows shutdown at bounded low force; attach results to release attestation.

**Acceptance**

Every enabled motor-output model passes the full stuck-force matrix before stable status.

---

## A-097 — Incomplete migration journals are not discovered/recovered on normal application startup

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 MIGRATION RECOVERY

**Evidence**

Journal files are created and the setup window polls the current token, but source search finds no startup scan that detects old `pending`/`running`/failed rollback transactions from a previous crash/reboot.

**Risk**

After a crash during driver migration, the next launch can proceed as normal instead of entering a recovery/verification workflow.

**Required fix**

On startup enumerate unresolved journals, compare the current machine/wheel state with the saved snapshot/expected target, and require recovery/resolution before new migrations.

**Acceptance**

Terminate the admin/UI process mid-migration; next launch detects the unresolved transaction and enters recovery, not normal setup.

---

## A-098 — Data schema accepts unknown newer versions instead of blocking downgrade writes

**Status:** WARN  
**Severity:** HIGH  
**State:** OPEN — C8 RELIABILITY

**Evidence**

`EnsureDataSchema()` returns success whenever stored `Version >= CurrentDataSchemaVersion`; it does not distinguish equal from future/unsupported schema versions.

**Risk**

An older LogiMate launched against data written by a newer version can read fields imperfectly and later overwrite files in an incompatible format.

**Required fix**

Reject/write-protect future schema versions and offer read-only/export or explicit supported downgrade recovery.

**Acceptance**

Schema version `Current+1` causes a clear “newer data format” block before settings/config writes.

---

## A-099 — Audited C7 release package is internally checksum-consistent and archives are structurally readable

**Status:** PASS  
**Severity:** LOW  
**State:** VERIFIED IN AUDIT

**Evidence**

The original release SHA-256 manifest matched EXE, installer, portable, source and GitHub-ready assets. ZIP contents were readable; subsequent audit-only packages regenerate and verify checksums.

**Risk**

This establishes a trustworthy immutable C7 behavior baseline for C8 work.

**Required fix**

Keep release checksum verification as a packaging gate; add signatures per A-092.

**Acceptance**

Final audit package passes fresh SHA-256 and archive integrity verification.

---

## A-100 — C7 is not ready for a stable/native-motor release until C8 blockers and external gates are closed

**Status:** BLOCK  
**Severity:** BLOCK  
**State:** OPEN — FINAL RELEASE GATE

**Evidence**

Across A-001–A-099 the audit found multiple output-ownership, emergency-stop/recovery, identity, input, migration, telemetry and multi-process blockers plus unsigned/unvalidated hardware gates.

**Risk**

Proceeding directly to D1 feature expansion would compound unsafe ownership/state assumptions.

**Required fix**

C8 must close every BLOCK finding and high-risk prerequisites, then D0 establishes the unified engine. Only after regression + hardware + signing/UI gates pass should stable readiness be reconsidered.

**Acceptance**

Master audit shows zero OPEN BLOCK findings; required external gates are PASS; final regression/hardware report is attached to the candidate release.

---

# 17. Release/change log for this tracker

## 2026-09-12 — C7 audit baseline / A-001–A-010

- Uploaded release treated as immutable C7 baseline before changes.
- Original release checksums verified.
- Source/build baseline checked.
- New Plan-D engineering audit created.
- Findings A-001 through A-010 recorded.
- Three initial C8 safety blockers identified: A-007, A-008 and A-009.
- No production Go code changed.

## 2026-09-12 — C7 audit continuation / A-011–A-020

- Audited device discovery, persisted identity, Raw Input correlation, hotplug/reconnect and multi-wheel gates.
- Added C8 blockers A-011 and A-012 plus high-priority safety finding A-013.
- Confirmed fail-closed stored-target behavior and exact G27 Raw-HID correlation (A-016/A-017).
- Documented G25/DFGT and native-output multi-wheel limitations for D0/D1.
- No production Go code changed; C7 remains the immutable behavior baseline.
- Next audit batch: A-021–A-030 (input engine).

## 2026-09-12 — C7 audit continuation / A-021–A-030

- Audited the live input state model, Direct-HID/WinMM fallback, axes/pedals, steering, button/D-pad semantics, H-shifter learning and calibration persistence.
- Identified C8 input blockers A-022 (synthetic/invalid samples can be learned) and A-024 (unsigned axis normalization underflow).
- Added high-priority source-layout/calibration findings A-021/A-023/A-025/A-026/A-027.
- Kept G25 H-shifter semantics behind a physical-hardware validation gate (A-028).
- Recorded process-global WinMM telemetry and silent profile-corruption behavior for cleanup (A-029/A-030).
- No production Go code changed; C7 remains the immutable behavior baseline.
- Next audit batch: A-031–A-040 (output/FFB engine).

---

## 2026-09-12 — Full C7 Plan-D engineering audit / A-031–A-100

- Completed output/FFB, safety/recovery, migration, game/telemetry, UI, reliability/security and release-readiness audit batches.
- Full Plan-D engineering audit now stands at **100/100 reviewed**.
- Confirmed additional C8 safety blockers including worker/lease hand-off, recovery marker durability, startup/shutdown recovery, telemetry Game-FFB gating, multi-process ownership and migration transaction verification.
- Confirmed multiple good foundations: bounded packet builders/mixer, loopback telemetry, archive extraction defenses, UAC separation, driver/profiler backup integrity and a solid tag-release software CI baseline.
- Physical motor behavior, crash/power/disconnect matrix, full Windows UI validation and Authenticode remain external release gates.
- No production Go code changed during A-001–A-100; C7 remains the immutable behavior baseline.
- Next development checkpoint is **C8 — Reliability & Architecture Cleanup**, not D1 feature expansion.

# 18. Rule for subsequent releases

Every future C8/D0/D1/... release must update this file with:

1. version/checkpoint,
2. audit points completed,
3. exact fixes implemented,
4. tests added/run,
5. physical hardware evidence still missing,
6. newly discovered regressions/blockers,
7. exact next action.

This file is the canonical “where are we?” document for a new chat.


---

## C8 remediation status — 0.2.9-alpha

The A-001…A-100 C7 audit is now historical baseline. C8 applies the remediation plan. The authoritative C8 closure report is `LOGIMATE_C8_FIX_REPORT.md`; release state is `C8_RELEASE_READINESS.md`; physical evidence is controlled by `HARDWARE_CERTIFICATION.json`.

### C8 outcome

- All 24 C7 software BLOCK findings are closed for the alpha code path.
- External hardware/Windows/signing claims remain explicit Stable gates and are **not** marked PASS without evidence.
- Three non-blocking structural findings remain deliberate D0 work: package decomposition, residual stringly state at compatibility/UI boundaries, and convergence to one cancellable command queue/transport.
- C8 is the reliability bridge. No new wheel family should be promoted to stable motor output before D0 plus the hardware certification matrix.

### Next stage after C8

**D0 — Native Engine Foundation**

1. WheelManager / DeviceRegistry as explicit owners of physical/logical identity.
2. Typed WheelModel/OperatingMode/Capabilities throughout the backend.
3. One cancellable/overlapped hardware transport and command queue.
4. Canonical WheelInputState and output/safety state machine.
5. Profile and telemetry adapter interfaces with explicit lifecycle.
6. Keep OpenG27 only as attributed parity/reference/fallback tooling while removing duplicated runtime responsibilities.
7. Only then expand/promote G25/DFGT output features and continue D1–D5.

### New-chat handoff

Read, in this order:

1. `LOGIMATE_MASTER_AUDIT_PLAN.md`
2. `LOGIMATE_FINAL_AUDIT_REPORT_C7.md` (historical baseline)
3. `LOGIMATE_C8_FIX_REPORT.md` (what C8 changed)
4. `C8_RELEASE_READINESS.md`
5. `HARDWARE_CERTIFICATION.md` / `.json`
6. `FUSION_ROADMAP_C_TO_D.md`

Then continue with D0 unless a C8 regression is reported.


---

# 18. 2026-09-12 — Native D0 / 0.3.0-alpha hand-off

## Current truth

- C7 100-point audit: complete.
- C8 remediation: complete at the software level; all historical software BLOCK findings were closed before D0.
- D0 Native Engine Foundation: **implemented in 0.3.0-alpha**.
- Normal Modern operation: **no external OpenG27 process and no Logitech Profiler/LGS runtime required**.
- Legacy remains an optional original Logitech compatibility mode.
- Stable remains blocked by real hardware/recovery/migration/HID-stress/UI/signing gates.

## D0 code delivered

- New `internal/wheelengine` package with typed ModelID/OperatingMode, descriptors/capabilities, canonical Device/Input/Output/Health state and runtime manager.
- G25/G27/DFGT capability descriptors: native PID + C294 selector + controls/output capabilities.
- `system.State` is bridged into the canonical engine; selected model/mode and engine generation are explicit.
- Direct HID/WinMM live samples update canonical per-wheel/per-session state.
- One Native HID output transport boundary is used by productive range/LED/autocenter/FFB/conditions/C6 game output/native mode writes.
- One named output mutex/lease remains the only productive hardware ownership authority.
- One journaled Modern migration path for G25/G27/DFGT; the old G27-specific entry is compatibility-only and delegates to it.
- Wreckfest Pino canonical adapter ID is `wreckfest2-pino`; legacy `openg27-pino` is accepted/migrated.
- Game FFB is capability-driven across G25/G27/DFGT; Rev LEDs only execute when the descriptor has RPM LEDs.
- Normal UI/setup says Modern / LogiMate Native for all supported models and does not expose OpenG27 as the normal System action.

## Exact next development step — D1

Do **not** start another architectural rewrite. Use D0 as the baseline and perform **Model Certification & Refinement**:

1. G25 physical test/capture matrix.
2. DFGT physical test/capture matrix.
3. G27 regression matrix against the common D0 transport.
4. Convert every proven hardware difference into a descriptor/model adapter plus fixture.
5. Verify mode switch C294↔native, all inputs, range, constant/spring/damper/friction/autocenter, game FFB, reconnect and emergency stop.
6. Keep unsupported/unproven capabilities behind `hardware-gate`; never infer PASS from code compilation.
7. After D1 evidence, begin D2 consolidation of historical `fusion_c*`/OpenG27-port runtime duplication while preserving provenance/license records.

## New-chat instruction

Upload the latest `LogiMate-0.3.0-alpha-Native-D0-Release.zip`, then say:

> Read `docs/LOGIMATE_MASTER_AUDIT_PLAN.md`, `docs/D0_NATIVE_ENGINE_REPORT.md` and `docs/D0_STANDALONE_READINESS.md`. Continue at D1 Model Certification & Refinement. Preserve the D0 single-engine/single-output-owner architecture and do not reintroduce external runtime dependencies.


---

# 19. 2026-09-12 — Native D1/D2 / 0.3.2-alpha hand-off

## Current truth

- C7 A-001…A-100 audit: complete historical baseline.
- C8 reliability remediation: software-complete.
- D0 Native Engine Foundation: complete.
- D1 software model refinement: **complete**; physical G25/G27/DFGT certification remains an external Stable gate.
- D2 unified native engine consolidation: **complete at the productive runtime boundary**.
- Normal Modern operation requires neither external OpenG27 nor Logitech Profiler/LGS.
- `internal/openg27port` is retained only as an attributed reference/parity/import boundary, not as productive Modern runtime.

## D1 code delivered

- Explicit G25/G27/DFGT model adapters with native PID/selector, layout ID, report length and capabilities.
- Platform-independent native input decoding and model fixture tests.
- G27 Direct-HID source/layout metadata corrected to `direct-hid` / `g27-c29b-v1`.
- Evidence-based `ModelCertification` contract and per-model Stable packaging gate.
- Physical certification is deliberately still PENDING; builds/fixtures cannot promote hardware status.

## D2 code delivered

- LogiMate-native classic protocol builders in `internal/wheelengine`.
- LogiMate-native ForceFrame/FFB scheduler in `internal/wheelengine`.
- LogiMate-native Wreckfest/Pino parser and LED threshold semantics.
- Productive Native Game Output uses one capability-driven path for G25/G27/DFGT.
- Productive Direct-HID parsing and game output no longer import `internal/openg27port`.
- Historical C3 live output removed from normal UI; parity/dry-run history remains for regression.
- Existing `openg27-pino` profiles remain readable and normalize to `wreckfest2-pino`.
- Cross-package parity tests prove LogiMate-native protocol/scheduler/Pino behavior against the attributed oracle without runtime dependency.

## Intentional remaining boundaries

- Historical C2/C4/C7 diagnostics and release self-tests may import `internal/openg27port` as an oracle.
- OpenG27 profile import may parse the historical schema.
- External OpenG27 process detection remains a conservative Alpha safety interlock.
- The physically unverified G27 LED trailing-byte discrepancy remains unresolved until hardware capture/testing.
- Real G25/G27/DFGT motor/input/recovery/migration evidence remains required before Stable.

## Exact next development step — D3

Build the **Adapter Ecosystem** without touching the motor core unless a regression proves it necessary:

1. Define a stable `GameAdapter` / telemetry-source contract with lifecycle: discover/configure/start/stop/health.
2. Keep adapter output semantic (`RPM`, player-control state, normalized requested force/effects), never raw HID writes.
3. Route every adapter through the existing Native Game Output/safety/lease layer.
4. Add per-adapter health, packet age/rate, last error and setup instructions to diagnostics.
5. Migrate Wreckfest/Pino into the contract as the reference adapter without changing its wire behavior.
6. Add additional games/SDKs only through adapters and fixtures.
7. Preserve old profile IDs as migrations, not permanent duplicate runtimes.
8. Continue collecting D1 hardware evidence in parallel; hardware findings may refine model adapters but must not create game-specific motor bypasses.

## New-chat instruction

Upload `LogiMate-0.3.2-alpha-Native-D2-Release.zip`, then say:

> Read `docs/LOGIMATE_MASTER_AUDIT_PLAN.md`, `docs/D1_MODEL_CERTIFICATION_REPORT.md`, `docs/D2_UNIFIED_ENGINE_REPORT.md` and `docs/D2_RELEASE_READINESS.md`. Continue at D3 Adapter Ecosystem. Preserve the D2 rule that game adapters never write HID directly and do not reintroduce OpenG27/LGS as Modern runtime dependencies.


# 20. 2026-09-12 — Native D3 / 0.3.3-alpha hand-off

D3 Adapter Ecosystem is software-complete. `internal/gameadapter` is now the only intended entry point for new game/telemetry integrations. Adapters publish normalized semantic frames and health only; they must never write wheel HID or bypass Native Game Output. Wreckfest/Pino and Local JSON are reference adapters on the same lifecycle. `openg27-pino` remains a migration alias only. Game-profile schema is version 2 with future-version write protection.

## Exact next development step — D4

Build **Advanced FFB** on top of the existing single output engine, without weakening D0–D3 boundaries:

1. Add explicit filter pipeline (smoothing/low-pass, minimum-force/deadband compensation, slew/jerk control) as pure tested math before packet generation.
2. Add clipping/latency/rate telemetry and per-effect diagnostics.
3. Add safe model-aware presets and response curves; presets may reduce limits but never bypass the per-wheel safety ceiling.
4. Extend the semantic adapter frame only for normalized effects actually supported by the engine; never allow raw HID/report injection.
5. Move synchronous HID transport toward validated overlapped/cancellable `WriteFile` only when hardware evidence supports it.
6. Continue D1 hardware certification in parallel. Any discovered wire/layout difference belongs in model adapters/capabilities, not game adapters.

## New-chat instruction

Upload `LogiMate-0.3.3-alpha-Native-D3-Release.zip`, then say:

> Read `docs/LOGIMATE_MASTER_AUDIT_PLAN.md`, `docs/D3_ADAPTER_ECOSYSTEM_REPORT.md`, `docs/D3_RELEASE_READINESS.md`, `docs/D2_UNIFIED_ENGINE_REPORT.md` and `docs/HARDWARE_CERTIFICATION.md`. Continue at D4 Advanced FFB. Preserve the rule that game adapters only publish semantic frames and never own HID/motor output.

# 21. 2026-09-13 — Native D4 / 0.4.0-alpha hand-off

D4 Advanced FFB is software-complete. The platform-independent `wheelengine` now owns a tested force-shaping pipeline for deadband, minimum-force compensation, response curves, low-pass filtering, smoothing and semantic constant/transient gains. D4 shaping always runs before the existing hard `NativeFFBConfig` safety mixer. Native Game Output exposes raw/shaped/applied force, pipeline and safety clipping, deadband/minimum-force counts, sample latency and active HID backend. Custom Wheel Engine profiles expose independent Constant/Spring/Damper/Friction gains.

The preferred Windows HID writer now uses true OVERLAPPED `WriteFile` with exact-operation `CancelIoEx`; a classic HID stack may use the bounded `HidD_SetOutputReport` compatibility backend only after a synchronous error proves the overlapped operation was not queued. Any ambiguous pending write remains fail-closed.

## Exact next development step — D5

Complete **Modern-mode independence / historical dependency retirement** without breaking provenance or Legacy fallback:

1. Remove normal-user OpenG27 launch/install/A-B actions that are no longer necessary for supported Modern operation.
2. Keep attributed `openg27port` only where it provides parity tests, import compatibility or provenance value; no productive runtime imports may reappear.
3. Convert OpenG27-specific user-facing preset/import naming to neutral LogiMate migration/import terminology while preserving backward-compatible file parsing.
4. Audit setup/system/diagnostics text for any implication that OpenG27 or LGS is required by Modern.
5. Keep Logitech LGS/Profiler as an explicit Legacy-mode alternative, not a dependency.
6. Re-run the full standalone/runtime dependency audit and package a D5 report proving which external executables/files are optional vs. required.
7. Continue physical G25/G27/DFGT certification in parallel; D5 software independence is not a substitute for motor/hardware certification.

## New-chat instruction

Upload `LogiMate-0.4.0-alpha-Native-D4-Release.zip`, then say:

> Read `docs/LOGIMATE_MASTER_AUDIT_PLAN.md`, `docs/D4_ADVANCED_FFB_REPORT.md`, `docs/D4_RELEASE_READINESS.md`, `docs/ARCHITECTURE.md` and `docs/HARDWARE_CERTIFICATION.md`. Continue at D5 Modern-mode independence. Keep Legacy Logitech support optional and preserve OpenG27 provenance/import compatibility without restoring it as a Modern runtime dependency.


# 22. 2026-09-13 — Native D5 / 0.5.0-alpha hand-off

D5 Modern-mode Independence is **software-complete**. The production `cmd/logimate` dependency graph no longer contains `internal/openg27port`; the build script enforces this as a regression gate. LogiMate no longer discovers, downloads, installs, launches or polls an OpenG27 executable. Native G27 input defaults/calibration are LogiMate-owned and do not read `%APPDATA%\OpenG27\config.json`.

OpenG27 remains only as attributed source/test provenance and an explicit offline legacy game-profile import format. Logitech LGS/Profiler remains a deliberately selectable **Legacy** alternative and is not required by Modern.

## D5 acceptance

- Modern G25/G27/DFGT runtime requires Windows + LogiMate only: **PASS**.
- Productive runtime imports `internal/openg27port`: **NONE / PASS**.
- OpenG27 executable fallback/download/launch in normal UI: **REMOVED / PASS**.
- External OpenG27 config changes live input semantics: **NO / PASS**.
- Legacy OpenG27 profile parsing retained as offline migration: **PASS**.
- Legacy Logitech mode retained: **PASS**.
- Physical Stable certification: **PENDING by design**.

## Exact next development step — post-D5 / Stable certification

Do not rewrite the engine again. Use 0.5.0-alpha as the standalone baseline and move to evidence-driven hardware/release validation:

1. Run the complete G27 physical matrix first because hardware is available/known.
2. Capture G25 and DFGT evidence when physical devices are available.
3. Validate C294↔native mode switches, all inputs, range and every enabled effect.
4. Exercise USB yank, process kill, suspend/resume, reconnect and crash-recovery paths.
5. Run Modern↔Legacy↔Modern rollback tests.
6. Feed every proven hardware difference back only into model adapters/capabilities/fixtures.
7. Complete UI/accessibility, HID-stress and Authenticode gates before Stable/1.0.
8. After the three current models are certified, new classic Logitech wheels may be added through descriptors/adapters rather than new engines.

## New-chat instruction

Upload `LogiMate-0.5.0-alpha-Native-D5-Release.zip`, then say:

> Read `docs/LOGIMATE_MASTER_AUDIT_PLAN.md`, `docs/D5_MODERN_INDEPENDENCE_REPORT.md`, `docs/D5_RELEASE_READINESS.md`, `docs/HARDWARE_CERTIFICATION.md` and `docs/ARCHITECTURE.md`. D5 is complete. Continue with hardware certification / Stable readiness without reintroducing external Modern runtime dependencies.


---

## D5.1 — Detection Recovery (0.5.1-alpha)

A real-hardware report after D5 triggered a focused discovery audit before further development. The primary root cause was a literal escaping error in `deviceIsPnPVerified()`: Windows emits `USB\VID...` / `HID\VID...`, but D5 checked for two literal backslashes. Real wheels could therefore remain `PnPVerified=false` and be blocked by otherwise correct safety gates.

D5.1 fixes that regression and adds a HidSharp/OpenG27-like SetupAPI HID-interface discovery pass, fused discovery paths, USB/HID child grouping by USB topology, session-only G25/G27 C294 consensus from independent Windows evidence, immediate guarded native-mode restore after discovery, and correct G27 read/write handle sharing. See `docs/D5_1_DETECTION_RECOVERY_REPORT.md`.

**Next step after user-side G27 validation:** Hardware Certification Assistant and physical certification matrix. Do not proceed to Stable until the D5.1 cold-plug/replug/native-switch behavior is confirmed on real Windows hardware.

---

## D5.2 — OpenG27-style Detection (0.5.2-alpha)

A second real-hardware-focused detection audit compared LogiMate directly against OpenG27's actual HID lifecycle. D5.2 makes direct HID enumeration the primary visibility source, ranks candidates by HID output capability, collects one coherent HID/PnP/Raw-Input snapshot, settles C294/native re-enumeration, and uses direct HID while waiting for the post-switch native PID.

C294 remains multi-model safe: direct G25/G27 HID product identity may authorize only the current session; remembered native identity is reused automatically only with the same Windows device session, a strong hardware fingerprint, or fresh matching model evidence. USB-port history by itself never authorizes a native-mode command.

See `docs/D5_2_OPEN_G27_STYLE_DETECTION_REPORT.md` and `docs/D5_2_RELEASE_READINESS.md`.

**Next step:** validate D5.2 on the user's physical G27 before changing the engine further. If C294/C29B detection now behaves correctly, use that G27 as the first Hardware Certification Assistant target.

### D5.3 — G27 Native Activation Recovery — 0.5.3-alpha ✅ software / ⏳ physical G27 validation
- Real hardware screenshot confirmed Direct HID sees the wheel as `PID C294 · G27 Racing Wheel · In=8 Out=8`. Detection is therefore working; the remaining failure was activation/re-enumeration.
- Automatic native preparation no longer requires a pre-existing `modern` preference. A single actionable Generic-HID wheel may auto-promote unless Legacy was explicitly selected.
- G27 native switch now matches OpenG27 exactly: one `F8 09 04 01 00 00 00` report, then bounded wait for C29B.
- G25/DFGT retain their existing model-specific sequence until physical certification.
- Immediate next gate: verify on the user's G27 that C294 becomes C29B and Direct HID live input binds after re-enumeration.


### D5.4 — OpenG27-Compatible G27 Handshake — 0.5.4-alpha ✅ software / ⏳ physical G27 validation

- Direct HID already proves the user's wheel is a C294 G27 (`G27 Racing Wheel`, In=8, Out=8). D5.4 therefore focuses on the handshake, not detection.
- G27 C294 writer matches HidSharp/OpenG27 sharing semantics and write timeout.
- Exact switch bytes remain `00 F8 09 04 01 00 00 00`.
- Live Generic-HID binding overrides a stale persisted Legacy preference for automatic activation.
- Devices exposes an explicit OpenG27-like Connect/Native rescue action with phase diagnostics.
- Physical pass criterion: cold C294 G27 re-enumerates to C29B and Direct HID rebinds without OpenG27/LGS.

### D5.5 — Automatic HID Evidence Fusion Fix — 0.5.5-alpha ✅ software / ⏳ physical G27 validation

- Real G27 evidence showed Direct-HID could read `G27 Racing Wheel` on C294 and manual confirmation then worked.
- Root cause was narrowed to `mergeDeviceEvidence`: the all-class PnP record won de-duplication and the matching HID-interface record (carrying `HIDProduct`/serial) was discarded.
- Duplicate records are now enriched field-by-field, so direct HID model evidence reaches session consensus automatically.
- Expected next physical test: cold C294 G27 must become ModelConfirmed and enter the D5.4 C29B handshake without clicking manual model confirmation.

### D5.6 — Advanced Wheel View — 0.5.6-alpha ✅ software / ⏳ physical UI validation

- Existing **Lenkrad** page gains an in-place `Erweiterte Ansicht`; no new page is introduced.
- Compact dashboard remains visible; expanded view adds complete live input, HID, identity, calibration, output/FFB and raw-report visibility.
- Direct HID candidates are captured in the background scan and exposed through `State.HIDCandidates`, avoiding hardware calls from the paint path.
- Old Rohdatenmodus and the new header toggle share one state.
- Next physical check: validate layout/scrolling and raw values on the user's real G27 while continuing automatic C294→C29B testing.

---

## D5.7 — Hardware Certification Assistant — 0.5.7-alpha ✅ software / ⏳ physical evidence

The post-D5 hardware matrix is now executable inside LogiMate rather than being only a document checklist.

Implemented:
- per-physical-wheel evidence store under `LogiMateData\\Certification`;
- required checks sourced from the canonical wheelengine model certification list;
- automatic evidence only for provable current native PID/PnP/model facts;
- guided G27/G25/DFGT input, range and bounded FFB tests;
- guided reconnect/USB-yank/process-kill/suspend/migration evidence;
- Markdown + JSON export;
- live PASS/FAIL/PENDING matrix in the existing Lenkrad → Erweiterte Ansicht.

D5.6 Advanced Wheel View was accepted by the user on the real G27. Do not infer that automatic G27 C294→C29B is fixed: if manual model confirmation is still required, `native-mode-switch` remains FAIL.

### Exact next action

1. Use 0.5.7-alpha on the real G27.
2. Run `Kalibrieren & Lernen → Hardware Certification Assistant` in workflow order.
3. Record automatic C294→C29B as PASS only if it succeeds from cold/replug without manual model confirmation.
4. Complete input/range/effect checks.
5. Perform reconnect, USB-removal, process-kill, suspend/resume and Modern↔Legacy↔Modern tests.
6. Export the G27 evidence report and review it before setting any repository certification flag.
7. Repeat on real G25 and DFGT hardware when available.
8. Then close global Stable gates: accessibility, HID stress and Authenticode.

---

## D5.8 — Accessibility / UI Automation Foundation — 0.5.8-alpha ✅ software / ⏳ Windows validation

The first global post-D5 Stable gate now has a native Windows accessibility bridge without replacing LogiMate's custom-painted Fluent UI.

Implemented:
- transparent standard Windows BUTTON/CheckBox/RadioButton accessibility peers over the main-window navigation, page actions, Settings theme/toggles and Wheel Advanced View;
- standard Windows/MSAA/UI Automation names, roles, enabled state, keyboard focus and checked/selected state;
- one action path for mouse, keyboard and UI Automation invocation;
- a native status surface exposing the current page heading/subtitle/live body context;
- Wheel Advanced View added to keyboard focus order; Settings automatically scroll focused off-screen controls into view;
- D5.8 bridge/runtime status included in Diagnostics and Engine Health;
- regression coverage for accessibility/control focus-ID separation and Advanced View keyboard reachability.

Truth boundary:
- `docs/HARDWARE_CERTIFICATION.json -> uiAccessibilityValidated` remains **false**.
- Successful compilation cannot certify Narrator/UI Automation behavior.
- Secondary custom dialogs/setup/changelog plus High Contrast, reduced motion and mixed-DPI movement remain in the real Windows validation matrix.

### Exact next action

1. Run 0.5.8-alpha on Windows with Narrator and/or Accessibility Insights and confirm the main-window controls expose the expected names/roles/states and invoke the same actions.
2. Traverse every main page with Tab/Shift+Tab/arrows/Enter/Space/Escape, including `Erweiterte Ansicht` and off-screen Settings.
3. Repeat in Windows High Contrast and with client animations disabled/reduced motion.
4. Move LogiMate between 100% and scaled/mixed-DPI displays and check focus geometry/reading order.
5. Validate remaining custom dialogs/setup/changelog; fix any missing semantics before changing `uiAccessibilityValidated`.
6. Keep the existing real G27/G25/DFGT D5.7 certification work in parallel.
7. Next software Stable-readiness step: D5.9 HID stress/adverse-I/O evidence; Authenticode remains the final external signing gate.



---

## D5.9 — HID Stress / Adverse-I/O Hardening — 0.5.9-alpha ✅ software / ⏳ physical evidence

The second global Stable-readiness gate now has explicit tooling and observability instead of remaining a prose-only checklist.

Implemented:
- process-lifetime native HID transport counters for opens, writes, failures, timeouts, partial writes, `CancelIoEx`, poison events, compatibility fallbacks, closes and write latency;
- fail-closed hardening for partial output writes and failed OVERLAPPED completion confirmation — the writer is poisoned/closed and cannot be reused;
- deterministic D5.9 software policy stress integrated into Engine Health;
- guided 100-cycle real-wheel reopen/write/input stress using only an idempotent non-motor range report;
- guided USB-yank adverse-I/O window with expected disconnect evidence and confirmed central lease release;
- Markdown + JSON D5.9 evidence export under `LogiMateData\Certification`;
- explicit HID-stress parameter in the runtime Stable gate.

Truth boundary:
- `docs/HARDWARE_CERTIFICATION.json -> hidStressValidated` remains **false**.
- Software/fault-policy PASS is not physical USB/HID certification.
- D5.9 never auto-promotes repository Stable flags.
- Physical G25/G27/DFGT evidence still has to be reviewed.

### Exact next action

1. On the real G27 run `Kalibrieren & Lernen → D5.9 HID Stress / adverse I/O → Software Fault-Injection / Policy Stress`; it must PASS.
2. With no game/FFB writer active, run the 100-cycle live test and export its evidence.
3. Run the USB-yank test, remove the G27 during the test window, confirm the real I/O failure and lease release, reconnect, then rerun the 100-cycle test.
4. Review transport counters for unexpected partial writes, timeouts, poison events or compatibility fallback.
5. Repeat the required adverse-I/O evidence on physical G25 and DFGT hardware when available.
6. Keep D5.7 model certification and D5.8 Windows accessibility validation open in parallel.
7. After those physical gates are reviewed, the remaining Stable/1.0 external release gate is Authenticode signing/verification and final release evidence review.

---

## D5.10 — Release Trust / Authenticode Hardening — 0.5.10-alpha ✅ software / ⏳ external signing

The final global Stable release gate is now enforced by the build and visible at runtime instead of remaining a documentation-only requirement.

Implemented:
- runtime Authenticode status/publisher/thumbprint visibility in Engine Health, diagnostics and a dedicated D5.10 trust view;
- Stable detection from the release version (no prerelease suffix);
- exact VERSION ↔ certification-manifest match for Stable;
- Stable signing configuration through an externally provisioned certificate thumbprint and RFC3161 timestamp URL;
- LogiMate.exe signing + verification before it is embedded into the installer;
- final installer signing + verification after build;
- same-publisher/thumbprint enforcement across app and installer;
- machine-readable RELEASE_ATTESTATION.json with final hashes/trust/certification state;
- prerelease builds may remain unsigned but can never satisfy the Stable signing gate.

Truth boundary:
- 0.5.10-alpha is intentionally unsigned in the audit environment because no private release certificate is available.
- D5.10 does not mark any D5.7/D5.8/D5.9 physical gate as passed.
- Stable/1.0 still requires reviewed hardware, crash/recovery, migration, accessibility and HID-stress evidence before the signing stage is even allowed to run.

### Exact next action

1. Use 0.5.10-alpha and confirm D5.10 Release Trust reports the prerelease unsigned state accurately.
2. Continue the real G27 D5.7 hardware matrix and D5.9 adverse-I/O evidence.
3. Complete D5.8 Narrator/UIA/High-Contrast/reduced-motion/mixed-DPI validation.
4. Repeat required hardware evidence on real G25 and DFGT.
5. Promote repository certification flags only after evidence review.
6. On the trusted Windows release host, provision the Authenticode certificate and timestamp service, then build the Stable version.
7. Independently verify LogiMate.exe, LogiMate-Setup-x64.exe, SHA256SUMS.txt and RELEASE_ATTESTATION.json before publication.



---

## D6 — Wheel Control Panel / Force Feedback UI — 0.6.1-alpha ✅ software / ⏳ physical UX + wheel validation

D6 turns the existing D4/D5 FFB engine into an integrated visual control panel on the existing Wheel page. It does not create another main page or another motor writer.

Implemented:
- `Lenkrad` in-page tabs: `Live-Test` and `Force Feedback`;
- slider-based Master, Constant, Spring, Damper, Friction and rotation controls backed by the existing per-wheel Native Engine profile;
- slider-based D4 Transient, Minimum Force, Deadband, Response Curve, Low-Pass, Smoothing and Pre-Safety Limit;
- FFB pipeline toggle plus Gentle/Balanced/Direct and Neutral/Smooth/Responsive/Compensated presets;
- live Raw → Shaped → Applied telemetry and clipping/latency status;
- bounded Constant/Spring/Damper/Friction/Autocenter tests plus Emergency Neutralize using the existing OutputLease/watchdog/recovery path;
- mouse dragging, keyboard slider adjustment and native D5.8 accessibility peers for the new controls.

Safety boundary:
- hard NativeFFBConfig caps remain downstream and cannot be raised by D6 UI values;
- gain edits do not automatically start motor output;
- rotation is written live only when Native Wheel Output was already explicitly enabled;
- D6 does not alter physical certification/Stable evidence flags.

### Exact next action

1. Run 0.6.1-alpha on the real G27 and verify the Force Feedback tab layout at normal and scaled DPI.
2. Move every slider, restart LogiMate and verify per-wheel persistence.
3. With Native Output enabled, verify 270/360/540/720/900° range application and Emergency Stop.
4. Run each bounded FFB test at low strength and compare the live Raw/Shaped/Applied monitor.
5. Include the D6 controls in Narrator/UIA/High Contrast testing.
6. Feed real-hardware differences back into the existing wheel model adapters/capabilities only; do not introduce alternate writers.


---

## D6.1 — HID Sharing Fix & Control Panel UX — 0.6.1-alpha ✅ software / ⏳ real G27 re-test

A real G27 FFB Live-Test exposed an HID sharing violation. D6.1 keeps the protected writer as first choice but, on `ERROR_SHARING_VIOLATION` only, retries once with the cooperative `FILE_SHARE_READ | FILE_SHARE_WRITE` semantics used by OpenG27/HidSharp when no known competing wheel writer is running. Native Output activation and each bounded motor test now run a no-report HID preflight first. The FFB panel surfaces share mode, backend, fallback count and known external writer conflicts.

**Next physical action:** use the same G27/PC, enable Native Output, confirm `HID bereit` or `HID bereit · Shared`, then run the low-strength Constant/Spring/Damper/Friction/Autocenter tests. If access still fails, capture the new inline conflict/error detail before changing the transport again.


---

## D6.2 — FFB Verification / Control-Panel Rework — 0.6.2-alpha ✅ software / ⏳ real-wheel verification

A setting-by-setting audit found that D6.0/D6.1 persisted values correctly but did not always make the effect visible in the currently running path. D6.2 therefore verifies ownership and runtime scope instead of treating persistence as proof of effect.

Implemented:
- active Game FFB now receives Wheel-profile and D4 shaping changes through a runtime tuning refresh + revision counter;
- D4 Constant/Transient 0% is now a real zero rather than being normalized back to 100%, with v1 compatibility migration;
- Transient Gain is disabled until an adapter publishes a semantic Transient channel;
- Wheel-profile edits and D4 edits mark only their own layer `Custom`;
- the FFB panel is split into `Basis`, `Effekte`, `Signalformung`, `Live & Test`;
- active Game Profile gain layers are visible so multiplicative gain precedence is no longer hidden;
- manual Constant test uses the canonical wheelengine/OpenG27-lg4ff packet;
- Autocenter now sends SpringSet + SpringEnable and neutralizes with SpringOff/global stop;
- live monitor switches between Game telemetry and direct Hardware-Test requested/applied values;
- the 250 ms live timer invalidates only the monitor region and Win32 painting honors the dirty rectangle; UIA overlay geometry/text churn is reduced.

Truth boundary:
- Spring/Damper/Friction are real condition gains but the current telemetry adapter path is Constant-only; they are primarily verifiable through condition tests until an adapter publishes those channels.
- Alpha hard motor caps can clip upstream gain changes; clipping is expected safety behavior, not a reason to raise caps without evidence.
- physical G27/G25/DFGT effect behavior remains pending.

### Exact next action

Use the real G27 to run the D6.2 matrix in `docs/POST_D6_2_MASTER_AUDIT.md`: persistence with output off, HID preflight, rotation, low-strength Constant/Spring/Damper/Friction/Autocenter, Game gain 0/25/50/75/100, D4 shaping one control at a time, Emergency Stop, USB-yank/reconnect, then D5.9 stress. The best next UI expansion after this evidence is a `Profile` tab that makes Wheel profile + Game profile + D4 preset precedence explicit rather than adding another motor path.

---

## 0.0.1-alpha · Builds 002–007 — UX Foundation Rework

Build 001 remains the persistence/version stabilization baseline. Builds 002–007 complete the planned product-structure pass without changing the native motor architecture:

- **Build 002:** fixed Wheel tabs (`Live · Force Feedback · Kalibrierung · Profile · Gerät`); old catch-all Wheel action removed.
- **Build 003:** inline Calibration 2.0 for steering, pedals, buttons/paddles and H-shifter with valid-sample/session/source checks and final-only persistence.
- **Build 004:** Profile/effective-settings view makes Wheel/Game/D4 gain precedence and hard Safety caps explicit.
- **Build 005:** Diagnostics 2.0 centralizes active problems, tests, session log, export and Release Trust.
- **Build 006:** resumable 8-step first-run setup; successful migration no longer equals completed onboarding.
- **Build 007:** About reorganization, stale roadmap removal, duplicate-action cleanup and targeted non-flickering live invalidation.

### Exact next action after Build 007

Use the physical G27 as the first evidence target. Run native activation, all inputs/calibration, range, bounded FFB effects, Emergency Stop, USB-yank/reconnect, process-kill recovery, suspend/resume and Modern↔Legacy↔Modern. Feed only observed hardware differences back into the owning adapter/capability/transport layer. Keep the visible version pinned at `0.0.1-alpha`; advance internal BUILD only when another snapshot is needed.


---

## 0.0.1-alpha · Build 008 — Visual QA / Contrast / Layout

Build 008 is a presentation/reliability pass only; it does not add another HID writer or change motor ownership. The Win32 UI was audited for clipped text, weak semantic contrast, over-saturated accent surfaces and fixed-height content that could disappear on short windows.

Implemented:
- AA-level semantic text contrast targets for Dark/Gray/Light normal application surfaces;
- separate warning foreground/surface roles and contrast-aware decorative accents;
- fitted paragraphs/ellipsis for long dynamic text;
- adaptive compact info rows;
- taller Settings rows and group headers;
- scrollable Wheel Calibration/Profile/Device and inline calibration views;
- Diagnostics rows with multi-line details and height-aware visible counts;
- FFB help/status/selected-state readability fixes;
- Build 008 Windows regression tests for theme contrast and Settings geometry.

### Exact next action after Build 008

Run a visual acceptance matrix on the real Windows PC at 100/125/150/200% scaling in Dark/Gray/Light, including the five Wheel tabs, Settings, Diagnostics, Setup and About. Record any remaining font/driver-specific clipping with screenshots. Continue G27 hardware certification in parallel; do not raise FFB caps based on UI work.


## 0.0.1-alpha · Build 009 — FFB Live-Test Repair

Build 009 fixes the user-reported Force Feedback Live & Test buttons. The custom-painted buttons now use explicit press/capture/release activation, all manual motor tests replace any previous test through confirmed Emergency Neutralize, and Spring/Damper/Friction use the canonical wheelengine/OpenG27-lg4ff direct report family rather than the historical generic slot builders. Constant and Autocenter also participate in the same replace-current-test lifecycle.

### Build 009 validation boundary

- Software packet parity and Windows compile/vet gates: required.
- Real G27 motor response: still requires physical user validation.
- Hard NativeFFBConfig safety ceilings, OutputLease, watchdog, recovery marker and Emergency Neutralize remain in force.


---

## 0.0.1-alpha · Build 010 — G27 Unified HID Output

Real G27 testing after Build 009 showed that no motor effect reacted even though the UI path and per-effect report builders were active. The common output architecture was therefore audited against OpenG27's current G27 lifecycle. OpenG27 opens one non-exclusive HidStream and uses that exact stream for both continuous input and lg4ff output. LogiMate still had a read-only Direct-HID handle plus a separately opened output handle.

Build 010 converges the G27 C29B path onto one shared read/write session. Direct-HID input uses OVERLAPPED reads; native output borrows the same session and uses OVERLAPPED writes, with same-handle `HidD_SetOutputReport` compatibility only after a synchronous unsupported-write result. A G27 output request no longer falls back silently to an unrelated second handle. Ambiguous output I/O invalidates the complete session and requires reconnect.

The FFB panel's `Output prüfen` action now performs a non-motor RPM-LED pulse after the logical preflight, giving real physical evidence that reports reach the G27 before Constant/Spring/Damper/Friction/Autocenter are attempted.

### Build 010 validation boundary

- Software architecture/build gates: required before packaging.
- Physical G27 LED ping and motor-effect response: **PENDING user hardware validation**.
- Safety caps are unchanged; this build changes transport/session ownership, not permitted motor strength.
- G25/DFGT retain their existing transport until their physical certification justifies equivalent changes.


---

## 0.0.1-alpha · Build 011 — FFB Slot Activation & Adjustable Test Strength

Physical Build-010 evidence is now concrete: the user confirmed that `Output prüfen` flashes the G27 RPM LEDs. That proves the unified C29B HID session, report framing and write path reach the real wheel. The remaining failure was effect activation: only the special Spring transaction produced a brief response.

Build 011 therefore replaces the manual effect layer with the authoritative new-lg4ff slot lifecycle. Constant uses slot 0; Spring, Damper and Friction use independent slots 1, 2 and 3 with distinct start/update/stop opcodes. The manual test slider is a direct 1–30% hardware-diagnostic request (15% default) rather than being scaled again through Wheel/Game gains. Normal game/persistent FFB caps are unchanged. Autocenter's FE/0D clip field is corrected from the former `ramp=2` value to `0x80`.

### Build 011 physical validation boundary

1. `Output prüfen` must still flash the LEDs.
2. Constant at 15%, then 30% should pull consistently in one direction.
3. Spring should provide a centering/restoring force.
4. Damper and Friction must be judged while physically moving the wheel; neither is expected to create a static directional pull.
5. Autocenter should now be perceptible with the corrected clip.
6. STOP/Emergency Neutralize must remove every effect immediately.
7. If a specific slot still fails, capture the Build-011 diagnostic status/transport metrics and fix that slot only; do not reopen the transport architecture.
