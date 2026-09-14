# LogiMate Native Engine — Path C → D roadmap

The project will ship a testable ZIP after every numbered fusion checkpoint.

## Path C — controlled OpenG27 Core port

### C1 — Pure Core parity — 0.2.1-alpha
- Provenance/license ledger.
- ForceScaling + ForceFrame/ForceMixer port.
- Centered steering-axis calibration port.
- Pedal calibration port.
- RangeTracker port.
- Hardware-independent parity tests and in-app self-test hooks.
- **No new motor/HID behavior.**

### C2 — lg4ff report parity — 0.2.2-alpha ✅
- Port OpenG27 `Lg4ffReports` into an isolated Go reference module.
- Golden vectors for native switch, range, LEDs, constant force and condition effects.
- Compare against LogiMate's existing builders byte-for-byte.
- Shadow/Dry Run only; no behavior changes until parity is explicit.
- Implemented Protocol Shadow reports in Engine Health/diagnostics; known live-builder differences are recorded rather than silently promoted.

### C3 — FFB scheduler parity — 0.2.4-alpha ✅
- Ported OpenG27 `IFfbSource`, `FfbEngine`, `WheelOutput` and `WheelOutputOptions` behavior to native Go.
- Preserves upstream parity cadence at 6 ms and deterministic `PumpOnce` for tests.
- Preserves LogiMate generation IDs, single owner, hard force limits, converted slew-rate budget, watchdog and crash marker.
- Added hardware-free dry-run plus scheduler/write/watchdog telemetry.
- Added a separate G27-only live test at ±2/±5% with a hard 900 ms stop; existing LogiMate FFB remains available for A/B comparison.

### C4 — Device/report parser parity — 0.2.5-alpha ✅
- Ported OpenG27 `G27ReportParser` into the isolated Go parity core.
- Every production G27 Direct-HID report is shadow-parsed and counted as MATCH/DIFF/error.
- Mirrored OpenG27 `G27Device` lifecycle policy: prefer C29B, switch C294 only when native is absent.
- Kept LogiMate StableWheelID, exact target correlation, multi-wheel fail-closed behavior and reconnect backoff as authoritative.
- Fixed Engine Health to refresh live every 500 ms instead of showing a static snapshot.
- Transport unification across G25/G27/DFGT remains a Path-D generalization task; C4 does not weaken current model-specific safety gates.

### C5 — Profiles/import — 0.2.6-alpha ✅
- Port GameProfile matching semantics.
- Read/import OpenG27 config and profiles into LogiMate's StableWheelID schema.
- Never overwrite existing calibration without explicit user confirmation.

### C6 — Wreckfest 2 adapter — 0.2.7-alpha ✅
- Port Pino telemetry parser with golden packet fixtures.
- RPM LEDs.
- Player-control/physics gating.
- Telemetry FFB routed through LogiMate's safety mixer.

### C7 — Native cutover — 0.2.8-alpha ✅
- Added C1–C6 hardware-free cutover gate plus live PASS/WARN/BLOCK readiness.
- Promoted Windows Generic HID + LogiMate Native to the normal G27 Modern runtime.
- Removed automatic OpenG27 download/start from the G27 Modern migration path.
- External OpenG27 remains an explicit fallback/A-B oracle under single-owner safety handoff.
- Kept old C2 builder differences and the not-yet-complete physical G27 C3/C6 matrix visible as WARN instead of inventing parity evidence.
- Path C is functionally complete; remaining physical certification continues as a release gate while development moves to Path D.

### C8 — Reliability & Architecture Cleanup — 0.2.9-alpha ✅
- Remediates the complete C7 A-001…A-100 audit before new feature expansion.
- Centralizes native output ownership with a fail-closed lease/mutex and durable crash-recovery evidence.
- Hardens C294 identity, SetupAPI/Raw Input, Direct-HID sample validity, calibration persistence and multi-wheel correlation.
- Makes Modern/Legacy migrations journal-required with verified terminal states/rollback.
- Fixes LED-only/Pino FFB authorization and configuration-corruption handling.
- Hardens update/uninstall trust/ownership and aligns VERSION/local/CI/release packaging.
- Adds enforceable Stable hardware/signing gates. C8 is software-complete Alpha, not physical Stable certification.

