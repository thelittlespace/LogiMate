# Architecture — LogiMate 0.0.1-alpha

LogiMate's Modern architecture is a self-contained Windows wheel runtime for the integrated classic Logitech models. External OpenG27 and Logitech LGS/Profiler are not runtime dependencies of Modern mode.

## Layers

```text
Win32 UI / setup / diagnostics
            │
            ▼
       system facade
            │
            ▼
      wheelengine core
  ┌─────────┼──────────┐
  │         │          │
Device    Input      Output/Safety
Registry  State      Lease + Transport
  │         │          │
  └──── profiles / adapters ────┘
            │
            ▼
 SetupAPI / Raw Input / WinMM / Windows HID
```

### `internal/wheelengine`

Platform-independent canonical state and model capabilities. It owns typed model/mode IDs, descriptors, device/input/output/health structures and runtime generation/session semantics.

### `internal/gameadapter`

Platform-independent semantic game/telemetry adapter boundary introduced by D3. An adapter may discover/start/stop its own game-side data source and publish normalized frames plus health. It must never open wheel HID, acquire motor ownership or emit Logitech reports.

### `internal/system`

Windows integration and the compatibility facade used by the current UI. It still contains historical compatibility/diagnostic file names (`g27_shared_hid`, `classic_shared_hid`, `fusion_c*`), but D2 routes productive model decoding, protocol building, game telemetry and FFB scheduling through the common wheel engine. Historical Fusion modules may compare against reference behavior but do not own the productive runtime.

### `internal/openg27port`

Attributed OpenG27-derived/reference logic retained under the upstream MIT terms. It is removed from the production executable dependency graph entirely. It exists only as source/test provenance and a parity oracle. Legacy OpenG27 profile import is parsed by LogiMate-owned compatibility structures in `internal/system`, so the shipped runtime does not need this package. Provenance remains tracked in `OPENG27_PROVENANCE.md` and `THIRD_PARTY_NOTICES.md`.

## Device identity

Windows PnP/SetupAPI establishes a real device. Native PIDs C299/C29A/C29B are authoritative model identities. C294 is shared compatibility mode and requires explicit current-device confirmation unless a stronger identity is available.

A runtime device carries StableID, SessionID and optional hardware fingerprint. Session changes invalidate stale input/output state.

## Input

Direct HID is preferred for supported native devices. WinMM is an internal compatibility fallback. Samples are canonicalized with source/layout/session/generation/time/validity metadata before the engine accepts them.

Calibration is scoped to the actual physical wheel/source/layout/session context and cannot learn synthetic pre-first-report values.

## Output and safety

All productive hardware writes converge on the Native output ownership boundary:

1. validate selected model/mode/PnP state;
2. acquire the central in-process lease and named Windows mutex;
3. correlate exactly one HID output path;
4. for motor output, persist the crash-recovery marker before force;
5. serialize reports through `nativeHIDTransport`;
6. stop/neutralize;
7. clear recovery evidence and release ownership only when completion is proven.

If stop/write completion cannot be proven, the state remains fail-closed rather than authorizing a replacement writer.

`nativeHIDTransport` denies `FILE_SHARE_WRITE`, serializes writes and prefers a true OVERLAPPED `WriteFile` operation with an exact-operation `CancelIoEx` deadline path. Any ambiguous pending write poisons the handle fail-closed. If a classic HID stack synchronously rejects the OVERLAPPED path before an operation is queued, LogiMate may reopen the same selected interface using a bounded `HidD_SetOutputReport` compatibility backend. The active backend is exposed in advanced diagnostics; physical model validation is still required for Stable.

## Capabilities

Model-specific behavior is data-driven where possible. G27-only Rev LEDs are guarded by `RPMLEDs`; DFGT has no clutch/H-shifter; all three currently expose the shared native FFB/range effect family at the software level. Physical certification is recorded separately from capability implementation.

## Modern migration

All supported models use the same journaled Modern transaction. There is no G27-only destructive path. Native selector/PID comes from the model descriptor. Backup, target verification, terminal postconditions and rollback rules are common.

## Profiles and adapters

