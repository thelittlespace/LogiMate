# Changelog

## 0.0.1-alpha · Build 018 — Full Re-Audit / Release Hardening

- Re-audited the verified Build 017 baseline across installer, updater, lifecycle, FFB safety, diagnostics/privacy, UI/accessibility, CI and reproducibility boundaries.
- Removed every productive path that could change Windows Memory Integrity/HVCI; the status is now an explicit link to Windows Security and never toggles the setting.
- Hardened portable-update target authorization, managed uninstall boundaries, HTTPS-only external links and final diagnostics redaction.
- Fixed refresh/setup state synchronization issues, including a duplicate-lock regression found by the Build 018 audit.
- Pinned the release compiler to exact stable Go 1.27.1 and the release shell to exact PowerShell 7.6.6.
- Added verified portable release bootstrap with official upstream SHA-256 checks and `GOTOOLCHAIN=local`.
- Added toolchain identity to `RELEASE_ATTESTATION.json`; release output cannot claim Go 1.27.1 unless that exact compiler actually ran.
- Added double-build byte reproducibility enforcement for the alpha application and installer in the Windows release bootstrap.
- Visible SemVer remains `0.0.1-alpha`; internal build identity is `018`.

## 0.0.1-alpha · Build 016 — Accessibility / Setup / Diagnostics

- Fixed UI Automation activation for Calibration, Profiles and Device wheel tabs.
- Added keyboard/UIA coverage for Wheel subpage actions and Diagnostics tabs/actions.
- Dashboard now shows the exact paused setup-resume step.
- Preserved Diagnostics lifecycle and hardware-certification status as first-class user-visible state.

## 0.0.1-alpha · Build 015 — Hardware Certification

- Recorded real G27 LED + FFB evidence without claiming full model certification.
- Added a guided Constant/Spring/Damper/Friction/Autocenter certification suite at 15%.
- Certification result write failures now surface in Diagnostics instead of being ignored.
- Diagnostics shows current physical certification progress for the selected wheel.

## 0.0.1-alpha · Build 014 — Reliability / Lifecycle

- Suspend/standby now neutralizes output and closes live HID sessions.
- Resume invalidates stale HID/PnP state and schedules a clean rebind without auto-starting FFB.
- Lifecycle transitions are surfaced in Diagnostics.
- Added Windows power-broadcast regression coverage.

## 0.0.1-alpha · Build 013 — Responsive / DPI / Visual QA

- Capped high-DPI minimum window size to the usable physical desktop.
- Narrow windows now force the compact navigation rail.
- Content layout no longer creates off-screen minimum widths.
- Resize/DPI transitions clamp scrolling and keep content reachable.
- Added Build 013 responsive-layout regression tests.

## 0.0.1-alpha · Build 012 — Master Audit & GitHub Readiness

- Canonical FFB protocol helpers and self-tests now match the hardware-confirmed G27 slot map.
- Diagnostic/hardware export privacy hardened; stable identifiers are redacted in public evidence.
- Legacy wheel-model preference persistence now fails closed on malformed/future files.
- Unsigned alpha updater no longer auto-downloads an artifact it cannot safely verify/apply.
- Public GitHub scaffolding, CI, contribution/security/support docs and publication checklist added.
- High-visibility historical phase/version jargon reduced; visible application version remains 0.0.1-alpha.

# 0.0.1-alpha · Build 011 — FFB Slot Activation & Adjustable Test Strength

- Real G27 evidence from Build 010 confirmed the unified HID output path: the RPM LED ping reaches the wheel.
- Reworked manual Constant Force from the simplified direct packet to the real new-lg4ff slot-0 lifecycle: start `0x11`, update `0x1C`, stop `0x13`.
- Reworked condition live tests to the authoritative new-lg4ff slot map: Spring slot 1 (`0x21/0x2C/0x23`), Damper slot 2 (`0x41/0x4C/0x43`), Friction slot 3 (`0x81/0x8C/0x83`).
- Corrected the convenience Damper/Friction builders so they no longer accidentally address Spring slot 1.
- Manual hardware tests now send the lg4ff fixed-loop-mode preamble before slot activation to make firmware state deterministic.
- Test strength is now directly adjustable from **1–30%**, defaults to **15%**, and is no longer multiplied by Wheel/Game profile gains. The value shown in the test slider is the actual diagnostic request.
- Normal game FFB safety/profile caps are unchanged; only the explicitly user-started, short manual hardware test has the dedicated 30% ceiling.
- Manual tests are bounded to 3 seconds and still use OutputLease, recovery marker, transport fail-closed handling and Emergency Neutralize.
- Fixed Autocenter FE/0D encoding: byte 4 is now a usable `0x80` clip/saturation value instead of the old UI `ramp=2`, which made the effect effectively imperceptible.
- Added Build 011 packet tests for all four lg4ff slots, Autocenter clip semantics and the 1–30% UI test-strength range.
- Visible SemVer remains `0.0.1-alpha`; internal build identity is `011`.

# 0.0.1-alpha · Build 010 — G27 Unified HID Output

- Reworked the real G27 C29B lifecycle so Direct-HID input and native output borrow one shared read/write device session instead of opening an independent motor-writer handle.
- The shared G27 handle now opens with `GENERIC_READ | GENERIC_WRITE`, full read/write sharing and OVERLAPPED I/O; input reads and FFB writes can coexist on the same native session.
- G27 native output is fail-closed: if the unified session cannot be established, motor/LED output is blocked instead of silently falling back to a detached writer.
- The preferred G27 shared backend uses OVERLAPPED `WriteFile`; a synchronous unsupported-write result may fall back on the same handle to `HidD_SetOutputReport` without opening a second device session.
- Ambiguous shared-output I/O invalidates the complete G27 session and forces a fresh reconnect, preserving the existing OutputLease/watchdog/recovery safety model.
- `Output prüfen` now performs a harmless physical G27 RPM-LED pulse after the logical preflight. This distinguishes “Windows accepted a handle” from “an output report really reached the wheel.”
- Force Feedback UI now reports `G27 Unified Session` / `g27-shared-overlapped` (or same-session HID compatibility fallback) instead of a generic shared writer.
- Added Build 010 regression coverage ensuring output wrappers borrow the live G27 session and never close the input-owned handle.
- Visible SemVer remains `0.0.1-alpha`; internal build identity is `010`.

# 0.0.1-alpha · Build 009 — FFB Live-Test Repair

- Repaired the custom-painted FFB Live & Test button activation path with explicit press/capture/release routing instead of a release-only hit test.
- Every manual motor test now replaces the previous one through confirmed Emergency Neutralize before acquiring a fresh OutputLease.
- Constant and Autocenter can now follow active condition/autocenter tests without leaving a stale lease that makes the next button appear dead.
- Spring, Damper and Friction live tests now use the canonical LogiMate wheelengine/OpenG27-lg4ff packet family instead of the historical generic slot builders.
- Spring sends Set + Enable and SpringOff; Damper/Friction use their canonical start/update/off packets.
- Added explicit start/error/success feedback and active-test highlighting in the Control Panel.
- Added Build 009 regression tests for all condition packet families and six independent Live/Test hit targets.
- Visible SemVer remains `0.0.1-alpha`; internal build identity is `009`.

# 0.0.1-alpha · Build 008 — Visual QA / Contrast / Layout

- Audited all current Win32 theme roles and raised Dark/Gray/Light secondary/warning text to WCAG-AA contrast against the normal application surfaces.
- Split warning foreground from warning surface so warning text is no longer brown-on-brown or pale-on-pale.
- Added readable accent fallback for fixed project colors and selected/chip states across Dark, Gray and Light.
- Fixed several accent-tinted cards that accidentally used nearly the same foreground and background color.
- Fixed the common card accent stripe paint order, so the stripe is no longer covered by the card surface.
- Added adaptive compact/two-line info rows, fitted paragraphs and ellipsis handling for long runtime/device/profile text.
- Increased Settings row/group height and changed descriptions to fitted multi-line text instead of clipped one-line labels.
- Added scrolling to Wheel Calibration/Profile/Device panels and the inline calibration wizard for short windows.
- Reworked Diagnostics problem/test/log rows so details get real multi-line space and visible rows adapt to available height.
- Improved FFB notes/status/help text, selected-tab contrast and long Live/Test descriptions.
- Hardened About, first-run setup, changelog, sidebar/header and action text against truncation.
- Added Build 008 Windows visual-QA regression tests for semantic theme contrast and minimum Settings text space.
- Visible SemVer remains `0.0.1-alpha`; internal build identity is `008`.

# 0.0.1-alpha · Build 007 — UX Foundation Rework

- Build 002: replaced the wheel catch-all workflow with fixed `Live · Force Feedback · Kalibrierung · Profile · Gerät` tabs.
- Build 003: added non-modal inline calibration for steering, pedals, buttons/paddles and H-shifter; captures remain session/source validated and commit only after a complete valid sequence.
- Build 004: added Profile/effective-settings view showing Wheel, Game and D4 shaping precedence plus hard safety caps.
- Build 005: replaced the old diagnostics cards with a six-view diagnostics center (`Übersicht · Probleme · Tests · Protokoll · Export · Release`) and session diagnostic events.
- Build 006: replaced the old 3-page first-run text flow with a resumable 8-step setup workflow; migration no longer marks setup complete before input/calibration/FFB follow-up.
- Build 007: restructured About, removed stale roadmap copy, reduced duplicate actions and stopped 250-ms live refresh from repainting static Wheel configuration views.
- Fixed Effective Settings display so an intentional D4 Constant gain of 0 % remains 0 % instead of being presented as 100 %.
- Visible SemVer remains pinned at `0.0.1-alpha`; internal build identity is now `007`.

# 0.0.1-alpha · Build 001 — Stabilization Baseline