## D0 — Native Engine Foundation — 0.3.0-alpha ✅
- Added platform-independent `internal/wheelengine` typed model/device/input/output/health state and descriptors.
- G25/G27/DFGT share model capabilities, one Modern migration transaction and one Native output ownership/transport boundary.
- Direct input is projected into canonical per-wheel/session state; stale state is invalidated on session changes.
- Wreckfest Pino runtime identity is LogiMate-native (`wreckfest2-pino`) with a backward-compatible old alias.
- Game FFB is capability-driven across the three integrated models; G27-only LEDs remain capability-gated.
- External OpenG27/LGS is not required for normal Modern operation. OpenG27-derived source remains attributed reference code.
- The serialized HID transport has a deadline/cancellation/poison boundary; promotion to a different overlapped write primitive is deferred until physical model validation rather than changed blindly.

## Path D — LogiMate-native evolution

### D1 — Model certification & refinement — 0.3.1-alpha ✅ software refinement
- Added explicit G25/G27/DFGT model adapters for native PID/selector, input layout and advertised capabilities.
- Moved model-specific report decoding into platform-independent fixtures and corrected Direct-HID source/layout metadata.
- Added evidence-based per-model certification requirements and Stable gates.
- **Physical certification remains pending** for real G25/G27/DFGT hardware; D1 software completion does not claim motor certification. Any discovered hardware difference must still be encoded as an adapter/capability plus fixture.

### D2 — Unified native wheel engine — 0.3.2-alpha ✅ software consolidation
- Productive classic Logitech protocol builders, FFB scheduler and Wreckfest/Pino parser now live in `internal/wheelengine`.
- Productive Modern Direct-HID decoding, game FFB and telemetry no longer use `internal/openg27port`.
- `internal/openg27port` remains an attributed reference/parity/import boundary so equivalence can be tested without making it a runtime dependency.
- The duplicate historical C3 live writer is removed from normal UI; one capability-driven Native Game Output path serves G25/G27/DFGT.
- Known wire-level differences without hardware evidence are not guessed away; they remain explicit certification items.

### D3 — Adapter ecosystem — 0.3.3-alpha ✅
- Added typed `internal/gameadapter` interface/registry with capabilities, lifecycle, normalized frames and health.
- Migrated Wreckfest 2 / Pino and Local JSON to the same contract.
- Native Game Output is adapter-capability-driven and contains no Pino-specific motor path.
- Added packet/frame rate, last-frame age, setup guidance and adapter catalog diagnostics.
- Added game-profile schema v2/future-version write protection and retained legacy adapter IDs only as migrations.
- Architecture guard tests prohibit direct wheel HID primitives inside adapter implementations.
- Future games/SDKs must enter through this contract; no game-specific motor bypass is allowed.

### D4 — Advanced FFB — 0.4.0-alpha ✅ software complete
- Added pure tested force shaping: deadband/rescale, minimum force, response curves, low-pass filtering, smoothing and semantic constant/transient gains.
- Added raw/shaped/applied force, pipeline-vs-safety clipping and latency diagnostics.
- Added per-wheel presets plus fully individual Constant/Spring/Damper/Friction custom tuning.
- Preserved the hard safety mixer as the final ceiling after all D4 shaping.
- Preferred HID output now uses true OVERLAPPED `WriteFile` + exact-operation `CancelIoEx`; synchronous pre-queue incompatibility may use the bounded HidD compatibility backend.
- Physical feel/transport certification remains pending per G25/G27/DFGT and still gates Stable.

### D5 — Modern-mode independence — 0.5.0-alpha ✅ software complete
- Production `cmd/logimate` has no dependency on `internal/openg27port`; a build gate prevents regression.
- Normal UI/runtime no longer discovers, downloads, installs, launches or polls OpenG27.
- Native G27 input defaults are LogiMate-owned; external OpenG27 config files no longer influence live input.
- Old OpenG27 game-profile files remain available only as explicit offline Legacy import/migration data.
- Output ownership relies on LogiMate's lease/mutex/exclusive HID writer instead of external process-name interlocks.
- Legacy LGS/Profiler remains available as a deliberate first-class alternative mode.
- D5 completes software-side standalone Modern operation; Stable still requires the physical G25/G27/DFGT certification matrix and remaining external release gates.

### D5.2 — OpenG27-style Detection — 0.5.2-alpha ✅ software / ⏳ physical G27 validation
- Direct HID is now the primary discovery source and candidates are ranked by report capabilities.
- PnP and Raw Input enrich one coherent snapshot instead of racing separate refreshes.
- C294/native transitions use a bounded settle/reopen lifecycle.
- Explicit HID product identity and safe previous-native bindings improve G27/G25 reconnect behavior without treating every C294 as G27.
- Physical G27 validation is the immediate next gate.

### D5.3 — G27 Native Activation Recovery — 0.5.3-alpha ✅ software / ⏳ physical G27 validation
- First-run confirmed G27 C294 can auto-promote without a previously saved Modern preference.
- Explicit Legacy mode remains the hard opt-out.
- G27 native switch is byte-for-byte aligned with OpenG27's proven single-report sequence before C29B re-enumeration.
- Next step: physical G27 certification on C294→C29B, input, pedals/shifter/buttons and bounded FFB tests.