Game profiles are LogiMate-native and use schema 2 in D3. Telemetry adapters implement one typed contract: descriptor/capabilities, start, stop, health, snapshot and normalized frames. The Wreckfest 2 Pino adapter is `wreckfest2-pino`; the old `openg27-pino` value is a read-compatible migration alias. Local JSON is the second reference implementation.

The adapter layer owns **game-side I/O only**. It publishes semantic values such as RPM, normalized force, physics/player-control state and FFB authorization. Native Game Output consumes those values and remains the only route to wheel output. HID, output leases, recovery markers and Logitech protocol reports are therefore impossible to require as part of an adapter implementation without violating package architecture.

Game FFB uses the common output lease. Rev LEDs are written only if both wheel and adapter capabilities allow them. LED-only sessions never acquire a motor lease.


## Advanced FFB / signal-shaping pipeline

Game/telemetry adapters still publish normalized semantic force only. The engine inserts a platform-independent shaping stage between semantic force and the existing hard safety mixer:

```text
adapter force → game gain/invert → D4 shaping → engine-profile gains → hard NativeFFBConfig ceiling → slew/watchdog → HID
```

Signal shaping supports deadband/rescale, response curves, optional minimum force, low-pass filtering, smoothing, semantic constant/transient gain and a normalized pre-safety output limit. Because `MixNativeEffects` remains after this stage, no shaping preset can increase the final hardware ceiling. Diagnostics distinguish pipeline clipping from safety clipping and track adapter-to-pipeline latency.

## External software boundary

### Not required in Modern mode

- OpenG27 executable;
- Logitech Profiler/LGS;
- separate calibration/FFB/profile utility.

### Optional/compatibility only

- explicit offline import of old OpenG27 game-profile files;
- LGS/Profiler when deliberately selecting Legacy mode.

There is no OpenG27 executable discovery, downloader, launcher or runtime process dependency in production. The production dependency graph is gated so `cmd/logimate` cannot depend on `internal/openg27port`.

## Privilege model

The normal UI is non-elevated. UAC is requested for explicit driver/security operations. Migration journals are mandatory and terminal states/rollback are verified before success is recorded.

## Release trust

Alpha can be unsigned. Stable remains blocked until machine-readable hardware/recovery/migration/UI/HID-stress certification is complete and Authenticode signing is verified.


## Detection evidence fusion

Wheel presence is now resolved from three independent Windows paths: all-class SetupAPI devnodes, direct SetupAPI HID-interface enumeration, and Raw Input HID paths. Native C299/C29A/C29B remains authoritative. For a single C294 G25/G27, matching read-only PnP + explicit WinMM model evidence may authorize the current session only; it is never persisted as port-only hardware identity. The G27 direct reader uses shared read/write access so LogiMate's own input and serialized output handles can coexist.

## Detection boundary

Wheel **visibility** begins at the HID interface, not at WinMM or Raw Input. The discovery stack is:

`SetupAPI HID interfaces + HID caps → PnP/USB correlation → native PID/model → Raw Input corroboration → WinMM fallback`.

A short settle loop prevents C294/native USB re-enumeration from being turned into a false disconnect. Native-mode re-open also uses direct HID discovery first. Model-specific output remains gated by a confirmed logical model and selected PnP target; visibility alone never authorizes hardware writes.

## Physical certification boundary

Hardware certification is intentionally outside the normal runtime ownership graph. `internal/system/hardware_certification_windows.go` records evidence about the existing wheelengine/input/output paths; it does not create a second transport, FFB engine or device manager. The assistant in `internal/app/hardware_certification_windows.go` reuses the same selected wheel, canonical input state, Output Lease, Watchdog and Emergency Stop paths as normal LogiMate Native operation.

Local evidence is stored under `LogiMateData\\Certification` and is never allowed to mutate repository Stable certification flags automatically.
## Accessibility / UI Automation boundary

The main window remains custom-painted, but D5.8 overlays transparent native Windows child controls for accessibility only. Standard `BUTTON`/CheckBox/RadioButton HWNDs provide the Windows MSAA/UI Automation semantics for navigation, page actions, theme/settings toggles and Wheel Advanced View. They do not own visual painting and return `HTTRANSPARENT` for pointer hit-testing, so existing mouse behavior remains on the buffered parent surface. Keyboard/UIA invocation is routed back through the same LogiMate action functions.