- Completed first technical stabilization audit pass across versioning, updater, setup state, calibration capture and mutable configuration persistence.
- Calibration/learning now rejects invalid, malformed or not-yet-sampled input instead of persisting synthetic defaults.
- Input, Native Engine, Native FFB, wheel-confirmation and native-identity stores now preserve corrupt/future data rather than silently overwriting it.
- `settings.json` now has a schema boundary; future settings become write-protected until an explicit reset.
- User-facing version is intentionally frozen at **0.0.1-alpha**. Future stabilization snapshots increment `BUILD`, not SemVer.
- Added separate build identity for diagnostics, updater bookkeeping and reproducible support reports.
- Updater is pinned to `0.0.1-alpha.N` and ignores historical internal 0.5/0.6 preview tags.
- Fixed first-run `Später` semantics: setup remains incomplete, progress is preserved and the assistant is offered again after a 24-hour snooze.
- D6.2 FFB/UI work remains the functional baseline; no engine downgrade occurred.

> Entries below are historical internal preview milestones and do not supersede the current frozen user-facing version.

# 0.6.2-alpha — D6.2 FFB Verification / Control-Panel Rework

- Audited every visible D6 FFB setting against persistence, runtime, hardware-test scope and adapter capabilities.
- Added live retuning of the running Game FFB source for Wheel-profile gains and D4 shaping without restarting the game output session.
- Fixed D4 0% Constant/Transient gain semantics and added schema-v1 compatibility migration.
- Added the missing Game Constant Gain control and disabled misleading Transient Gain until an adapter publishes that channel.
- Split Force Feedback into Basis / Effekte / Signalformung / Live & Test, with explicit scope badges and active Game Profile gain visibility.
- Moved manual Constant Force to the canonical wheelengine/OpenG27-lg4ff packet.
- Fixed Autocenter live test to send SpringSet + SpringEnable and neutralize through SpringOff/global stop.
- Live monitor now shows Game telemetry or direct Hardware-Test requested/applied values as appropriate.
- Reduced live UI flicker with dirty-region invalidation, rcPaint-clipped double buffering and cached/throttled accessibility overlays.
- Added D6.2 software regression coverage and a post-D6.2 master audit/roadmap.

# 0.6.1-alpha — D6.1 HID Sharing Fix & Control Panel UX

- Fixes the G27 FFB live-test sharing violation seen when the HID interface is already open.
- Native HID output now retries a sharing violation once with OpenG27/HidSharp-compatible `FILE_SHARE_READ | FILE_SHARE_WRITE` semantics.
- Known external wheel writers (OpenG27/OpenG27FFB/LCore) are surfaced in the FFB panel instead of producing a generic Windows error.
- Enabling Native Output automatically probes the exact selected HID writer without sending a motor report.
- Every bounded motor test performs the same non-motor preflight and fails closed before FFB if the writer is unavailable.
- The FFB header shows writer share mode, backend, shared fallback count and detected conflicts.
- Existing OutputLease, watchdog, Emergency Neutralize and hard NativeFFBConfig caps remain authoritative.

# 0.6.1-alpha — D6 Wheel Control Panel / Force Feedback UI

- Added `Live-Test` and `Force Feedback` subtabs directly inside the existing Lenkrad page; no new main navigation page or separate settings dialog.
- Added visual, draggable sliders for overall gain, Constant Force, Spring, Damper, Friction and 270/360/540/720/900° operating range.
- Added D4 sliders for Transient Gain, Minimum Force, Deadband, Response Curve, Low-Pass, Smoothing and Pre-Safety Output Limit plus a pipeline on/off control.
- Added Gentle/Balanced/Direct Wheel presets and Neutral/Smooth/Responsive/Compensated D4 shaping presets.
- Added a live Raw → Shaped → Applied FFB monitor with clipping, effect, HID backend and latency state.
- Added bounded Constant/Spring/Damper/Friction/Autocenter test controls and an Emergency Stop control; all reuse the existing central OutputLease/watchdog/recovery implementation.
- Manual edits persist per physical wheel as the existing `Custom` Native Engine / D4 configuration. Rotation is only written immediately when Native Wheel Output has already been explicitly enabled.
- Added keyboard slider adjustment and native accessibility peers for all D6 controls.
- Kept hard NativeFFBConfig motor caps downstream of all D6 tuning and left physical Stable-certification flags unchanged.

# 0.5.10-alpha — D5.10 Release Trust / Authenticode Hardening

- Added runtime Authenticode trust visibility for the exact running LogiMate executable, including status, publisher subject and certificate thumbprint.
- Added a dedicated D5.10 Release Trust view plus Engine Health and diagnostic-report integration.
- Stable builds now require the certification manifest release to exactly match VERSION and all global/per-model certification gates to be complete before signing starts.
- Added explicit Windows SignTool integration using an externally provisioned `LOGIMATE_SIGN_THUMBPRINT` and `LOGIMATE_TIMESTAMP_URL`; no private key or signing secret is stored in the source/package.
- Stable `LogiMate.exe` is signed and verified before it is copied into the installer payload.
- Final `LogiMate-Setup-x64.exe` is then signed and verified, and both artifacts must match the configured publisher certificate thumbprint.
- Added `RELEASE_ATTESTATION.json` with final app/installer SHA-256 hashes, Authenticode identity and certification state.
- Portable packaging now consumes the final application payload after the signing stage.
- Pre-release builds may remain unsigned for development, but are explicitly incapable of satisfying the Stable signing gate.

# 0.5.9-alpha — D5.9 HID Stress / Adverse-I/O Hardening

- Added process-lifetime native HID transport metrics for handle opens, open failures, writes, write failures, deadlines/timeouts, partial writes, `CancelIoEx`, poison events, compatibility fallback, closes and write latency.
- Hardened ambiguous output completion: partial writes and failed `GetOverlappedResult` confirmation now poison/close the HID transport fail-closed and the handle cannot be reused.
- Preserved the strict compatibility rule: only synchronous ERROR_INVALID_FUNCTION / ERROR_NOT_SUPPORTED / ERROR_INVALID_PARAMETER may fall back from OVERLAPPED WriteFile to bounded `HidD_SetOutputReport`.
- Added deterministic D5.9 software policy stress to Engine Health plus a 10,000-iteration user-triggered stress test.
- Added a guided 100-cycle real-wheel HID reopen/write/input stress path using only the already-active range as an idempotent non-motor report.
- Added a guided USB-yank adverse-I/O window that expects a real HID failure/disconnect and verifies central output-lease release.
- Added Markdown + JSON HID-stress evidence export under `LogiMateData\Certification`.
- Added an explicit HID-stress condition to the runtime Stable gate.
- Kept `hidStressValidated=false`: synthetic/software PASS is not physical G25/G27/DFGT certification.

# 0.5.8-alpha — D5.8 Accessibility / UI Automation Foundation

- Added a native Windows accessibility bridge over the custom-painted main window. Transparent standard child BUTTON/CheckBox/RadioButton HWNDs provide Windows/MSAA/UI Automation peers while LogiMate keeps its existing buffered visual renderer.
- Exposed semantic names, roles, enabled state, focus and checked/selected state for navigation, page actions, theme selection, Settings toggles and the Wheel `Erweiterte Ansicht` control.
- Routed native accessibility invokes through the same existing LogiMate action functions as mouse and keyboard input; D5.8 does not introduce a second state-changing UI path.
- Added a transparent native status surface containing the current page heading/subtitle/live body so accessibility clients have textual page context in addition to interactive controls.
- Added keyboard reachability for the Wheel Advanced View and Settings auto-scroll when keyboard/UIA focus moves to an off-screen setting.
- Added D5.8 UI Automation bridge status to Diagnostics and Engine Health.
- Added Windows regression tests for focus/control-ID separation and Advanced View focus-order coverage.
- Kept `uiAccessibilityValidated=false`: Windows Narrator/UIA, High Contrast, reduced-motion, mixed-DPI and custom-dialog validation remain explicit release evidence instead of being inferred from compilation.

# 0.5.7-alpha — D5.7 Hardware Certification Assistant

- Added an evidence-driven Hardware Certification Assistant for real G25/G27/DFGT validation. It is integrated into the existing `Kalibrieren & Lernen` workflow and does not create a competing wheel runtime.
- Persists per-physical-wheel certification evidence under `LogiMateData\Certification\hardware-evidence.json`; PASS requires non-empty evidence and never changes the repository Stable gate automatically.
- Added automatic native-PID/PnP/model verification plus guided physical tests for steering, pedals, buttons, H-shifter, range, Constant Force, Spring, Damper, Friction and Autocenter.
- Added guided evidence steps for Game FFB, reconnect, USB removal, process-kill recovery, suspend/resume and Modern↔Legacy↔Modern rollback.
- Added Markdown + JSON evidence export for later Stable review.
- Added live certification progress to `Lenkrad → Erweiterte Ansicht`, keeping raw-data, engine and physical-evidence truth on one page.
- Added D5.7 regression coverage for persistence, empty-evidence rejection, model-specific requirements and native-PID auto evidence.
- D5.6 advanced wheel view was accepted by the user on the real G27; physical motor/recovery/migration certification remains pending until the assistant is run.

# 0.5.6-alpha — D5.6 Advanced Wheel View

