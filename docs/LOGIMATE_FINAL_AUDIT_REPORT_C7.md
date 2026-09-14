# LogiMate 0.2.8-alpha Fusion C7 — Final Plan-D Engineering Audit Report

**Audit date:** 2026-09-12  
**Baseline:** LogiMate 0.2.8-alpha — Fusion C7 / Native Modern Cutover  
**Audit scope:** A-001 through A-100  
**Production-code policy:** audit-only; no production Go code was changed  
**Next checkpoint:** C8 — Reliability & Architecture Cleanup

---

## Executive verdict

The Plan-D engineering audit is **complete: 100 / 100 reviewed**.

The C7 codebase is a useful and testable alpha foundation, but it is **not ready for a stable/native-motor release and should not move directly into D1 feature expansion**. The reason is not one isolated bug: several ownership, emergency-stop, crash-recovery, migration and telemetry gates can disagree about which process/session/device is allowed to control the wheel.

Current audit distribution:

- **24 BLOCK** findings
- **48 WARN** findings
- **19 PASS** findings
- **9 NOT TESTABLE WITHOUT HARDWARE / external validation gates**

The correct next move is **C8**, focused on making output ownership, neutralization, identity, migration transactions and persistence fail-safe. After C8 closes the blockers, **D0** can safely consolidate the native wheel architecture before G25/DFGT expansion.

---

## What was actually verified

The full C7 source tree was reviewed across architecture, device discovery, input, output/FFB, shutdown/recovery, Modern/Legacy migration, telemetry/game profiles, UI state truth, updater/security and release engineering.

Software baseline checks were re-run after the audit documentation was completed:

- `go test ./internal/openg27port/...` — **PASS**
- Windows amd64 application cross-build — **PASS**
- Windows amd64 `go vet -unsafeptr=false ./...` — **PASS**
- Windows `internal/system` test binary compilation — **PASS**
- Windows `internal/app` test binary compilation — **PASS**
- Windows ARM64 application validation build — **PASS**
- Installer `go vet -tags installer` — **PASS**
- Installer Windows amd64 build — **PASS**
- Current production source compared with the original extracted C7 source excluding audit/docs — **0 differences**

Windows-only tests and real HID/motor behavior cannot be executed on the Linux audit host. Those limitations are deliberately represented as external/hardware gates instead of false PASS results.

---

## 24 release-blocking findings

### Output ownership / emergency safety

- **A-007** — FFB and Autocenter do not share one exclusive motor-output lease
- **A-008** — `NativeOutputRelease()` can cancel the Autocenter watchdog without sending a neutral command
- **A-009** — Emergency Stop is coupled to normal output validation and can be blocked by the condition it is supposed to recover from
- **A-031** — FFB session hand-off can time out while the old hardware writer is still alive
- **A-033** — Crash-recovery marker creation errors are ignored for active motor sessions
- **A-034** — Release can clear the recovery marker without proving the output worker stopped
- **A-041** — Startup recovery clears the pending-output marker even when emergency neutralization fails
- **A-042** — Pending-output recovery is one-shot and is not retried when prerequisites are initially unavailable
- **A-043** — A panic inside `WM_DESTROY` can skip the remainder of motor shutdown
- **A-044** — Windows session-end/power-shutdown messages do not have an explicit output-safety path
- **A-045** — PnP topology changes ignore emergency-stop failure and then release output state
- **A-072** — Turning Experimental Native Output off can release bookkeeping without guaranteed neutralization
- **A-081** — There is no OS-wide single-instance/output-owner lock for LogiMate itself

### Device / input correctness

- **A-011** — Persisted “StableWheelID” is a USB-port identity, not a physical-device identity
- **A-012** — Native SetupAPI enumeration can return a partial topology as successful
- **A-013** — Synthetic Direct-HID continuity device can become PnP-verified
- **A-022** — Connected Direct HID can be reported as `Found=true` before any real input sample exists
- **A-024** — `AxisValues()` has unsigned-underflow normalization that can turn below-minimum input into 100%

### Migration transaction safety

- **A-051** — Mode migrations can commit without verifying the requested final device state
- **A-052** — Migration journal durability errors are ignored after journal creation
- **A-053** — Rollback is best-effort and has no verified terminal “rollback succeeded” postcondition

### Game / telemetry motor authorization

- **A-061** — A telemetry-LED-only game profile can still drive the wheel motor
- **A-062** — Pino telemetry `FFBEnabled=false` is ignored by the Fusion C6 motor source

### Overall release gate

- **A-100** — C7 is not ready for a stable/native-motor release until C8 blockers and external gates are closed