A native disabled STATIC surface exposes the current page title/subtitle/live body text to accessibility clients. The bridge reports runtime readiness in Diagnostics and Engine Health. This is deliberately a software foundation: repository `uiAccessibilityValidated` remains false until Windows Narrator/UIA, mixed-DPI, High Contrast, reduced-motion and dialog behavior have been manually/physically validated.



## HID adverse-I/O observability

The native HID writer remains the only productive low-level output transport. The runtime adds process-lifetime transport metrics and strengthens ambiguous completion handling: partial writes and completion-query failures poison/close the handle, while compatibility fallback is permitted only after a synchronous error proves no OVERLAPPED operation was queued. The live HID stress probe acquires the same central non-motor output lease as normal range/LED operations and uses only an idempotent range report; it never owns a second writer path.

## Release trust / signing boundary

Stable publication is now a build-time trust contract. The application is Authenticode-signed and verified before becoming the installer payload; the final installer is then signed and verified independently. Stable packaging requires the reviewed certification manifest to match the exact release version and requires app/installer to match the configured publisher certificate thumbprint. `RELEASE_ATTESTATION.json` records final hashes and trust state. Pre-release builds may remain unsigned, but runtime diagnostics identify that state and it can never satisfy the Stable gate. Private-key custody remains outside LogiMate/source control.


## D6 Wheel Control Panel boundary

The UI is a presentation/configuration layer over the existing Native Engine and signal-shaping pipeline. The Wheel page writes only the established per-wheel Native Engine profile and Advanced FFB configuration stores. It introduces no HID transport, scheduler, recovery marker, output lease or motor writer.

The force path remains:

`adapter force → signal shaping → Native Engine profile gains → hard NativeFFBConfig safety mixer → slew/watchdog → single HID transport`

The D6 bounded test controls call the existing Constant/Spring/Damper/Friction/Autocenter entry points and central Emergency Stop. Slider changes never directly emit motor packets. Range changes use the existing transactional Native Engine profile/range application only when Native Wheel Output is already explicitly authorized.


## D6.2 live-tuning and rendering boundary

Runtime configuration keeps the existing force order but makes configuration updates explicit at runtime. `RefreshNativeGameOutputTuning` swaps the running telemetry source to the currently persisted Native Engine profile and atomically resets the existing signal-shaping pipeline with the new tuning; it does not create a scheduler, HID handle or output lease. Rotation remains a separate transactional hardware command.

The manual Constant live test now uses the same canonical `wheelengine.ConstantForce` protocol family as productive Game FFB. Autocenter uses the canonical SpringSet + SpringEnable sequence and SpringOff during neutralization. Historical packet builders remain only where needed for reference/compatibility diagnostics.

The Wheel live timer is decoupled from full-window painting. FFB `Live & Test` invalidates only its live monitor rectangle; the custom double-buffered renderer clips to `PAINTSTRUCT.rcPaint` and blits only the dirty area. UI Automation child geometry is updated only when changed, and rapidly changing Wheel status text is throttled for accessibility publication.

## 0.0.1-alpha Build 007 UI ownership boundary

The Build 002–007 rework changes presentation/flow only. `Wheel/Live`, `Wheel/Force Feedback`, `Wheel/Kalibrierung`, `Wheel/Profile` and `Wheel/Gerät` are views over the existing selected-wheel state and audited persistence APIs. Calibration wizards buffer capture state in the UI and call the existing system persistence functions only after a complete valid sequence. Diagnostics aggregates existing subsystem state and UI notices; it does not become a new device owner. Setup 2.0 invokes the same tracked migration actions and resumes into the normal Wheel views. No Build 002–007 view opens a second motor writer or bypasses OutputLease, recovery markers or `NativeFFBConfig` safety limits.


## 0.0.1-alpha Build 008 visual rendering boundary

Presentation fixes remain in `internal/app`; the device/FFB engine is unchanged. Theme semantic colors must maintain readable foreground/background separation, decorative fixed accent colors must pass through `readableTextColor` when used as text, and dynamic prose should use fitted/wrapped rendering rather than assuming one fixed Win32 line. Wheel subviews may scroll independently through the shared content scroll state; this must not affect HID/session ownership.