- Extended the existing Wheel page instead of adding a separate diagnostics page. A new `Erweiterte Ansicht` toggle keeps the normal live dashboard intact and expands it in-place with a complete live raw-data surface.
- Added OpenG27-style low-level visibility for the selected wheel: stable/session identity, PnP/model-confirmation state, instance/interface/alias IDs, driver/INF evidence, hardware fingerprint and detection evidence.
- Added full live input telemetry: source/layout, sample validity/generation/timestamp, report rate/age, reconnects, current/last errors, axes/button counts and connected controller paths.
- Added six-axis raw tables with raw/min/max/normalized values plus steering/pedal semantic calibration.
- Added a complete 32-bit button matrix, POV/D-pad state, paddles, wheel buttons, shifter buttons and current gear.
- Added byte-for-byte Direct-HID report inspection with index, hex, decimal and binary representation for every byte.
- Added background-captured HID interface metadata (VID/PID, product/serial, report lengths, usage page/usage, version and full device path) so WM_PAINT never probes hardware.
- Added Native Engine visibility in the same view: activation phase, output status, FFB state/counters, output lease ownership and calibration/detection errors.
- Renamed the old raw-data toggle in `Kalibrieren & Lernen` to the same `Erweiterte Ansicht` state so both entry points control one in-page view.

# 0.5.5-alpha — D5.5 Automatic HID Evidence Fusion Fix

- Fixed the remaining automatic G27 confirmation regression: when the all-class SetupAPI pass and the direct HID-interface pass described the same Windows devnode, LogiMate previously dropped the HID-side `HIDProduct`/serial evidence as a duplicate. The Direct-HID diagnostics could therefore show `G27 Racing Wheel` while the automatic C294 model gate still saw only a generic device and waited for a manual confirmation.
- `mergeDeviceEvidence` now enriches duplicate PnP records field-by-field with stronger HID metadata instead of discarding it.
- A direct HID product string such as `G27 Racing Wheel` now reaches the existing current-session consensus automatically, sets `ModelConfirmed=true`, and allows the already validated D5.4 C294→C29B handshake to queue without a manual click.
- Added D5.5 regression coverage for duplicate devnode enrichment and the complete fused-snapshot → session-confirmation → auto-native-preparation path.

# 0.5.4-alpha — D5.4 OpenG27-Compatible G27 Handshake

- Matched OpenG27/HidSharp's Windows G27 compatibility handshake more closely: the short-lived C294 writer now opens read+write with shared read+write access instead of LogiMate's stricter normal-output sharing policy.
- Extended the G27 native-switch write deadline from the normal 350 ms output deadline to 3 seconds, matching HidSharp's default write window for the OpenG27 connection path.
- Kept the byte-exact 8-byte report `00 F8 09 04 01 00 00 00` and direct HID candidate ranking from D5.2/D5.3.
- A stale saved `legacy` preference no longer strands a wheel that Windows currently proves is actually bound as Generic HID; only a live Legacy driver binding suppresses automatic native activation.
- Added an explicit `G27 jetzt nativ verbinden (C294 → C29B)` rescue action in Devices, equivalent to OpenG27's Connect action, with exact phase/error reporting.
- Added activation phases (`compat-found`, `sending`, `waiting`, `native`, `failed`) to Direct HID diagnostics and Engine Health.
- Extended C29B wait time to 10 seconds after the switch to tolerate slower Windows USB/PnP convergence.
- Added D5.4 regressions for stale Legacy preference handling and the multi-second OpenG27-style switch deadline.

# 0.5.3-alpha — D5.3 G27 Native Activation Recovery

- Fixed the remaining first-run G27 activation gap: a single confirmed C294 G27 now auto-promotes to native mode unless the user explicitly selected Legacy mode. A missing `wheel.mode` no longer blocks the OpenG27-like native transition.
- Matched OpenG27's proven G27 switch sequence exactly: G27 C294→C29B now sends only `F8 09 04 01 00 00 00` (plus the HID report-ID byte) before waiting for re-enumeration.
- Kept the historical two-step Logitech prelude isolated to G25/DFGT until physical certification confirms whether either model needs it.
- Added regression tests for first-run auto-promotion, explicit Legacy opt-out and byte-exact OpenG27-compatible G27 native switching.
- Preserved D5.2 HID-first detection, candidate capability ranking, coherent snapshots, direct HID diagnostics and the D5 standalone runtime boundary.

# 0.5.2-alpha — D5.2 OpenG27-style Detection

- Reworked wheel discovery around a HID-first coherent snapshot instead of combining independent parallel SetupAPI/Raw-Input scans that could straddle a C294/native re-enumeration.
- Added OpenG27-style HID candidate ranking: largest output report first, then richer input report and joystick/gamepad usage.
- Added direct HID attribute/capability probing (VID/PID, product string, serial, input/output/feature report lengths).
- Added re-enumeration settling when C294 and a native PID overlap for the same stable wheel or when HID visibility leads the PnP device tree.
- Native-mode switching now waits/reopens using direct HID discovery first, with Raw Input only as supplemental evidence.
- Added session-only C294 G25/G27 confirmation from an explicit direct HID product string. Generic compatibility names still do not authorize model-specific writes.
- Added conservative native-identity memory: a previously observed C299/C29A/C29B identity can help restore C294 only when the Windows device session, strong hardware fingerprint or current model evidence still matches. USB port history alone never authorizes a model.
- Added direct HID serial data to strong hardware fingerprinting when the wheel exposes one.
- Added a Direct HID diagnostics view in Devices so users can see exactly which supported Logitech HID interfaces, report capabilities and product names LogiMate sees.
- Added D5.2 regression tests for HID ranking, re-enumeration settling and C294 HID-product consensus.
- Preserved the D5 standalone runtime boundary: no production dependency on `internal/openg27port`.

# 0.5.1-alpha — D5.1 Detection Recovery

- Fixed a critical PnP recognition regression: real Windows `USB\...` / `HID\...` instance IDs were compared against two-literal-backslash prefixes and could be marked unverified.
- Added a second SetupAPI HID-interface discovery pass, similar in spirit to HidSharp/OpenG27 device enumeration, independent of Raw Input activity.
- Fused SetupAPI devnodes, SetupAPI HID interfaces and Raw Input paths for more resilient Logitech wheel discovery.
- Grouped USB wheel nodes and HID child collections by their shared USB topology token before ContainerID fallback.
- Added session-only G25/G27 C294 model consensus when independent PnP and explicit WinMM evidence agree; no unsafe topology-only persistence was reintroduced.
- Kept DFGT C294 conservative until C29A or explicit user confirmation.
- Restored guarded C294→native switching immediately after safe discovery instead of waiting for the live-input page.
- Fixed G27 Direct-HID handle sharing so LogiMate input and its own serialized output transport can coexist.
- Added D5.1 Windows regression tests for real instance IDs, session consensus, conservative DFGT handling, USB/HID grouping and discovery-path deduplication.

# 0.5.0-alpha — Native D5 Modern Independence

- Removed OpenG27 executable discovery, download/install, launch and process-state ownership from the normal LogiMate runtime/UI.
- Removed the production dependency from `cmd/logimate` to `internal/openg27port`; the package remains source/test provenance and parity material only.
- Replaced the last native G27 input defaults named after OpenG27 with LogiMate-owned deterministic native input defaults; external `%APPDATA%\OpenG27\config.json` can no longer alter live input behavior.
- Kept old OpenG27 game-profile files as an explicit offline Legacy import format, parsed by LogiMate-owned compatibility code.
- Kept the historical `openg27-pino` adapter ID as a read-compatible migration alias only.
- Native output no longer polls for a named OpenG27 process; exclusive LogiMate output leases and exclusive HID writer handles are the ownership authority.
- Added a build gate that fails if the production executable dependency graph ever reintroduces `internal/openg27port`.
- Updated Engine Health, diagnostics and System UI to describe the D5 standalone Native Engine rather than an external fallback.
- Logitech LGS/Profiler remains a deliberate first-class Legacy-mode alternative, never a Modern dependency.
- Physical G25/G27/DFGT certification remains external and still blocks Stable.

# 0.4.0-alpha — Native D4 Advanced FFB

- Added a platform-independent Advanced FFB pipeline with deadband compensation, minimum force, response curves, low-pass filtering, smoothing and separate constant/transient semantic gains.
- Kept all D4 shaping before the hard per-wheel NativeFFB safety mixer so presets can never raise the final motor ceiling.
- Added per-wheel `advanced-ffb.json` schema with future-version write protection and code-safe Neutral/Smooth/Responsive/Compensated/Disabled presets.
- Added fully individual Constant/Spring/Damper/Friction tuning to custom Wheel Engine profiles.
- Added raw/shaped/applied force diagnostics, separate pipeline-vs-safety clipping counters, deadband/minimum-force counters and average/max sample latency.
- Promoted the preferred HID backend to true OVERLAPPED `WriteFile` with exact-operation `CancelIoEx` cancellation and fail-closed poisoned handles.
- Retained a bounded `HidD_SetOutputReport` compatibility backend only for synchronous pre-queue `WriteFile` rejection on classic HID stacks.
- Added D4 regression tests proving advanced shaping cannot bypass the hard force ceiling.
- Physical G25/G27/DFGT transport/feel certification remains intentionally external and blocks Stable.

# 0.3.3-alpha — Native D3 Adapter Ecosystem

- Added `internal/gameadapter` with a typed semantic adapter contract, registry, capabilities, lifecycle, health and normalized frames.
- Migrated Wreckfest 2 / Pino and LogiMate Local JSON onto the same adapter lifecycle.
- Removed Pino-specific assumptions from productive Native Game Output; output now checks adapter capabilities and matching adapter identity.
- Added packet/frame rate, last-frame age and setup guidance to telemetry diagnostics.
- Added architecture tests that prevent adapters from containing direct HID/output primitives.
- Added game-profile schema v2 with adapter port/priority fields and future-schema write protection.
- Preserved `openg27-pino` as a read-compatible migration alias only.
- Kept all wheel output behind the D0/D2 central output lease, recovery marker and native HID transport.
- Physical G25/G27/DFGT Stable certification remains external and intentionally incomplete.

# 0.3.2-alpha — Native D2 Unified Engine

