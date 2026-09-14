# LogiMate 0.0.1-alpha · Build 001 — Technical & UX Audit

**Audit date:** 2026-09-14  
**Functional baseline:** D6.2 FFB Verification / Control-Panel Rework  
**Visible product version:** frozen at `0.0.1-alpha`  
**Internal revision:** `Build 001`

## Executive result

The native wheel/FFB architecture does not need another rewrite. The central OutputLease, bounded HID transport, recovery marker, Native Wheel Engine, D4 signal shaping and D6 control panel remain the correct baseline. Build 001 therefore focuses on version identity, persistence integrity, calibration correctness and product-state truth before the larger UI/UX rework.

Automated gates used in this pass:

- host tests for `wheelengine`, `gameadapter` and historical OpenG27 parity;
- host race detector for the platform-independent engine/adapter packages;
- Windows amd64 `go vet -unsafeptr=false`;
- Windows amd64 compile of `internal/system`, `internal/app` and `cmd/logimate` tests;
- Windows amd64 GUI application build;
- Windows arm64 GUI compile validation;
- productive-runtime dependency scan to keep `internal/openg27port` out of `cmd/logimate`.

Physical G25/G27/DFGT validation is still a separate hardware gate and is not claimed by this source audit.

## Fixed in Build 001

### A001 — Public version identity was inconsistent
**Severity:** High · **Fixed**

The source previously contained the current 0.6.2 marker plus historical fallback versions in application and installer code. The public product version is now derived as `0.0.1-alpha`, with a separate `BUILD` value. Application, installer, diagnostics, changelog and build pipeline receive both values.

### A002 — Frozen public SemVer could make historical 0.5/0.6 previews look newer
**Severity:** Blocker for updater · **Fixed**

During the pinned alpha line, the internal update identity is `0.0.1-alpha.<build>`. Update discovery accepts only candidates from the same pinned line. Historical preview tags such as 0.5.x/0.6.x are ignored rather than offered as an upgrade.

### A003 — “Später” in first-run setup behaved like completion
**Severity:** High UX/logic · **Fixed**

Deferring setup now keeps `SetupCompleted=false`, persists the current step and stores a 24-hour snooze timestamp. Completing setup is a distinct state.

### A004 — Calibration/learning could accept a connected handle without a valid current sample
**Severity:** High data integrity · **Fixed**

Button, H-shifter and steering learning now require `SampleValid` from the current device generation and reject malformed/read-error/reconnect states before persisting calibration.

### A005 — Input profile corruption/future schema could be lost on writes
**Severity:** High persistence · **Fixed**

Input profiles now use strict reads before mutations. Corrupt files and unknown future schema versions block writes instead of being replaced.

### A006 — Native Engine profile corruption/future schema could be lost on writes
**Severity:** High persistence · **Fixed**

Native Wheel Engine profile mutations now use strict schema-aware loading and fail closed.

### A007 — Native FFB configuration accepted unknown future schema during save
**Severity:** High persistence/safety · **Fixed**

`native-ffb.json` is now schema-gated before mutation. An older LogiMate build cannot overwrite a newer Native FFB configuration.

### A008 — Wheel confirmation loader claimed corrupt data blocked writes, but save paths could overwrite it
**Severity:** High device-identity safety · **Fixed**

Persistent wheel-model confirmation now has a strict mutation loader. Corrupt/future confirmation files block Save/Clear operations while read-only runtime falls back conservatively.

### A009 — Automatic native identity learning could overwrite corrupt/newer history
**Severity:** Medium/High identity reliability · **Fixed**

Background native-identity learning refuses to rewrite a corrupt or unsupported future history file. Discovery remains usable but does not destroy recovery evidence.

### A010 — `settings.json` had no schema boundary
**Severity:** High UX persistence · **Fixed**

UI settings now carry schema version 1. Files without a version migrate in memory as v1. A future schema remains readable where possible but becomes write-protected until an explicit settings reset, preventing silent downgrade data loss.

## Confirmed architecture strengths