The most important pattern is that several “safety” functions currently reuse the same validity gates designed for **starting a new command**. Emergency neutralization must be different: once LogiMate has held a hardware lease, stopping it must use the captured lease/target and fail conservatively even if discovery, mode, process ownership or topology changed afterward.

---

## Strong foundations already worth keeping

The audit also confirmed substantial good work. These parts should be preserved rather than rewritten unnecessarily:

- fail-closed stored-target behavior when a selected wheel disappears;
- exact G27 Raw-HID target correlation in the healthy single/multi-device path;
- bounded force/effect packet builders and mixer safety ceilings;
- proactive stop-before-rediscovery ordering on `WM_DEVICECHANGE`;
- loopback-only telemetry listeners with packet/numeric bounds;
- explicit UAC boundary for driver/HVCI operations;
- driver backup SHA-256 validation before restore;
- profiler backup integrity/path validation;
- setup close protection during tracked migration;
- explicit pre-UAC destructive-plan confirmation;
- ZIP traversal/decompression limits in update extraction;
- tag-release workflow with vet/tests/smoke/build/portable/source/checksums;
- checksum-consistent immutable C7 baseline package.

These are the pieces C8/D0 should build around.

---

## C8 implementation order

### P0 — Motor/output safety: must be first

1. Introduce an **OS-wide/per-wheel output lease** and prevent a second LogiMate process from acquiring the same wheel.
2. Route every hardware output through **one serialized OutputEngine**: constant force, spring, damper, friction, range, LEDs and autocenter.
3. Separate **command authorization** from **emergency neutralization**. Emergency stop uses the captured leased target and must not be blocked by normal discovery/OpenG27/mode gates.
4. Remove the 350 ms “assume stopped” hand-off. A new session cannot start until the old transport/worker is conclusively closed; otherwise enter `FAULTED/RECOVERY_REQUIRED`.
5. Make recovery-marker creation part of lease acquisition. Never start motor force if the marker cannot be durably written; never clear it until neutralization + handle closure are confirmed.
6. Harden startup recovery, device-change, close, panic, Windows session-end and power-transition paths around the same idempotent fail-safe shutdown routine.
7. Fix Fusion C6 authorization: **LED-only = zero motor force**, and non-zero force requires profile FFB enable + adapter FFB enable + fresh Physics + PlayerControl.

### P1 — Identity, input and migration correctness

8. Replace port-only `StableWheelID` trust with a stronger identity/fingerprint model and make SetupAPI enumeration strictly fail closed.
9. Add canonical `WheelInputState` with device/session/source/layout/sample-valid/generation/timestamp fields.
10. Fix unsigned axis normalization underflow, reject calibration before a first valid sample, bind pedal calibration to semantic input layout, and separate D-pad from button masks.
11. Turn Modern/Legacy changes into verified transactions: durable journal writes, explicit final postconditions, verified rollback, unresolved-migration recovery on next startup, and real HVCI requested/effective/reboot/policy states.

### P2 — Persistence, UI and release hardening

12. Make atomic writes concurrency-safe and surface/recover corrupt configuration instead of silently defaulting/overwriting it.
13. Reject unknown future data-schema versions from write access.
14. Make UI safety claims derive from backend lease/health truth. Disabling Native Output must be a verified neutralization action.
15. Add updater transaction binding and Authenticode publisher verification for stable releases.
16. Unify local/CI release packaging and version source; fix the stale `0.0.3-alpha` main-CI version.

---

## C8 definition of done

C8 is not complete when the app merely builds. It is complete when all of the following are true:

- every **A-xxx with Status BLOCK** is changed to PASS or, for a genuinely external requirement, to an explicit hardware/release gate with no unsafe production path enabled;
- only one process/session can own wheel output at a time;
- Emergency Stop and shutdown cannot be rejected because normal discovery state changed;
- output recovery markers are durable and truthful;
- a stalled output worker cannot coexist with a replacement worker;
- LED-only/Pino-FFB-disabled telemetry cannot produce motor force;
- device enumeration and selected-wheel identity fail closed;
- no calibration is saved from a synthetic/unvalidated sample;
- Modern/Legacy transaction success is proven by fresh postcondition checks and failed rollback becomes a persistent recovery state;
- corrupted config is surfaced rather than silently replaced;
- Windows software tests, build/vet, installer build and regression suite pass;
- the Master Audit file is updated with each resolved item and test evidence.

After that, proceed to **D0 Native Engine Foundation**, not directly to broad model-specific feature code.

---

## D0 architecture recommendation after C8