- Moved productive classic Logitech report builders, FFB scheduler and Wreckfest/Pino telemetry parser from the historical OpenG27-port runtime boundary into `internal/wheelengine`.
- Productive Modern input/output/game-FFB paths no longer import `internal/openg27port`; that package remains an attributed parity/reference/import oracle only.
- Added one capability-driven Native Game Output path for G25/G27/DFGT and removed the duplicate historical C3 live hardware test from normal UI.
- Preserved old `openg27-pino` profile IDs as a read-compatible alias while normalizing runtime state to `wreckfest2-pino`.
- Added parity tests that compare the LogiMate-native protocol/scheduler/Pino implementations against the attributed reference implementation without making it a runtime dependency.

## 0.3.1-alpha — Native D1 Model Refinement

- Added explicit G25/G27/DFGT model adapters for native PID/selector, layout IDs, report lengths and hardware capabilities.
- Moved model-specific native report decoding into platform-independent `wheelengine` code with G25/G27/DFGT fixture tests.
- Fixed G27 Direct-HID state so valid reports remain tagged `direct-hid` / `g27-c29b-v1` rather than falling back to WinMM metadata.
- Added evidence-based per-model hardware certification requirements; software compilation can never mark a wheel physically validated.
- Extended Stable packaging gates to require explicit G25, G27 and DFGT hardware evidence.

# 0.3.0-alpha — Native D0 Foundation

- Added platform-independent `internal/wheelengine` with typed model IDs, capabilities and canonical device/input/output/health state.
- Unified G25, G27 and Driving Force GT behind one model descriptor registry and one journaled Modern migration path.
- Routed production HID output through one serialized Native transport/lease boundary with deadline, cancellation attempt and fail-closed poisoned-handle behavior.
- Projected Direct-HID/WinMM input into one per-wheel/per-session state; reconnect invalidates stale samples.
- Generalized Wreckfest 2 game FFB to Native-FFB-capable G25/G27/DFGT while keeping G27 Rev LEDs capability-gated.
- Renamed the normal telemetry adapter to `wreckfest2-pino`; saved `openg27-pino` profiles migrate transparently.
- Updated the normal UI and setup flow: Modern means LogiMate Native for all three integrated models and requires no external wheel runtime.
- Kept OpenG27-derived code/imports attributed and backward compatible, but external OpenG27 is no longer part of the normal System workflow.
- Added D0 core self-test and updated architecture/support/standalone hand-off documents.
- Stable remains blocked until real G25/G27/DFGT hardware, crash/recovery, migration, HID-stress, UI/accessibility and Authenticode gates are complete.

# Changelog

## 0.2.8-alpha — OpenG27 Fusion C7 Native Cutover

- Promoted LogiMate Native to the normal G27 Modern runtime; external OpenG27 is now optional fallback only.
- Modern G27 migration runs the C1–C6 hardware-free parity gate before destructive driver work and no longer downloads or launches OpenG27 automatically.
- Added live C7 PASS/WARN/BLOCK readiness covering selected/PnP-verified G27, Generic HID, single output ownership, C4 live parser parity, Direct HID and C3 scheduler dry-run.
- Kept physical C3/C6 FFB/LED/reconnect validation as an explicit WARN rather than claiming stable hardware parity.
- Added C2 legacy-builder shadow warning: known old compatibility/test builder differences remain visible while C3/C6 use the ported OpenG27/lg4ff bytes.
- Added an explicit System-page OpenG27 Fallback manager for status, optional install/update and A/B launch.
- Starting the fallback keeps the existing safety handoff and stops LogiMate native output first.
- Removed the obsolete user-facing “auto-start OpenG27” setting; the persisted field remains read-compatible but defaults false.

## 0.2.7-alpha — OpenG27 Fusion C5 + C6

- C5: OpenG27 GameProfile matching, profile JSON import, Wreckfest preset translation and per-StableWheelID engine-profile creation.
- C5: imported `ProcessMatch` preserves OpenG27 case-insensitive substring matching without changing native LogiMate exact-EXE profiles.
- C5: profile import never changes the independent hard FFB safety profile and never auto-starts motor output.
- C6: Wreckfest 2 Pino Main-packet decoder and loopback-only UDP listener on 127.0.0.1:23123.
- C6: Pino frames feed the common LogiMate telemetry runtime, stale detection, ring buffer, Engine Health and diagnostics.
- C6: explicit G27-only Game-FFB + RPM-LED hardware session using the ported scheduler under LogiMate safety caps, slew limiter, owner generation, OpenG27 interlock and crash recovery.
- C6: Physics/PlayerControl gate force to zero; telemetry loss also requests immediate neutral instead of holding stale force.
- C6: OpenG27 Wreckfest preset is available even when OpenG27 is not installed.

## 0.2.5-alpha — OpenG27 Fusion C4 + Live Engine Health

- Ported OpenG27 `G27ReportParser` into the isolated Go parity core with upstream golden vectors.
- Added live G27 parser shadow counters (MATCH/DIFF/parse errors) without changing the production input path.
- Added OpenG27 `G27Device` lifecycle parity planning while retaining LogiMate StableWheelID/PnP authority.
- Fixed Engine Health: it now refreshes every 500 ms instead of showing a one-time snapshot.
- Live Engine Health now exposes Native FFB heartbeat/state, C3 scheduler counters, telemetry, C4 parser parity and state invariants.
- Added C4 self-test and diagnostic-report sections.

## 0.2.4-alpha — OpenG27 Fusion C3 Scheduler

- Ported OpenG27 `IFfbSource`, `FfbEngine`, `WheelOutput` and `WheelOutputOptions` semantics into `internal/openg27port`.
- Added the parity 6 ms scheduler loop, deterministic `PumpOnce`, repeated output tick, source watchdog, slew limiter and guaranteed Panic/center handoff.
- Added a G27-only C3 live scheduler test capped to ±5% with a hard 900 ms session timeout.
- C3 shares LogiMate's generation owner, crash marker, Emergency Stop, device-change/shutdown cancellation and OpenG27 process interlock.
- Existing LogiMate Constant/Spring/Damper/Friction paths remain available and are not silently replaced.
- Added C3 scheduler telemetry to Engine Health, FFB Engine Status and diagnostic exports.
- Added C3 provenance ledger entries, deterministic dry-run and scheduler regression tests.

## 0.2.3-alpha — UI & Dialog Hardening

- Fixed the inverted `TaskDialogIndirect.Find()` availability checks that caused many selection windows to be skipped on normal Windows 11 systems.
- Replaced app-level MessageBox/TaskDialog surfaces with a LogiMate-owned Acrylic/Mica dialog window matching the “Was ist neu?” design language.
- Converted wheel management and all input/native-engine command menus to the new modern dialog surface.
- Added live Modern/Legacy migration progress with current phase, recent durable journal steps and an expandable command/detail view.
- Rebuilt the standalone installer as a modern progress window with expandable command/log view while preserving safe `.previous` rollback semantics.
- Added regression tests for dialog choice parsing and migration-progress rendering helpers.
- OpenG27 Fusion C2 behavior remains unchanged; Fusion C3 moves to 0.2.4-alpha.

## 0.2.2-alpha — OpenG27 Fusion C2

- Ported OpenG27 `Lg4ffReports` into the isolated `internal/openg27port` reference package.
- Added translated OpenG27 golden tests for Constant Force, global stop, rotation range, G27 native switch, autocenter, damper, friction and rev LEDs.
- Added C2 Protocol Shadow comparison to Engine Health and diagnostic reports.
- Confirmed existing LogiMate parity for range, G27 switch, SpringSet, Damper/Friction slot layout and condition stop.
- Identified intentional pre-promotion differences in the old G27 LED packet, Constant Force packet and missing OpenG27 SpringEnable sequence.
- Live HID/motor routing is unchanged in C2; reference bytes remain shadow-only until C3.

## 0.2.1-alpha — OpenG27 Fusion C1

- Started the controlled C → D OpenG27 fusion roadmap.
- Added isolated `internal/openg27port` pure Core port with parity tests.
- Ported ForceScaling, ForceFrame/ForceMixer, AxisCalibration, PedalCalibration and RangeTracker behavior.
- Added full MIT attribution and per-file provenance ledger.
- Added in-app self-test checks for C1 parity.
- No new hardware output path is enabled in this release.

## 0.2.0-alpha — Audit 71–100: Game Profiles, Telemetry & Release Hardening

- **Audit 100/100:** the full code-audit sequence is now documented and implemented; stable 1.0 remains gated by physical hardware validation, signing and complete UI Automation.
- **Game profiles:** validated per-game executable bindings, native process/foreground detection, safe profile/range Auto-Apply and JSON import/export. Auto-Apply never starts motor FFB.
- **Telemetry framework:** adapter-neutral telemetry frame plus loopback-only JSON adapter, stale detection, bounded history, RPM LED mapping and physics/player-control force gate.
- **Reliability:** state invariant validator and hardware-independent protocol/self-test suite.
- **Snapshots:** whitelisted configuration ZIP snapshots with SHA-256 manifests and validated restore.
- **Crash recovery:** native FFB tests write an output marker and clean it only after safe stop; startup can issue an emergency zero-force recovery after an unclean session.
- **Versioned data schema:** explicit schema version and backup-first future migration model.
- **Release readiness:** diagnostic score separates code health from external certification gates.
- **OpenG27 fusion plan:** new architecture audit for a native LogiMate engine derived from/validated against MIT-licensed OpenG27 Core while preserving LogiMate UI and extending G25/DFGT support.

## 0.1.7-alpha — Audit 61–70: Native Effect Manager & Wheel-Engine Profiles