1. **One productive native wheel engine:** modern wheel behavior remains LogiMate-owned.
2. **No normal OpenG27 runtime dependency:** provenance/parity/import code stays outside the productive app dependency graph.
3. **Central motor ownership:** OutputLease/Watchdog/Emergency Neutralize remain the authority for motor-driving paths.
4. **D4 safety ordering is correct:** user shaping happens before hard NativeFFB safety caps.
5. **D6 FFB UI is attached to real persisted engine state:** it is not a mock settings panel.
6. **Config writes use the strict/atomic persistence layer in the important mutable paths audited here.**
7. **Updater integrity and Stable Authenticode gate already exist;** private signing material remains external to source control.

## Open technical work after Build 001

### P0 — Real G27 validation
- cold boot/replug C294 -> C29B without manual confirmation;
- steering/pedals/buttons/paddles/H-shifter;
- Constant/Spring/Damper/Friction/Autocenter at bounded levels;
- FFB 0/25/50/75/100% response checks within hard caps;
- rotation 270/360/540/720/900;
- Emergency Stop for every effect;
- USB yank/reconnect while output is active;
- process kill and recovery;
- suspend/resume;
- Modern -> Legacy -> Modern rollback.

### P0 — Remaining product-state truth
The UI still exposes too many advanced actions through modal menus and dialogs. The engine is now ahead of the information architecture. The next code phase should be a UI structure change rather than another protocol rewrite.

### P1 — Diagnostic event unification
Errors currently originate from several state fields/subsystems. Introduce a central `DiagnosticEvent` model with severity, subsystem, stable code, timestamp, active/resolved state, user action and technical detail. This becomes the source for Diagnostics 2.0 and support exports.

### P1 — Settings/concurrency follow-up
Continue replacing intentionally ignored non-critical persistence errors with visible diagnostic events where user state could be affected. Temporary cleanup failures may remain best-effort, but state-changing writes should never be silently discarded.

### P1 — G25 / DFGT certification
Software paths exist, but Stable support claims remain blocked until physical devices pass the same matrix.

## UX audit — recommended structure

### Main navigation
Keep the top-level navigation compact:

`Startseite | Lenkrad | System | Diagnose | Einstellungen | Über mich`

### Wheel page
Replace the current “Kalibrieren & Lernen” action menu with in-page wheel tabs:

`Live | Force Feedback | Kalibrierung | Profile | Gerät`

- **Live:** current input dashboard and advanced raw view.
- **Force Feedback:** D6 control panel.
- **Kalibrierung:** steering, pedals, buttons/paddles, H-shifter as status cards and inline workflows.
- **Profile:** Wheel profile + Game profile + effective-settings stack.
- **Gerät:** identity, VID/PID, mode, HID paths/reports, session/persistent identity and wheel-specific certification.

### Diagnostics 2.0
Internal tabs:

`Übersicht | Probleme | Tests | Protokoll | Export | Release`

Move Engine Health, HID Stress, hardware certification, recovery tests and Release Trust here. Diagnostics should preserve errors after transient dialogs disappear.

### First-run Setup 2.0
Replace the text-heavy 3-page wizard with a resumable state machine:

1. Welcome / wheel detection
2. Operating mode (Modern recommended vs Legacy)
3. Safety and migration preflight
4. Installation/mode-change progress
5. Input live test
6. Calibration
7. Bounded FFB test
8. Summary

“Später erinnern” snoozes and resumes. It never means “completed”. The Start page should show setup progress until all required steps are complete.

### About 2.0
Keep the current visual language but simplify hierarchy:

- Hero: LogiMate + `0.0.1-alpha` + Build ID
- “About LogiMate”
- “About Markus”
- Open source / credits / OpenG27 provenance
- related projects in one consistent card grid
- privacy statement
- support links at the bottom

Avoid hard-coded roadmap phrases that become stale between builds.

## Proposed implementation order

1. **Build 001:** frozen version baseline + technical integrity fixes (this release).
2. **Build 002:** Wheel navigation rework; remove “Kalibrieren & Lernen” popup menu.
3. **Build 003:** Calibration 2.0 inline workflows.
4. **Build 004:** Profiles 2.0 + Effective Settings stack.
5. **Build 005:** Diagnostics event model + Diagnostics 2.0.
6. **Build 006:** First-run Setup 2.0.
7. **Build 007:** About 2.0 + global UX consistency pass.
8. **Build 008+:** hardware findings, accessibility, HID stress and release gates until Alpha is ready to leave the frozen 0.0.1 line.

The visible product version remains `0.0.1-alpha` through these builds unless explicitly changed by project decision.