The post-C8 architecture should make invalid ownership states difficult or impossible to represent:

```text
WheelManager
├── DeviceRegistry
│   └── WheelDescriptor / Capabilities
├── InputEngine
│   └── WheelInputSession -> WheelInputState
├── OutputEngine
│   ├── OS/per-wheel OutputLease
│   ├── serialized command queue
│   ├── SafetyController
│   └── WheelTransport
├── ProfileManager
├── MigrationManager
└── AdapterManager
```

A motor/output lease should include at least:

```text
WheelID / strong fingerprint
SessionID / topology generation
ModelID / native PID
Raw HID target path
Owner process/session
Lease generation
AcquiredAt
Recovery marker ID
State: IDLE | ACTIVE | STOPPING | FAULTED | RECOVERY_REQUIRED
```

No UI, telemetry adapter or model-specific helper should write HID output outside this owner.

---

## External gates that remain after code fixes

The audit intentionally does not pretend software inspection proves physical behavior. Before stable status, the release still needs:

- G25/G27/DFGT physical input/output matrix for every enabled capability;
- low-force crash/process-kill/USB-yank/reconnect/sleep/resume/shutdown/no-stuck-force tests;
- Modern↔Legacy/HVCI/reboot/rollback validation on supported Windows 11 builds;
- Windows race/handle/goroutine stress under repeated device churn;
- full UI/accessibility/mixed-DPI guided-flow validation;
- Authenticode-signed application and installer with publisher verification.

---

## Complete A-001–A-100 index

The detailed evidence, risk, fix and acceptance criteria for every item are in `LOGIMATE_MASTER_AUDIT_PLAN.md`. This compact index is the final audit inventory.