- **Dedicated effect slots:** Constant Force uses slot 0, Spring slot 1, Damper slot 2 and Friction slot 3. Start/update/stop opcodes are built per slot rather than sharing one condition slot.
- **Spring hardware test:** short watchdog-bounded test path through the same strict selected-wheel/PnP/Generic-HID/OpenG27 interlock as every other native output.
- **Damper hardware test:** dedicated slot-2 bounded test with profile + safety gain stages.
- **Friction hardware test:** dedicated slot-3 bounded test with profile + safety gain stages.
- **Force mixer:** requested Constant/Spring/Damper/Friction values pass through profile effect gain, profile master, safety master and hard per-effect ceilings before packet construction.
- **Clip telemetry:** every hard mixer clamp is counted and shown in diagnostics so over-range sources are observable rather than silently flattened.
- **Wheel-Engine profiles:** Gentle, Balanced and Direct built-ins plus a per-wheel Custom profile store rotation and effect-character gains separately from the hard FFB safety profile.
- **Transactional profile activation:** profile state is persisted only after its rotation command succeeds; applying a profile never starts motor force by itself.
- **Generation-safe ownership:** every FFB worker receives a generation/owner ID. Stale workers cannot overwrite a newer session after cancellation/effect handoff.
- **Unified stop/self-test:** Emergency Stop clears all four FFB slots; Engine Dry Run validates mixer/slot/stop packet paths without motor movement. One-shot range writes no longer remain falsely marked active.
- **Diagnostics:** active profile, generation, Constant/Spring/Damper/Friction applied values, clip count and effect-transition count are exported.

## 0.1.6-alpha — Audit 51–60: Native FFB Engine Foundation

- **Per-wheel FFB safety profile:** `native-wheel-engine.json` stores master gain, constant-force test cap, future condition-effect gains, slew limit and watchdog per stable physical wheel ID using atomic writes.
- **Constant-force protocol:** bounded classic Logitech slot-0 start/update/stop report builders are implemented and regression-tested. The public test API refuses values outside ±10%.
- **Serialized worker:** a single short-lived native FFB worker owns one HID handle for a testsession and updates at 20 ms instead of reopening the device for every frame.
- **Hard force limits:** master gain can only reduce the requested test; configuration normalization can never raise the experimental constant-force ceiling above ±10%.
- **Slew limiter:** force changes are ramped in configurable 1–5 percentage-point steps per 20 ms tick so a test cannot jump instantly from zero to its target.
- **Heartbeat/watchdog:** every active FFB session publishes a heartbeat and self-stops after a clamped 500–2000 ms watchdog window. Presets expose 900/1200/1500 ms.
- **Condition-effect protocol basis:** pure builders for spring (`0x0b`), damper (`0x0c`) and friction (`0x0e`) follow the lg4ff slot encoding and are compile-tested, but remain hidden from persistent/game output until real hardware validation.
- **Output ownership:** the existing single-wheel/model/PnP/Generic-HID gate still applies. OpenG27 is checked before starting output and periodically while the bounded test worker is alive.
- **Failsafe handoff:** active force is stopped on app shutdown, Experimental-output disable, OpenG27 handoff, wheel target change, detection reset and any `WM_DEVICECHANGE` before identity is rebuilt.
- **FFB diagnostics:** requested/applied force, active effect, frames, slew-limit count, heartbeat, watchdog stops, emergency stops, config and last error are included in the UI status and diagnostic report.
- **Regression tests:** force encoding, effect-stop opcodes, condition builder limits, config clamps, slew behavior and heartbeat semantics are compile-tested for the Windows target.


## 0.1.5-alpha — Audit 41–50: Safe Native Output Foundation

- **Native output transport:** model-aware Set_Output_Report path for the selected native G25/C299, DFGT/C29A or G27/C29B target. UI code never writes raw HID packets directly.
- **Explicit opt-in:** `Native Wheel Output (Experimental)` is OFF by default and must be enabled intentionally in Settings.
- **Strict output gate:** exactly one physical supported wheel, explicit selection, `ModelConfirmed`, `PnPVerified`, native PID and Generic-HID mode are required.
- **OpenG27 interlock:** output refuses to start while OpenG27 is running. Starting OpenG27 from LogiMate first stops any active native-output test.
- **Rotation range:** 40–900° protocol support with UI presets 270/360/540/720/900° for G25/DFGT/G27.
- **G27 LEDs:** bounded 5-bit rev-LED bar test (0–5 LEDs) plus explicit all-off command.
- **Autocenter test:** experimental FE/0D command is restricted to 0–30% and ramp 0–7; UI exposes only 10/20/30% test levels.
- **Watchdog:** any non-zero autocenter test is automatically zeroed after two seconds.
- **Emergency stop:** manual stop, application shutdown and OpenG27 handoff return autocenter to zero; G27 LEDs are also cleared.
- **Output diagnostics:** last command/error/write time, command count, emergency stops and watchdog stops are exposed in diagnostics and the Native Output status dialog.
- **Protocol regression tests:** range, LED-mask and autocenter bounds are compile-tested for the Windows target.


## 0.1.4-alpha — Audit 31–40: Direct HID, Learning & Calibration

- **G25 / DFGT Direct HID foundation:** confirmed native C299/C29A targets try a conservative read-only HID reader before WinMM. The G27 keeps its dedicated parser. G25/DFGT remain hardware-validation gated.
- **Raw-data mode:** the Wheel dashboard can show the live Direct-HID report, rate, report age, reconnect counter and latest HID/WinMM error without sending output reports or FFB.
- **Button learning:** map one physical button to Paddle L/R, Wheel 1–6 or Shifter 1–8. Simultaneous new presses are rejected instead of guessed.
- **H-shifter learning:** G25/G27 can learn Neutral, gears 1–6 and Reverse. Duplicate raw signatures stop the wizard and surface a likely switch/decoder conflict.
- **Steering calibration:** capture full left, center and full right against a selectable 270/360/540/720/900-degree range.
- **Pedal calibration:** capture rest/full travel, infer inversion, select deadzone and response curve, and persist the result per physical wheel, model and operating mode. DFGT correctly omits a clutch step.
- **Input telemetry:** Direct HID reports actual reader frequency; WinMM exposes LogiMate polling frequency. Reconnect and historical error counters survive transient recovery.
- **Raw report snapshot:** copy the current HID payload as a byte table and hex string for hardware/revision comparisons.
- **Safety:** all learning/raw-data tools are read-only. C294 remains usable for generic diagnostics, but model-specific H-shifter semantics and native writes still require confirmed identity.
- **Regression tests:** classic HID parsing, single-button learning, steering direction, pedal clamp/inversion/deadzone/curve and profile persistence are compile-tested for the Windows target.

## 0.1.3-alpha — Wheel Identity & Setup State Machine Hardening

- **Stable physical wheel identity:** persisted selection now prefers `DEVPKEY_Device_LocationPaths` (`usbloc:*`) and falls back to the USB instance suffix (`usbslot:*`). Windows ContainerId remains a current-session grouping key only, because it may change during C294↔C299/C29A/C29B re-enumeration.
- **0.1.2 repair:** a stale `container:*` selection is migrated automatically only when exactly one current wheel has an authoritative native PID and the old model hint matches that model. The reported C294→C29B G27 case therefore recovers without silently jumping to another wheel.
- **No global C294 guessing:** the old machine-wide `wheel.model` fallback is retired. Manual G25/G27/DFGT confirmation is valid only for the selected stable physical wheel; a newly attached C294 never inherits another wheel's identity.
- **Model/action separation:** ambiguous C294 remains readable for generic diagnostics but cannot authorize native-mode writes or driver changes. Explicitly recognized unsupported C294 models fail closed.
- **Explicit confirmation flags:** logical targets now track `ModelConfirmed` and `PnPVerified` separately. A descriptive C294 friendly name can improve UI labelling but can never by itself authorize model-specific or destructive actions.
- **Integrated native-mode handling:** confirmed G25, Driving Force GT and G27 use their model-specific C294→C299/C29A/C29B selectors. G27 retains Direct-HID input; G25/DFGT use the conservative WinMM live-input path after native transition.
- **Robust HID switching:** when one wheel exposes multiple C294 Raw-Input collections, LogiMate tries the available interfaces until one accepts the complete Set_Report sequence instead of trusting the first collection blindly.
- **Single migration path:** obsolete direct `switch-open*` / `switch-legacy*` administrator actions are disabled. Every mode transition goes through the tracked setup transaction with target validation, backup, journal and rollback rules.
- **Legacy/Modern UX consistency:** the guided System and first-run paths use the same rules. A read-only driver-inventory error no longer pretends the wheel is unusable; the inventory is rechecked strictly in the elevated process before any destructive change.
- **Fresh Legacy path:** when no backup exists, Legacy + Profiler can use Logitech's official LGS 5.10.127 installer only after a valid Logitech Authenticode signature is verified.
- **Immediate model confirmation state:** after a C294 model is safely persisted for one physical wheel, the wizard/main snapshot is updated immediately while the background rescan confirms it.
- **Diagnostics:** logical targets expose stable/session IDs; low-level nodes expose stable ID, container, location path, physical ID, parent and instance ID.

## 0.1.2-alpha — Windows Device Container Identity

- **Duplicate-wheel fix:** LogiMate now groups PnP nodes through Windows `DEVPKEY_Device_ContainerId`, the OS-level identity for one physical device.
- Corrected the `DEVPKEY_Device_Parent` property key that was incorrectly defined in 0.1.1 and could leave the HID child node separate from its USB parent.
- USB input and HID game-controller nodes from one driverless C294 wheel now resolve to one logical wheel even when their instance paths are unrelated.
- Two genuinely separate wheels remain separate because their Windows device-container GUIDs differ.
- The PowerShell fallback follows the same ContainerId-first model.
- Diagnostic exports now include ContainerId, physical identity and raw interface ID for each low-level wheel node.

## 0.1.1-alpha — Physical Wheel Identity & Fresh Legacy Setup