### D5.4 — OpenG27-Compatible G27 Handshake — 0.5.4-alpha ✅ software / ⏳ physical G27 validation

D5.4 is a targeted real-hardware recovery step after D5.2/D5.3: the G27 is already visible as C294 on HID, so LogiMate now mirrors OpenG27/HidSharp's shared Windows handle and 3-second WriteFile handshake and exposes an explicit Connect-style rescue action.

### D5.5 — Automatic HID Evidence Fusion Fix — 0.5.5-alpha ✅ software / ⏳ physical G27 validation

Real-hardware D5.4 testing proved the G27 HID interface was visible and the manual-confirmation path could proceed, but automatic confirmation still did not. Root cause: duplicate SetupAPI/HID devnodes discarded the HID product/serial metadata during evidence fusion. D5.5 merges that metadata into the authoritative PnP record, allowing `G27 Racing Wheel` to reach the existing safe session-only model consensus and queue native activation automatically.

### D5.7 — Hardware Certification Assistant — 0.5.7-alpha ✅ software / ⏳ physical matrix

- Added per-wheel, evidence-driven G25/G27/DFGT certification inside LogiMate.
- Guided checks cover native PID, all inputs, range/effects, game FFB, reconnect, USB removal, process kill, suspend/resume and Modern↔Legacy rollback.
- Certification progress is visible in the existing Wheel Advanced View and can be exported as Markdown + JSON.
- Local evidence never promotes Stable automatically; repository certification flags remain review-gated.
- Immediate next action: run the full G27 matrix and keep automatic C294→C29B as FAIL until it works without manual confirmation.


### D5.8 — Accessibility / UI Automation Foundation — 0.5.8-alpha ✅ software / ⏳ Windows UIA matrix
- Native Windows accessibility peers cover the custom-painted main-window navigation, actions, Settings and Wheel Advanced View.
- UIA/keyboard/mouse share the same action path; Diagnostics reports bridge state.
- Repository accessibility certification remains false pending real Narrator/UIA, High Contrast, reduced-motion, mixed-DPI and secondary-dialog validation.


### D5.9 — HID Stress / Adverse-I/O Hardening — 0.5.9-alpha ✅ software / ⏳ physical stress matrix
- Adds native HID transport lifecycle/error/latency metrics.
- Partial writes and failed completion confirmation now poison/close the transport fail-closed.
- Adds deterministic policy stress, a guided 100-cycle non-motor live stress run and a guided USB-yank adverse-I/O window.
- Exports physical/runtime evidence without auto-promoting the repository Stable flag.
- `hidStressValidated` remains false until reviewed real G25/G27/DFGT evidence exists.

### D5.10 — Release Trust / Authenticode Hardening — 0.5.10-alpha ✅ software / ⏳ external Stable signing

- Runtime Authenticode identity is visible in Release Trust, Engine Health and diagnostics.
- Stable VERSION must exactly match the reviewed certification manifest before release signing may start.
- Stable signs/verifies LogiMate.exe before installer embedding, then signs/verifies the final installer and requires the same publisher thumbprint.
- RFC3161 timestamp configuration and Windows SignTool are mandatory for Stable.
- RELEASE_ATTESTATION.json records final hashes, signature identity and certification state.
- Preview builds may remain unsigned, but can never satisfy the Stable trust gate.


## D6 — Wheel Control Panel / Force Feedback UI — 0.6.1-alpha
- Adds an in-page `Force Feedback` tab to the existing Wheel page instead of a new main destination.
- Exposes Native Engine profile gains/range and D4 Advanced FFB shaping as direct visual sliders.
- Adds profile/pipeline presets, live force telemetry and bounded existing-engine effect tests.
- Reuses the single OutputLease/watchdog/recovery path; no new motor writer is introduced.
- Extends keyboard/UI Automation reachability to the new control surface.


## D6.2 — FFB Verification / Control-Panel Rework — 0.6.2-alpha
- Verifies every D6 setting against its actual runtime scope instead of equating persistence with effect.
- Adds live retuning for active Game FFB and true zero Game Constant semantics.
- Separates Basis/condition effects/game signal shaping/live tests.
- Fixes canonical manual Constant and Autocenter Set+Enable/Off sequences.
- Adds dirty-region live painting to prevent 250-ms full-window flicker.
- Next evidence gate is the real G27 D6.2 matrix; next recommended UI layer is Profile/effective-settings precedence.