| ID | Status | Severity | Finding |
|---|---|---|---|
| A-001 | WARN | MEDIUM | Architecture documentation contradicts C7 runtime |
| A-002 | WARN | HIGH | Capability/descriptor layer does not exist yet |
| A-003 | WARN | HIGH | `internal/system` is a multi-responsibility monolith |
| A-004 | WARN | HIGH | Model/mode truth is stringly typed and can misclassify aggregate multi-wheel state |
| A-005 | WARN | HIGH | Native G27 input behavior still depends on external OpenG27 config when present |
| A-006 | WARN | HIGH | Multiple live output/protocol implementations remain active with known byte differences |
| A-007 | BLOCK | BLOCK | FFB and Autocenter do not share one exclusive motor-output lease |
| A-008 | BLOCK | BLOCK | `NativeOutputRelease()` can cancel the Autocenter watchdog without sending a neutral command |
| A-009 | BLOCK | BLOCK | Emergency Stop is coupled to normal output validation and can be blocked by the condition it is supposed to recover from |
| A-010 | PASS | LOW | C7 source/release baseline is buildable and internally coherent enough to continue the audit |
| A-011 | BLOCK | BLOCK | Persisted “StableWheelID” is a USB-port identity, not a physical-device identity |
| A-012 | BLOCK | BLOCK | Native SetupAPI enumeration can return a partial topology as successful |
| A-013 | BLOCK | HIGH | Synthetic Direct-HID continuity device can become PnP-verified |
| A-014 | WARN | HIGH | Raw-Input fallback identities can be persisted as stable selections |
| A-015 | WARN | MEDIUM | Raw Input discovery has no error/completeness channel |
| A-016 | PASS | LOW | Missing persisted target and unselected multi-wheel states fail closed |
| A-017 | PASS | LOW | G27 selected-wheel to Raw-HID correlation is exact and fail-closed |
| A-018 | WARN | HIGH | G25/DFGT multi-wheel input discards available selected-device correlation |
| A-019 | WARN | HIGH | Native output remains intentionally machine/session single-wheel-only |
| A-020 | NOT TESTABLE WITHOUT HARDWARE | HIGH validation gate | Physical identity/reconnect claims still require a real Windows hardware matrix |
| A-021 | WARN | HIGH | `JoyState` is not an authoritative per-device input sample |
| A-022 | BLOCK | HIGH | Connected Direct HID can be reported as `Found=true` before any real input sample exists |
| A-023 | WARN | HIGH | Pedal calibration is keyed to wheel/mode, not to the actual input layout/source used to learn it |
| A-024 | BLOCK | HIGH | `AxisValues()` has unsigned-underflow normalization that can turn below-minimum input into 100% |
| A-025 | WARN | HIGH | Steering calibration accepts impossible geometry and arbitrarily tiny travel |
| A-026 | WARN | HIGH | Classic G25/DFGT `Buttons` mask includes encoded D-pad/payload bits as if they were logical buttons |
| A-027 | WARN | HIGH | G25/DFGT malformed Direct-HID reports are silently dropped while the previous state remains apparently usable |
| A-028 | NOT TESTABLE WITHOUT HARDWARE | HIGH validation gate | G25 H-shifter decoding is threshold-based and not physically certified |
| A-029 | WARN | MEDIUM | WinMM input telemetry is process-global instead of per selected controller/session |
| A-030 | WARN | MEDIUM | Input profile read failures silently degrade to empty calibration/mappings |
| A-031 | BLOCK | BLOCK | FFB session hand-off can time out while the old hardware writer is still alive |
| A-032 | WARN | HIGH | Motor-output start APIs report success before HID startup has actually succeeded |
| A-033 | BLOCK | BLOCK | Crash-recovery marker creation errors are ignored for active motor sessions |
| A-034 | BLOCK | BLOCK | Release can clear the recovery marker without proving the output worker stopped |
| A-035 | WARN | HIGH | Native engine profile application is not fully transactional |
| A-036 | PASS | LOW | Effect builders, slot allocation and mixer safety caps are well bounded |
| A-037 | WARN | HIGH | Synchronous HID writes have no hard I/O timeout or cancellation primitive |
| A-038 | WARN | MEDIUM | Native FFB/profile JSON corruption silently falls back to defaults |
| A-039 | NOT TESTABLE WITHOUT HARDWARE | HIGH validation gate | Native output protocol still requires physical model certification |
| A-040 | WARN | HIGH | Range, LEDs, autocenter and FFB still bypass one authoritative output command queue |
| A-041 | BLOCK | BLOCK | Startup recovery clears the pending-output marker even when emergency neutralization fails |
| A-042 | BLOCK | BLOCK | Pending-output recovery is one-shot and is not retried when prerequisites are initially unavailable |
| A-043 | BLOCK | BLOCK | A panic inside `WM_DESTROY` can skip the remainder of motor shutdown |
| A-044 | BLOCK | HIGH | Windows session-end/power-shutdown messages do not have an explicit output-safety path |
| A-045 | BLOCK | BLOCK | PnP topology changes ignore emergency-stop failure and then release output state |
| A-046 | PASS | LOW | Device-change handling proactively stops output before refreshing identity |
| A-047 | WARN | HIGH | Runtime output recovery marker lacks a strong target/session fingerprint |
| A-048 | PASS | LOW | Native FFB has conservative gain/slew/watchdog limits and OpenG27 coexistence checks |
| A-049 | WARN | HIGH | State invariants do not detect all possible LogiMate-internal dual-output ownership |
| A-050 | NOT TESTABLE WITHOUT HARDWARE | BLOCK validation gate | Kill/crash/USB-yank/suspend stuck-force behavior is not physically certified |
| A-051 | BLOCK | BLOCK | Mode migrations can commit without verifying the requested final device state |
| A-052 | BLOCK | HIGH | Migration journal durability errors are ignored after journal creation |
| A-053 | BLOCK | HIGH | Rollback is best-effort and has no verified terminal “rollback succeeded” postcondition |
| A-054 | WARN | HIGH | Operating-mode preference write failure is only a warning even though migration returns success |
| A-055 | WARN | HIGH | HVCI/Memory Integrity handling models a registry request, not the effective security state |
| A-056 | PASS | LOW | Fresh Legacy install verifies the official Logitech installer signer before execution |
| A-057 | PASS | LOW | Driver backup restore verifies recorded SHA-256 hashes |
| A-058 | PASS | LOW | Profiler backup/restore has integrity and bounded-path checks |
| A-059 | WARN | MEDIUM | Old migration cleanup can delete unresolved transaction journals solely by age |
| A-060 | NOT TESTABLE WITHOUT HARDWARE | HIGH validation gate | Modern↔Legacy/HVCI rollback remains a real-Windows hardware gate |
| A-061 | BLOCK | BLOCK | A telemetry-LED-only game profile can still drive the wheel motor |
| A-062 | BLOCK | BLOCK | Pino telemetry `FFBEnabled=false` is ignored by the Fusion C6 motor source |
| A-063 | WARN | HIGH | Automatic telemetry startup errors are swallowed by game-session logic |
| A-064 | WARN | HIGH | Background game-profile selection is nondeterministic when multiple matching games run |
| A-065 | WARN | MEDIUM | Corrupt game-profile storage silently becomes an empty/default profile set |
| A-066 | WARN | MEDIUM | Pino telemetry packet diagnostics never increment `Packets` |
| A-067 | PASS | LOW | Telemetry listeners are loopback-only and parsing is bounded |
| A-068 | PASS | LOW | Telemetry freshness plus Physics/PlayerControl gates are present |
| A-069 | PASS | LOW | Game-profile import is size-bounded and normalized before persistence |
| A-070 | WARN | HIGH | Telemetry “adapter registry” is metadata, not yet an adapter lifecycle interface |
| A-071 | WARN | HIGH | UI claims an exclusive output gate that C7 does not actually enforce |
| A-072 | BLOCK | BLOCK | Turning Experimental Native Output off can release bookkeeping without guaranteed neutralization |
| A-073 | WARN | MEDIUM | Experimental motor-output opt-in is a one-click settings toggle with no durable risk acknowledgement |
| A-074 | WARN | MEDIUM | Corrupt UI settings silently reset individual/all values to defaults |
| A-075 | WARN | LOW | Obsolete `AutoStartOpenG27` remains in the settings schema after the C7 cutover |
| A-076 | WARN | MEDIUM | Setup can report completion even when its completion preference failed to persist |
| A-077 | PASS | LOW | Destructive setup flow fails closed on ambiguous/no/multiple wheel selection |
| A-078 | PASS | LOW | Setup window cannot be casually closed while a tracked migration is running |
| A-079 | PASS | LOW | Destructive mode changes have explicit user plan/confirmation before elevation |
| A-080 | NOT TESTABLE WITHOUT HARDWARE | MEDIUM validation gate | End-to-end Windows UI/accessibility/hardware workflow still requires external validation |
| A-081 | BLOCK | BLOCK | There is no OS-wide single-instance/output-owner lock for LogiMate itself |
| A-082 | WARN | HIGH | Atomic file writes use a fixed `.tmp` name and are not safe under concurrent writers |
| A-083 | WARN | HIGH | Updater verifies integrity but not independent publisher authenticity |
| A-084 | PASS | LOW | ZIP extraction has traversal and decompression-size defenses |
| A-085 | WARN | MEDIUM | Native updater helper accepts arbitrary apply paths from command-line arguments |
| A-086 | WARN | HIGH | Update/uninstall lifecycle is not coordinated with another running LogiMate/output owner |
| A-087 | WARN | MEDIUM | Diagnostic exports redact paths but still expose detailed device identifiers/topology |
| A-088 | PASS | LOW | Privileged driver/HVCI operations are explicitly UAC-gated rather than silently elevated |
| A-089 | WARN | HIGH | External OpenG27 ownership detection is executable-name based and can miss renamed instances |
| A-090 | NOT TESTABLE WITHOUT HARDWARE | HIGH validation gate | Windows race/handle-leak stress and real HID concurrency remain unexecuted gates |
| A-091 | WARN | HIGH | Main-branch CI artifacts are stamped with stale version `0.0.3-alpha` |
| A-092 | WARN | HIGH | Stable release signing is explicitly missing |
| A-093 | PASS | LOW | Tag release workflow already has a solid software build/test/package baseline |
| A-094 | WARN | MEDIUM | Local `build.ps1` and CI release packaging are not equivalent |
| A-095 | NOT TESTABLE WITHOUT HARDWARE | BLOCK validation gate | Required G25/G27/DFGT hardware matrix is documented but not an enforceable release artifact |
| A-096 | NOT TESTABLE WITHOUT HARDWARE | BLOCK validation gate | Crash-recovery/no-stuck-force certification has no automated or recorded release gate |
| A-097 | WARN | HIGH | Incomplete migration journals are not discovered/recovered on normal application startup |
| A-098 | WARN | HIGH | Data schema accepts unknown newer versions instead of blocking downgrade writes |
| A-099 | PASS | LOW | Audited C7 release package is internally checksum-consistent and archives are structurally readable |
| A-100 | BLOCK | BLOCK | C7 is not ready for a stable/native-motor release until C8 blockers and external gates are closed |

---

## Final decision

**C7 audit result: NOT READY FOR STABLE / READY TO ENTER C8.**

Do not add broad G25/DFGT feature expansion before C8. Fix the motor-output ownership/recovery cluster first, then identity/input/migration, then persistence/UI/release hardening. Once the blocker set is closed and regression evidence is recorded, D0 can safely unify the native wheel engine and Plan D can continue.

**Canonical continuation files:**

1. `LOGIMATE_MASTER_AUDIT_PLAN.md` — full 100-point evidence and acceptance criteria.
2. `LOGIMATE_FINAL_AUDIT_REPORT_C7.md` — this consolidated decision report and C8 execution plan.