- USB and HID PnP nodes of one physical Logitech wheel are now collapsed into a single logical wheel using the Windows parent-device identity.
- A driverless G27 in C294 mode no longer appears twice merely because Windows exposes both `USB\VID_046D...` and `HID\VID_046D...` nodes.
- Multiple truly separate wheels remain separate and still require explicit selection.
- Existing selections from 0.0.7–0.1.0 are migrated from an interface ID to the physical-wheel ID automatically.
- C294 remains intentionally model-ambiguous until the user confirms G25, G27 or Driving Force GT; that confirmation is stored per physical device.
- Fresh Legacy setup can fetch Logitech Gaming Software 5.10.127 directly from Logitech when no local driver backup exists. The downloaded installer must have a valid Logitech Authenticode signature before LogiMate starts it.

## 0.1.0-alpha — Profile Hub UI Redesign

### Komplettes UI-System
- Die Hauptnavigation wurde auf eine stabile breite Profil-Sidebar umgestellt. Standardmäßig bleibt sie dauerhaft ausgeklappt; Einstellungen sitzt getrennt am unteren Rand wie in der neuen Profilvorlage.
- Startseite, System, Diagnose und Über mich verwenden keine gemeinsame Textfläche mehr, sondern eigene Dashboard-Kompositionen mit großen modernen Karten.
- Die Startseite priorisiert den nächsten sicheren Schritt und zeigt Wheel, Modus, HVCI und Backup als eigenständige Statuskarten.
- System bündelt Treiber/Backup, Betriebsmodus, Windows-Sicherheit und Wheel Engine in klar getrennten Bereichen.
- Diagnose zeigt Erkennung, Renderer und Windows-Umgebung als schnelle Statuskarten plus Support-Aktionen.
- Lenkrad behält die vollständigen Instrumente und das Mehrgeräte-Hinweispanel oberhalb der Live-Anzeigen.

### Profil & Projekte
- Die Über-mich-Seite ist jetzt als echte Profilseite aufgebaut: Profil-Hero, „Auf einen Blick“, „Über Markus“, „Aktuell“, Projekte, Lieblingsbereiche und Support.
- Die Profilansicht nutzt bewusst ein Initialen-Avatar statt ein erfundenes Porträt. Ein echtes Foto kann später als optionale Ressource ergänzt werden.
- Eine große „Indicana Projekte“-Kachel öffnet einen In-App-Projekt-Hub mit LogiMate, Indicana Tools, StromPilot, Green-ITea und TheLittleCyclist.
- LogiMate-/Indicana-Links und der PayPal-Support bleiben direkt erreichbar.

### Bedienung & Fehlerkorrekturen
- Die frühere Hover-Logik konnte die Sidebar beim Verlassen des Fensters wieder einklappen; die neue breite Navigation bleibt stabil.
- Die alte Sidebar-Einstellung ist nun eine dauerhafte Wahl zwischen breiter Profilnavigation und kompakter Icon-Leiste statt einer ständig animierenden Hover-Leiste.
- Das Indicana-Panel blockiert nur seinen eigenen Inhaltsbereich; die Hauptnavigation bleibt erreichbar, Escape schließt das Panel.
- Projektpanel-Scrolling wird nicht versehentlich auf die darunterliegende Profilseite durchgereicht.
- Das Hauptfenster startet größer und besitzt eine höhere Mindestgröße, damit die neue Kartenhierarchie nicht zusammengedrückt wird.

## 0.0.9-alpha — Acrylic Navigation & Persistent Wheel Dashboard

### Acrylic / Navigation
- Bei aktivem Windows-Systemmaterial wird die Sidebar nicht mehr als deckende Farbfläche über das Backdrop gemalt. Das dokumentierte DWM-Material bleibt im Navigationsbereich sichtbar; ausgewählte und gehoverte Navigationseinträge behalten kontrastreiche Karten.
- Safe UI, Hochkontrast und Systeme ohne aktiven Backdrop bleiben bewusst vollständig opak.

### Lenkradseite
- Der Hardware-Dashboard-Aufbau bleibt immer sichtbar. Kein Erkennungs- oder Auswahlzustand ersetzt mehr Lenkungs-, Achsen-, Tasten- oder Shifter-Instrumente durch eine Vollflächen-Leermeldung.
- Mehrere erkannte Wheels erzeugen oberhalb der Instrumente ein Warn-/Auswahlpanel. Ist ein Ziel gewählt, nennt das Panel das aktive Gerät; ohne Ziel fordert es zur bewussten Auswahl auf.
- Erkennungsfehler, kein Gerät und ein nicht mehr verbundenes gespeichertes Ziel nutzen denselben nicht-destruktiven Hinweisbereich, während die Instrumente darunter stabil sichtbar bleiben.

### Langfristige Wheel Engine
- Neuer Architekturplan `docs/OPENG27_INTEGRATION_PLAN.md`: OpenG27-Funktionen sollen schrittweise als native LogiMate-Komponenten übernommen werden.
- Ziel ist für den G27-Modern-Pfad langfristig Microsoft Generic HID + LogiMate ohne separate OpenG27-Anwendung. Externer OpenG27-Support bleibt bis zur hardware-validierten Funktionsparität als Fallback erhalten.

## 0.0.8-alpha — Ultimate UX & About

### Nutzerführung
- Die Übersicht besitzt jetzt eine kontextabhängige primäre Aktion: Erkennung, Geräteauswahl, Modellbestätigung, Treiber-Backup, OpenG27 oder Hardwaretest erscheinen genau dann, wenn sie der sinnvollste nächste Schritt sind.
- Der empfohlene nächste Schritt steht in der Übersicht an erster Stelle und wird nicht mehr unter technischen Statusblöcken versteckt.
- Der globale Status-Chip unterscheidet Bereit, Aktion nötig, fehlendes Backup und echte Erkennungs-/Treiberfehler.
- Backup- und HVCI-Farben sind kontextsensitiv statt pauschal gut/schlecht.
- Einstellungen, Diagnose und Über-mich-Seite verzichten auf das dort unnötige Wheel-Statusband und nutzen den zusätzlichen Platz für Inhalt.
- Die System-Seite erklärt direkt, warum ein Moduswechsel erlaubt oder gesperrt ist.

### Geräte verwalten
- Ein zentraler Dialog **Geräte verwalten** bündelt Lenkradauswahl, vollständige Neu-Erkennung und Reset der Erkennungsdaten.
- Eine gespeicherte, aber gerade nicht verbundene Auswahl wird ausdrücklich erklärt; LogiMate wechselt weiterhin niemals still auf ein anderes Rad.
- Pedal-Lernen ist deaktiviert, solange kein eindeutig ausgewähltes unterstütztes Ziel existiert.

### Über mich / Projekte / Support
- Neue native Seite **Über mich** mit Markus Kleine, LogiMate, Indicana Tools, StromPilot, Green-ITea, TheLittleCyclist und Open-Source-Credits.
- Neuer **Sharing is caring**-Support-Button öffnet PayPal; die PayPal-Adresse `indicana@tutanota.de` ist sichtbar und separat kopierbar.
- Direkte Links zu LogiMate und Indicana Tools auf GitHub sind integriert.

### Tests
- Neue UX-Regressionstests prüfen die kontextabhängige Hauptaktion, About-Inhalte, Seitengeometrie und verständliche Sperrgründe für Moduswechsel.

## 0.0.7-alpha — Multi-Device Selection & Audit 21–30

### Audit 21–30 / Mehrgeräte-Erkennung
- Expliziter Geräteauswahldialog listet alle erkannten Räder mit Modell, PID, Modus und verkürzter stabiler Geräte-ID.
- Eine gespeicherte Auswahl springt beim Abziehen niemals still auf ein anderes physisches Rad.
- „Erkennung zurücksetzen“ löscht nur Auswahl und manuelle C294-Identität; Treiber, OpenG27, Backups, Betriebsmodus und Pedal-Kalibrierungen bleiben erhalten.
- Geräte-Scanfehler werden von „kein Gerät angeschlossen“ getrennt und sperren sicherheitskritische Aktionen.
- C294-Modellbestätigungen werden pro physischem Gerät gespeichert.
- Pedal-Mappings werden pro physischem Gerät gespeichert, mit Rückwärtskompatibilität zu alten Modell/Modus-Mappings.
- Auswahlwechsel schließen Direct-HID-Handles und invalidieren Input-Caches, bevor das neue Ziel gelesen wird.
- Auswahl- und Erkennungsstatus erscheinen in Übersicht, Wheel-Seite, System-Seite und Diagnosebericht.
- Identische Räder werden in UI/Diagnose über PID plus Geräte-ID unterscheidbar.
- Neue Regressionstests decken Auswahl, Hotplug, Reset, per-device C294 und Pedal-Mappings ab.

## 0.0.6-alpha — Acrylic Recovery & Audit 11–20

### Hauptfenster / Acrylic
- Der Hauptrenderer verwendet jetzt einen **32-Bit-DIB-Backbuffer** statt eines Device-Dependent Bitmaps. Vor dem Present werden Alpha-Werte für den DWM-Glassheet-Pfad repariert; dadurch verschwinden GDI-Inhalte nicht mehr auf einer transparenten Acrylic-Fläche.
- DWM-/HWND-Materialänderungen laufen ausschließlich auf dem gelockten UI-Thread. Der frühere Hintergrund-Worker wurde entfernt.
- Schlägt `DWMWA_SYSTEMBACKDROP_TYPE` oder `DwmExtendFrameIntoClientArea` fehl, wird der Materialzustand vollständig zurückgesetzt, statt einen halben Glassheet-Zustand zu hinterlassen.
- Windows 11 24H2+ aktiviert für das Hauptfenster zusätzlich `DWMWA_REDIRECTIONBITMAP_ALPHA`, passend zum premultiplied 32-Bit-Backbuffer.
- Kann der Alpha-Backbuffer nicht sicher angelegt werden oder wäre er unvernünftig groß, fällt LogiMate automatisch auf eine sichtbare opake Oberfläche zurück.

### Audit 11–20 / Geräte- und Laufzeitrobustheit
- Modern/Generic-HID und Legacy setzen jetzt **genau ein angeschlossenes unterstütztes Lenkrad** voraus; mehrere Wheels können für Diagnosen ausgewählt werden, aber Treiberpaket-Wechsel bleiben wegen möglicher Paketüberschneidungen fail-closed.
- Der State modelliert mehrere physische Geräte als `[]WheelDevice` plus persistente `SelectedWheelID`; auf der Lenkradseite kann zwischen erkannten Zielen gewechselt werden.
- Bereits aktive Modi bleiben sichtbar, sind aber deaktiviert. Ebenso werden nicht sichere/nicht verfügbare Moduswechsel für Maus und Tastatur gesperrt.
- Der 7-Sekunden-Autorefresh ist jetzt ein leichter nativer Refresh. Driver-Store-/Profiler-Inventuren laufen nicht mehr periodisch schwergewichtig; Vollscans bleiben Startup, manuellen Refreshes, Geräteänderungen und kritischen Aktionen vorbehalten.
- `ProfilerSummary()` ist cache-only, damit kein Paint-/UI-Pfad PowerShell anstoßen kann.
- `WM_DEVICECHANGE` invalidiert Input-Caches und wird entprellt; ein USB-Ereignis geht auch während eines laufenden Scans nicht verloren.
- WinMM-Capabilities werden gecacht. Der 250-ms-Livetest pollt danach nur die ausgewählte Joystick-ID und baut das Mapping bei einem verschwundenen Slot einmal neu auf.
- Direct HID zeigt `CreateFile`-/Lesefehler sichtbar, verwendet begrenztes Reconnect-Backoff und unterscheidet Connected, Idle, Malformed, Read error, Open error und Reconnecting.
- Diagnoseberichte enthalten logische Wheel-Ziele, `SelectedWheelID`, Renderer/Windows-Build sowie den bevorzugten Live-Input-Status.


## 0.0.5-alpha — Reliability Foundation

### Sichere Modern-/Legacy-Migration
- Ersteinrichtungs-Migrationen werden jetzt als **Transaktion mit dauerhaftem Journal** verfolgt. Der Assistent gilt erst als abgeschlossen, wenn der erhöhte Prozess tatsächlich Erfolg zurückmeldet.
- Jeder kritische Schritt schreibt Status, Detail und Zeitstempel in `Migrations/<id>.json`; fehlgeschlagene Setups bleiben sichtbar und der Assistent offen.
- Modern-Wechsel versuchen bei späteren Pflichtfehlern automatisch, Treiber, Profiler-Zustand und vorherige Betriebspräferenz best effort zurückzurollen.
- Beim Legacy-Weg wird ein nur für den Wechsel deaktiviertes HVCI bei fehlgeschlagenem Restore best effort wieder aktiviert.
- Destruktive Moduswechsel werden blockiert, wenn kein eindeutig unterstütztes Ziel-Lenkrad angeschlossen ist oder die Legacy-Treiberinventur nicht zuverlässig gelesen werden kann.

### Backups & Integrität
- Profiler-Backups erhalten jetzt wie Treiberbackups ein vollständiges **SHA-256-Manifest**; Manipulation/Beschädigung wird vor Restore erkannt.
- Neue Treiberbackups speichern zusätzlich Modell und erkannte Wheel-Geräte als Kontext.
- Einstellungen, Pedal-Mapping, Modell-/Moduspräferenz, OpenG27-Pfad und Backup-/Migrationsmanifeste werden atomar über temporäre Datei + Replace geschrieben.
- OpenG27 wird nur noch installiert, wenn das GitHub-Asset einen gültigen SHA-256-Digest liefert. Download und Entpacken erfolgen in Staging; ein vorhandener Versionsordner bleibt bis zur erfolgreichen Aktivierung als Rückfallebene erhalten.

### Erkennung & Laufzeit
- Legacy-Treiber- und Profiler-Erkennung unterscheiden jetzt strikt zwischen „nicht vorhanden“ und „Status konnte nicht gelesen werden“. Fehler werden nicht mehr als leerer Zustand fehlinterpretiert.
- Schwergewichtige Treiber-/Profiler-Inventuren werden für die normale Oberfläche kurz gecacht; destruktive Aktionen erzwingen weiterhin einen frischen, strikten Scan.
- `WM_DEVICECHANGE` stößt nach USB-Reenumeration sofort einen Status-Refresh an.
- Direct HID merkt echte Open-/Read-Fehler und verwendet einen begrenzten Reconnect-Backoff statt aggressiver Wiederholungsversuche.

### Updater & Installer
- Standalone-/Portable-Selbstupdates behalten die vorherige EXE als Rollback und stellen sie wieder her, wenn die neue Version nicht aktiviert/gestartet werden kann.
- Update-Downloads haben Größenlimits und werden bei bekannter Asset-Größe zusätzlich auf exakte Byteanzahl geprüft; ZIP-Extraktion hat Datei-/Gesamtgrößenlimits.
- Der Installer bricht ein Update ab, wenn die alte LogiMate-Instanz nach dem Wartefenster noch läuft, statt die EXE trotzdem zu überschreiben.
- Installer-Aktivierung verwendet `.new`/`.previous`, damit ein fehlgeschlagener Replace die bestehende Installation nicht zerstört.

### Tests
- Neue Regressionstests prüfen atomare Dateischreibvorgänge, Migrationsjournal-Roundtrip und Erkennung manipulierter Profiler-Backups.

## 0.0.4-alpha — Persistent Direct HID & Settings-Übersicht

### Dauerhafte Lenkradverbindung
- Direct HID verwendet jetzt den **offenen HID-Handle** als Verbindungsstatus statt eines 2-Sekunden-Timers seit der letzten Eingabe.
- Ein stillstehendes G27 bleibt daher dauerhaft als verbunden sichtbar; der letzte gültige Zustand bleibt stehen, bis Windows tatsächlich einen Geräte-/Read-Fehler meldet.
- Während Windows bei einem Hintergrund-Refresh kurzzeitig keine PnP-/Raw-Input-Liste liefert, hält ein aktiver Direct-HID-Handle die bereits bestätigte G27-Identität stabil.
- Vor dem ersten Eingabereport zeigt LogiMate bereits „Direct HID verbunden“ statt die Erkennung wieder zu verlieren.

### Einstellungen
- Die lange Optionsliste wurde in vier klar getrennte Bereiche gegliedert: **Darstellung**, **Lenkrad & Verhalten**, **Updates** sowie **Windows & Einstieg**.
- Farbmodus bleibt als eigene Auswahl oben stehen.
- Zeichnen und Hit-Test verwenden jetzt dieselbe zentrale Settings-Geometrie; dadurch sind Scrollen, Hover und Anklicken konsistenter und der Code ist einfacher zu warten.
- Keine vorhandene Einstellung wurde entfernt.

## 0.0.3-alpha — Direct HID, H-Shifter & Setup-Migration

### Live-Eingaben
- Ein bestätigtes G27 wird jetzt **direkt von LogiMate über den nativen C29B-HID-Pfad** gelesen; OpenG27 ist dafür nicht mehr erforderlich.
- Wenn der Nutzer im Assistenten ausdrücklich den modernen G27-Weg wählt und das Rad noch als C294 vorliegt, kann LogiMate selbst den bekannten C294→C29B-Native-Mode-Switch ausführen. Passive Erkennung verändert den USB-Modus weiterhin nie.
- OpenG27 darf parallel geöffnet bleiben. Ist dessen `config.json` vorhanden, spiegelt LogiMate Kalibrierung, Deadzones, Sensitivity und Pedalrollen best effort.
- WinMM bleibt nur Fallback für Legacy/andere Räder oder wenn Direct HID noch nicht verfügbar ist.

### Tasten & H-Shifter
- Native G27-Reports werden semantisch dekodiert: 6 Lenkradtasten, beide Schaltwippen, 8 Shifter-Tasten, D-Pad und H-Schaltung 1–6/N/R.
- Neue grafische H-Schaltkulisse mit live markiertem Gang sowie separate Button-Anzeigen für Wheel und Shifter.
- Generische WinMM-Quellen bleiben absichtlich neutral nummeriert, weil alte Treiber die Nummerierung unterschiedlich liefern können.

### Ersteinrichtung
- Schrittfolge neu geordnet: **Lenkradmodell → Betriebsart → Ablauf prüfen/ausführen**.
- Betriebsart kann direkt zwischen **Modern / OpenG27** und **Original Logitech / Legacy** gewählt werden.
- Modern: OpenG27 wird beim G27 zuerst heruntergeladen/verifiziert, dann Profiler-Einstellungen und Treiber gesichert, Profiler optional deinstalliert, Legacy-Treiber entfernt und Direct HID vorbereitet.
- Legacy: gesicherte Logitech-Treiber werden wiederhergestellt; Profiler-Einstellungen/Anwendung sind optional und werden nur automatisch installiert, wenn ein echter Installer mitgesichert werden konnte.
- Begrüßung bleibt dynamisch über den lokalen Windows-Benutzernamen.

### Sicherheit & Cleanup
- Destruktive Modern-Migration startet erst, nachdem OpenG27 (beim G27) vorbereitet und die notwendigen Backups erfolgreich geschrieben wurden.
- Treiberentfernung nutzt jetzt einen gemeinsamen internen Pfad, damit Setup-Migration und manuelle System-Aktion dieselbe Sicherheitslogik verwenden.
- Nicht mehr verwendete Setup-/Input-Hilfsfunktionen entfernt; keine Sicherheits- oder Diagnosefunktionen gekürzt.

## 0.0.2-alpha — Branding, DFGT & Shared-HID

### Branding
- Neues LogiMate-Lenkrad/HUD-Symbol ist direkt in die Anwendung eingebettet; keine externe Bilddatei wird für die Oberfläche benötigt.
- Dasselbe Symbol wird im Hauptfenster, in der Sidebar, im Einrichtungsassistenten und in „Was ist neu?“ verwendet.
- Fenster- und Taskleisten-Icon nutzen das eingebettete Multi-Resolution-ICO.

### Lenkraderkennung
- **Driving Force GT** als unterstütztes Diagnose-/Verwaltungsmodell ergänzt. Native Identität: `VID_046D&PID_C29A`.
- `C294` bleibt bewusst gemeinsamer Logitech-Kompatibilitätsmodus, weil auch ein treiberloses G27 dort als „Driving Force“/„Driving Force GT“-ähnlich benannt werden kann.
- Beweisreihenfolge: SetupAPI/PnP native PID → Raw Input native PID → explizite C294-Zusatzinformation → gespeicherte C294-Auswahl → Live-Input.
- Vier grafische Auswahlkarten im Setup: **Automatisch**, **Driving Force GT**, **G25**, **G27**. Die Auswahl ist nur C294-Fallback; native PIDs gewinnen immer.

### OpenG27 / Live-Sensoren
- Für einen bestätigten G27 mit installiertem OpenG27 bevorzugt LogiMate jetzt einen **read-only Shared-HID-Pfad** statt WinMM.
- OpenG27 darf parallel geöffnet bleiben; LogiMate sendet über diesen Pfad keinerlei FFB-, LED- oder Konfigurationsbefehle.
- Lenken sowie Gas/Bremse/Kupplung werden aus dem nativen C29B-Inputreport gelesen.
- `%APPDATA%\OpenG27\config.json` wird best effort für Steering-/Pedal-Kalibrierung, Deadzones, Sensitivity und Pedalrollen gespiegelt; die Config wird gecacht und nicht pro HID-Report von der Platte gelesen.
- WinMM bleibt Fallback für G25, DFGT und wenn Shared HID beim G27 nicht verfügbar ist.

### Einrichtungsassistent
- Weiterhin nur drei Seiten, aber grafischer aufgebaut und mit LogiMate-Branding.
- Persönliche Begrüßung verwendet lokal den Windows-Benutzernamen des jeweils angemeldeten Nutzers.
- Reihenfolge: **Modell/Erkennung → Live-Sensoren → Sicherheit/Empfehlung**. Keine Treiberänderung erfolgt automatisch.

### Code & Größe
- Modell-/PID-Wissen in zentrale Wheel-Helper ausgelagert statt über viele String-Sonderfälle zu verteilen.
- Live-Input-Quellen über einen einzigen bevorzugten Pfad zusammengeführt.
- OpenG27-Konfiguration wird zeit-/mtime-gecacht, Handles und eingebettete Icons werden beim Beenden freigegeben.
- Release-Build bleibt ohne zusätzliches GUI-Framework oder Runtime und nutzt weiterhin `-trimpath -s -w`.

## 0.0.1-alpha — Erste öffentliche Alpha

LogiMate setzt die Versionsnummer bewusst auf **0.0.1-alpha** zurück. Frühere interne Preview-Nummern waren Entwicklungsstände und sind kein öffentliches SemVer-Versprechen. Ab hier wird das Projekt schrittweise in Richtung 1.0 versioniert.

### Ersteinrichtung
- Neuer optionaler **3-Seiten-Ersteinrichtungsassistent** im selben Fluent-/Windows-Material-Stil wie **„Was ist neu?“**.
- Kleine persönliche Begrüßung über den lokalen Windows-Benutzernamen, ohne Onlinekonto oder Telemetrie.
- Weniger Schritte, dafür ausführlichere Erklärung direkt auf jeder Seite: **Erkennung**, **Live-Eingaben**, **Sicherheit & Empfehlung**.
- Der Assistent verändert niemals automatisch Treiber.
- Beim ersten stabilen Start kann er angeboten werden; Überspringen deaktiviert weitere automatische Angebote.
- Unter **Einstellungen → Ersteinrichtung beim Start anbieten** kann das Angebot jederzeit wieder aktiviert werden.
- **Übersicht → Einrichtung prüfen** öffnet den Guide jederzeit manuell.

### Lenkraderkennung
- Erkennungsreihenfolge neu geordnet und nach Beweisstärke getrennt:
  1. native **SetupAPI/PnP**-Erkennung von `VID_046D` und `PID_C294/C299/C29B`,
  2. eindeutige native PID (`C299 = G25`, `C29B = G27`),
  3. **Raw Input** als unabhängige physische Gegenprüfung bzw. Fallback,
  4. bei `C294` nur explizite G25/G27-Gerätenamen als zusätzliche Modellinformation,
  5. gespeicherte manuelle C294-Bestätigung erst als letzter Fallback,
  6. **WinMM ausschließlich für Live-Eingaben** und nur mit strengem Mehrcontroller-Verhalten.
- Eine native PID überschreibt immer eine alte manuelle C294-Auswahl.
- Raw Input mit `C299/C29B` kann einen nur als C294 sichtbaren PnP-Pfad disambiguieren.
- Bei mehreren WinMM-Controllern werden breite Namen wie „Driving Force“ oder „Racing Wheel“ nicht mehr geraten; nur ein explizites G25/G27 kann automatisch gewählt werden.
- Bei genau einem WinMM-Gerät darf ein generischer/Driving-Force-Name nur dann korreliert werden, wenn PnP/Raw Input vorher bereits ein Logitech G25/G27 bestätigt hat.
- SetupAPI liest den tatsächlichen **DriverInfPath**, damit Legacy-/Generic-HID-Modus zuverlässiger erkannt wird.
- Diagnosebericht zeigt den verwendeten **Erkennungsweg**.

### Hardwaretest
- Grafisches Lenkinstrument, Live-Achsen, Button-Matrix und POV-Anzeige.
- WinMM prüft zuerst mit `joyGetPosEx`, welche Slots wirklich physisch belegt sind; `joyGetNumDevs` wird nur als Slot-Kapazität behandelt.
- **Pedale lernen** erkennt Gas/Bremse/Kupplung anhand echter Bewegung und speichert die Zuordnung getrennt nach Lenkrad und Legacy-/Generic-HID-Modus.
- Raw Input dient als Identitäts-Gegenprüfung; WinMM überschreibt niemals PnP-Modell oder Treibermodus.

### Darstellung
- Vier Modi: **Dunkel**, **Grau**, **Hell**, **System**.
- Windows-Material ist jetzt theme-adaptiv statt einheitlich dunkles Full-Window-Acrylic:
  - **Dunkel → Desktop Acrylic**,
  - **Grau → Mica Alt**,
  - **Hell → Mica**,
  - **System → Mica/Acrylic passend zum effektiven Windows Hell/Dunkel-Modus**.
- Dadurch bleiben Grau und Hell mit aktivierter Transparenz deutlich lesbarer.
- Der Schalter heißt nun **„Transparenz / Windows-Material“**; Safe UI deaktiviert nur das Material, nicht das gespeicherte Theme.
- Segoe UI Variable, Double Buffering, Graustufen-Antialiasing und selbst gezeichnete Fluent Command Buttons bleiben erhalten.
- High Contrast und reduzierte Windows-Animationen werden berücksichtigt.

### Bedienung
- Aktionen geben jetzt im Hauptfenster sichtbares Feedback: Status-Refresh, Windows-Test, Ordner öffnen, OpenG27-Start, Update-Prüfung, Einstellungen zurücksetzen und UAC-Aktionen.
- Fehler beim Öffnen von Daten-, Diagnose- oder Profilordnern werden nicht mehr still ignoriert.
- Diagnose-Kopieren meldet einen Zwischenablagefehler statt scheinbar nichts zu tun.
- Eigene Controls unterstützen Tastaturfokus, Tab/Shift+Tab, Enter/Leertaste und passende Pfeiltastensteuerung.

### Sicherheit & Zuverlässigkeit
- Treiber-Backup bleibt vor dem Entfernen alter Logitech-Treiber verpflichtend.
- Neue Backups enthalten SHA-256-Dateihashes; Restore prüft diese vor Installation.
- HVCI/Memory Integrity wird niemals still deaktiviert.
- Privilegierte Treiberaktionen bleiben hinter Windows-UAC und expliziter Bestätigung.
- Portable-/Standalone-Updates werden nach Prozessende über einen nativen Helper angewendet; die laufende EXE wird nie direkt überschrieben.
- Update-Assets müssen die SHA-256-Prüfung bestehen.
- Startup-/Update-Logs werden rotiert.

### Plattform & Status
- **Windows 11 x64** ist das primäre Ziel; **Windows 10 x64** ist das Mindestziel der Go-1.23-Release-Builds.
- ARM64 wird in CI nur kompiliert und noch nicht als hardware-validiert verteilt.
- G27 ist das primäre Hardwareziel.
- G25-Erkennung, Diagnose und Treiberverwaltung sind implementiert, bleiben aber bis zu echten C294/C299-/Shifter-/Pedal-/FFB-Tests experimentell.
- Preview-Binaries sind noch nicht Authenticode-signiert.

## 0.0.1-alpha · Build 017 — GitHub release candidate

- Added pinned-SHA CodeQL analysis on Windows.
- Added a manually reviewable release-candidate pipeline with artifact provenance attestations.
- Added Dependabot coverage for GitHub Actions and Go modules.
- Added an exact GitHub repository/ruleset/security configuration guide.
- Synchronized public issue templates with Build 017 while keeping the visible app version at 0.0.1-alpha.
