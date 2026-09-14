# LogiMate 0.2.9-alpha — Fusion C8 final audit / fix report

Baseline: **0.2.8-alpha Fusion C7**, audited A-001…A-100. C8 is the remediation release. This report never converts a physical/hardware claim into PASS without evidence.

## Executive result

- All **24 C7 BLOCK findings** are closed at the software/release-policy level for the C8 alpha; A-100 becomes “Alpha ready / Stable gated”.
- **65** findings are code-fixed, **20** prior PASS findings are retained/rechecked, **11** remain explicit external certification gates, and **3** non-blocking architecture items are deliberately carried into D0.
- The remaining external gates are not software defects hidden as PASS: real G25/G27/DFGT behavior, kill/crash/USB/suspend recovery, real Windows migration/HVCI, HID stress, Windows UI/accessibility, and Stable Authenticode execution.
- C8 therefore qualifies as a **software-complete alpha**, not a Stable/native-motor certification.

## C8 safety changes that close the original blockers

- Central in-process + system-wide native output ownership; second GUI instance and second native writer are blocked.
- Mandatory pre-force recovery marker; marker/lease survive any unconfirmed stop. Recovery correlates wheel/session/model and strong serial-derived fingerprint when available.
- Conservative C294 authorization: PnP/topology is not model proof; no topology-only cross-session model inheritance.
- Fail-closed SetupAPI/Raw Input discovery and selected-wheel Direct-HID correlation.
- Input validity/calibration fixes including the confirmed unsigned pedal normalization bug.
- Journal-required, terminal-state-verified Modern/Legacy migration and rollback.
- LED-only/Pino force gating prevents telemetry-only configuration from driving the motor.
- Corruption-preserving settings/profile persistence, unique atomic temp files and downgrade-schema protection.
- Authenticode same-publisher automatic-update gate, one-shot update helper authorization, singleton/recovery-aware update/uninstall.

## Revalidation A-001…A-100

| ID | C7 | C8 | Finding | C8 disposition |
|---|---|---|---|---|
| A-001 | WARN | CODE FIXED | Architecture documentation contradicts C7 runtime | Current architecture documentation rewritten for Native-by-default C8 ownership. |
| A-002 | WARN | CODE FIXED | Capability/descriptor layer does not exist yet | Central WheelCapabilities descriptor added for G25/G27/DFGT PID, selector and capabilities. |
| A-003 | WARN | D0 ARCHITECTURE DEBT | `internal/system` is a multi-responsibility monolith | The broad internal/system package is still intentionally decomposed in D0; C8 fixes safety ownership without a risky late package rewrite. |
| A-004 | WARN | D0 ARCHITECTURE DEBT | Model/mode truth is stringly typed and can misclassify aggregate multi-wheel state | Safety/protocol decisions now use typed capabilities, but legacy/UI model/mode strings remain for D0 cleanup. |
| A-005 | WARN | CODE FIXED | Native G27 input behavior still depends on external OpenG27 config when present | Native G27 input uses LogiMate calibration defaults; external OpenG27 config no longer controls native parsing. |
| A-006 | WARN | CODE FIXED | Multiple live output/protocol implementations remain active with known byte differences | C3 live writer removed as an independent production writer; live output converges on the central native path. |
| A-007 | BLOCK | CODE FIXED | FFB and Autocenter do not share one exclusive motor-output lease | One native output lease + system-wide mutex now serializes FFB/autocenter/range/LED ownership. |
| A-008 | BLOCK | CODE FIXED | `NativeOutputRelease()` can cancel the Autocenter watchdog without sending a neutral command | Release/disable paths must neutralize successfully before motor ownership can be dropped. |
| A-009 | BLOCK | CODE FIXED | Emergency Stop is coupled to normal output validation and can be blocked by the condition it is supposed to recover from | Emergency stop is routed through a dedicated fail-closed coordinator and is not blocked by the normal start gate. |
| A-010 | PASS | PASS / RETAINED | C7 source/release baseline is buildable and internally coherent enough to continue the audit | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-011 | BLOCK | CODE FIXED | Persisted “StableWheelID” is a USB-port identity, not a physical-device identity | C294 topology is no longer treated as physical hardware identity; session-only confirmation is default without a strong serial fingerprint. |
| A-012 | BLOCK | CODE FIXED | Native SetupAPI enumeration can return a partial topology as successful | SetupAPI enumeration fails on unexpected enumeration errors instead of accepting a partial topology. |
| A-013 | BLOCK | CODE FIXED | Synthetic Direct-HID continuity device can become PnP-verified | Synthetic DIRECT continuity devices can no longer become PnP-verified targets. |
| A-014 | WARN | CODE FIXED | Raw-Input fallback identities can be persisted as stable selections | Only PnP-verified stable targets can be persisted as selections. |
| A-015 | WARN | CODE FIXED | Raw Input discovery has no error/completeness channel | Raw Input discovery has an error-returning strict path used by state collection. |
| A-016 | PASS | PASS / RETAINED | Missing persisted target and unselected multi-wheel states fail closed | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-017 | PASS | PASS / RETAINED | G27 selected-wheel to Raw-HID correlation is exact and fail-closed | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-018 | WARN | CODE FIXED | G25/DFGT multi-wheel input discards available selected-device correlation | G25/DFGT Direct-HID input now correlates to the selected PnP wheel instead of discarding target correlation. |
| A-019 | WARN | PASS / RETAINED | Native output remains intentionally machine/session single-wheel-only | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-020 | NOT TESTABLE WITHOUT HARDWARE | EXTERNAL GATE | Physical identity/reconnect claims still require a real Windows hardware matrix | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-021 | WARN | CODE FIXED | `JoyState` is not an authoritative per-device input sample | JoyState carries wheel/session/source/layout/sample-validity/generation metadata. |
| A-022 | BLOCK | CODE FIXED | Connected Direct HID can be reported as `Found=true` before any real input sample exists | Direct HID is not a valid live sample until a real report is received. |
| A-023 | WARN | CODE FIXED | Pedal calibration is keyed to wheel/mode, not to the actual input layout/source used to learn it | Pedal calibration is bound to wheel/session/input source/layout. |
| A-024 | BLOCK | CODE FIXED | `AxisValues()` has unsigned-underflow normalization that can turn below-minimum input into 100% | Axis normalization uses signed arithmetic/clamping; below-min input cannot wrap to 100%. |
| A-025 | WARN | CODE FIXED | Steering calibration accepts impossible geometry and arbitrarily tiny travel | Steering calibration validates geometry and minimum usable travel. |
| A-026 | WARN | CODE FIXED | Classic G25/DFGT `Buttons` mask includes encoded D-pad/payload bits as if they were logical buttons | G25/DFGT D-pad payload bits are masked from semantic button state. |
| A-027 | WARN | CODE FIXED | G25/DFGT malformed Direct-HID reports are silently dropped while the previous state remains apparently usable | Malformed classic reports are surfaced and do not masquerade as fresh valid state. |
| A-028 | NOT TESTABLE WITHOUT HARDWARE | EXTERNAL GATE | G25 H-shifter decoding is threshold-based and not physically certified | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-029 | WARN | CODE FIXED | WinMM input telemetry is process-global instead of per selected controller/session | WinMM telemetry/error state is tied to the selected controller/session rather than one process-global truth. |
| A-030 | WARN | CODE FIXED | Input profile read failures silently degrade to empty calibration/mappings | Critical input JSON uses strict corruption-preserving reads/writes. |
| A-031 | BLOCK | CODE FIXED | FFB session hand-off can time out while the old hardware writer is still alive | Timed-out/unconfirmed FFB worker keeps global ownership and recovery state; a replacement writer cannot start. |
| A-032 | WARN | CODE FIXED | Motor-output start APIs report success before HID startup has actually succeeded | Motor start performs a real initial HID write before reporting success. |
| A-033 | BLOCK | CODE FIXED | Crash-recovery marker creation errors are ignored for active motor sessions | Recovery marker creation is mandatory before the first motor command. |
| A-034 | BLOCK | CODE FIXED | Release can clear the recovery marker without proving the output worker stopped | Marker/lease clear only after confirmed neutralization; failures remain fail-closed. |
| A-035 | WARN | CODE FIXED | Native engine profile application is not fully transactional | Engine profile apply is stop-first, hardware-first, persist-after-success with rollback attempt on persistence failure. |
| A-036 | PASS | PASS / RETAINED | Effect builders, slot allocation and mixer safety caps are well bounded | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-037 | WARN | EXTERNAL GATE | Synchronous HID writes have no hard I/O timeout or cancellation primitive | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-038 | WARN | CODE FIXED | Native FFB/profile JSON corruption silently falls back to defaults | Native FFB/profile JSON uses strict corruption-preserving storage. |
| A-039 | NOT TESTABLE WITHOUT HARDWARE | EXTERNAL GATE | Native output protocol still requires physical model certification | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-040 | WARN | D0 ARCHITECTURE DEBT | Range, LEDs, autocenter and FFB still bypass one authoritative output command queue | The global lease serializes production output safely; a single cancellable command queue/transport remains D0 architecture work. |
| A-041 | BLOCK | CODE FIXED | Startup recovery clears the pending-output marker even when emergency neutralization fails | Startup recovery clears its marker only after exact target correlation and successful neutralization. |
| A-042 | BLOCK | CODE FIXED | Pending-output recovery is one-shot and is not retried when prerequisites are initially unavailable | Pending recovery is retried when safe/current device state becomes available. |
| A-043 | BLOCK | CODE FIXED | A panic inside `WM_DESTROY` can skip the remainder of motor shutdown | Lifecycle shutdown isolates/reports stop failures so one recovered panic cannot silently skip all hardware safety handling. |
| A-044 | BLOCK | CODE FIXED | Windows session-end/power-shutdown messages do not have an explicit output-safety path | Session-end, end-session and power/suspend messages have explicit output safety handling. |
| A-045 | BLOCK | CODE FIXED | PnP topology changes ignore emergency-stop failure and then release output state | PnP/device-change stops output fail-closed before identity refresh; failed stop no longer means bookkeeping release. |
| A-046 | PASS | PASS / RETAINED | Device-change handling proactively stops output before refreshing identity | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-047 | WARN | CODE FIXED | Runtime output recovery marker lacks a strong target/session fingerprint | Recovery marker now binds wheel ID, current session, model and strong hardware fingerprint when available. |
| A-048 | PASS | PASS / RETAINED | Native FFB has conservative gain/slew/watchdog limits and OpenG27 coexistence checks | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-049 | WARN | CODE FIXED | State invariants do not detect all possible LogiMate-internal dual-output ownership | Runtime invariants cover FFB/output-without-lease, dual OpenG27/native ownership and lease-target mismatch. |
| A-050 | NOT TESTABLE WITHOUT HARDWARE | EXTERNAL GATE | Kill/crash/USB-yank/suspend stuck-force behavior is not physically certified | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-051 | BLOCK | CODE FIXED | Mode migrations can commit without verifying the requested final device state | Modern/Legacy migration verifies the requested terminal device/driver state before commit. |
| A-052 | BLOCK | CODE FIXED | Migration journal durability errors are ignored after journal creation | Migration journal writes are mandatory; durability failure aborts progress. |
| A-053 | BLOCK | CODE FIXED | Rollback is best-effort and has no verified terminal “rollback succeeded” postcondition | Rollback has its own terminal-state verification. |
| A-054 | WARN | CODE FIXED | Operating-mode preference write failure is only a warning even though migration returns success | Operating-mode preference persistence failure is fatal to migration completion. |
| A-055 | WARN | CODE FIXED | HVCI/Memory Integrity handling models a registry request, not the effective security state | HVCI reports configured, effective and restart-required state separately. |
| A-056 | PASS | PASS / RETAINED | Fresh Legacy install verifies the official Logitech installer signer before execution | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-057 | PASS | PASS / RETAINED | Driver backup restore verifies recorded SHA-256 hashes | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-058 | PASS | PASS / RETAINED | Profiler backup/restore has integrity and bounded-path checks | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-059 | WARN | CODE FIXED | Old migration cleanup can delete unresolved transaction journals solely by age | Unresolved migration journals are retained/discovered rather than deleted by age. |
| A-060 | NOT TESTABLE WITHOUT HARDWARE | EXTERNAL GATE | Modern↔Legacy/HVCI rollback remains a real-Windows hardware gate | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-061 | BLOCK | CODE FIXED | A telemetry-LED-only game profile can still drive the wheel motor | LED-only telemetry does not create a motor lease/scheduler. |
| A-062 | BLOCK | CODE FIXED | Pino telemetry `FFBEnabled=false` is ignored by the Fusion C6 motor source | Pino FFBEnabled, Physics and PlayerControl gate all non-zero game force. |
| A-063 | WARN | CODE FIXED | Automatic telemetry startup errors are swallowed by game-session logic | Automatic telemetry startup failure is surfaced into the game session and stops C6 output. |
| A-064 | WARN | CODE FIXED | Background game-profile selection is nondeterministic when multiple matching games run | Background profile choice is deterministic. |
| A-065 | WARN | CODE FIXED | Corrupt game-profile storage silently becomes an empty/default profile set | Game-profile JSON uses strict corruption-preserving storage. |
| A-066 | WARN | CODE FIXED | Pino telemetry packet diagnostics never increment `Packets` | Pino packet diagnostics increment actual packet counts. |
| A-067 | PASS | PASS / RETAINED | Telemetry listeners are loopback-only and parsing is bounded | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-068 | PASS | PASS / RETAINED | Telemetry freshness plus Physics/PlayerControl gates are present | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-069 | PASS | PASS / RETAINED | Game-profile import is size-bounded and normalized before persistence | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-070 | WARN | CODE FIXED | Telemetry “adapter registry” is metadata, not yet an adapter lifecycle interface | Telemetry adapters now have explicit start/stop lifecycle ownership rather than metadata-only registration. |
| A-071 | WARN | CODE FIXED | UI claims an exclusive output gate that C7 does not actually enforce | UI output-ownership wording now matches an actually enforced central lease. |
| A-072 | BLOCK | CODE FIXED | Turning Experimental Native Output off can release bookkeeping without guaranteed neutralization | Turning Native Output off succeeds only after confirmed emergency stop/release. |
| A-073 | WARN | CODE FIXED | Experimental motor-output opt-in is a one-click settings toggle with no durable risk acknowledgement | Experimental motor output requires durable version-bound risk acknowledgement. |
| A-074 | WARN | CODE FIXED | Corrupt UI settings silently reset individual/all values to defaults | UI settings corruption is surfaced/preserved and normal overwrite is blocked. |
| A-075 | WARN | CODE FIXED | Obsolete `AutoStartOpenG27` remains in the settings schema after the C7 cutover | Obsolete AutoStartOpenG27 was removed from the current settings schema. |
| A-076 | WARN | CODE FIXED | Setup can report completion even when its completion preference failed to persist | Setup completion is not reported/persisted when completion storage fails. |
| A-077 | PASS | PASS / RETAINED | Destructive setup flow fails closed on ambiguous/no/multiple wheel selection | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-078 | PASS | PASS / RETAINED | Setup window cannot be casually closed while a tracked migration is running | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-079 | PASS | PASS / RETAINED | Destructive mode changes have explicit user plan/confirmation before elevation | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-080 | NOT TESTABLE WITHOUT HARDWARE | EXTERNAL GATE | End-to-end Windows UI/accessibility/hardware workflow still requires external validation | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-081 | BLOCK | CODE FIXED | There is no OS-wide single-instance/output-owner lock for LogiMate itself | Global GUI singleton and global native-output mutex prevent competing LogiMate processes. |
| A-082 | WARN | CODE FIXED | Atomic file writes use a fixed `.tmp` name and are not safe under concurrent writers | Atomic writes use unique sibling temp files and write-through replacement. |
| A-083 | WARN | CODE FIXED | Updater verifies integrity but not independent publisher authenticity | Automatic update apply requires valid Authenticode on current/candidate binaries from the same publisher certificate in addition to SHA-256. |
| A-084 | PASS | PASS / RETAINED | ZIP extraction has traversal and decompression-size defenses | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-085 | WARN | CODE FIXED | Native updater helper accepts arbitrary apply paths from command-line arguments | Update helper consumes a short-lived one-shot authorization manifest instead of arbitrary apply paths. |
| A-086 | WARN | CODE FIXED | Update/uninstall lifecycle is not coordinated with another running LogiMate/output owner | Update and uninstall coordinate with singleton/output-recovery state; destructive replacement is blocked while unsafe. |
| A-087 | WARN | CODE FIXED | Diagnostic exports redact paths but still expose detailed device identifiers/topology | Diagnostic exports redact/hash wheel/session/device identifiers rather than exposing raw topology IDs. |
| A-088 | PASS | PASS / RETAINED | Privileged driver/HVCI operations are explicitly UAC-gated rather than silently elevated | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-089 | WARN | CODE FIXED | External OpenG27 ownership detection is executable-name based and can miss renamed instances | Native output HID handles deny FILE_SHARE_WRITE; explicit OpenG27 interlock remains as an additional guard. |
| A-090 | NOT TESTABLE WITHOUT HARDWARE | EXTERNAL GATE | Windows race/handle-leak stress and real HID concurrency remain unexecuted gates | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-091 | WARN | CODE FIXED | Main-branch CI artifacts are stamped with stale version `0.0.3-alpha` | VERSION is the single build version source; app/installer/CI no longer stamp the stale value. |
| A-092 | WARN | EXTERNAL GATE | Stable release signing is explicitly missing | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-093 | PASS | PASS / RETAINED | Tag release workflow already has a solid software build/test/package baseline | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-094 | WARN | CODE FIXED | Local `build.ps1` and CI release packaging are not equivalent | build.ps1, CI and tag-release packaging now use the same core artifact layout/version rules. |
| A-095 | NOT TESTABLE WITHOUT HARDWARE | EXTERNAL GATE | Required G25/G27/DFGT hardware matrix is documented but not an enforceable release artifact | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-096 | NOT TESTABLE WITHOUT HARDWARE | EXTERNAL GATE | Crash-recovery/no-stuck-force certification has no automated or recorded release gate | Cannot be truthfully certified without real Windows/hardware/signing evidence; Stable pipeline is blocked until evidence is recorded. |
| A-097 | WARN | CODE FIXED | Incomplete migration journals are not discovered/recovered on normal application startup | Normal startup discovers and handles unresolved migration journals. |
| A-098 | WARN | CODE FIXED | Data schema accepts unknown newer versions instead of blocking downgrade writes | Unknown newer data schema versions block downgrade writes. |
| A-099 | PASS | PASS / RETAINED | Audited C7 release package is internally checksum-consistent and archives are structurally readable | C7 behavior retained; rechecked as part of the C8 source/build audit. |
| A-100 | BLOCK | ALPHA READY / STABLE GATED | C7 is not ready for a stable/native-motor release until C8 blockers and external gates are closed | All C7 software BLOCK findings are closed for the C8 alpha. Stable remains blocked by the explicit external certification/signing gates. |

## Known limitation retained intentionally

**A-037:** the current classic Logitech output transport still uses synchronous Windows HID calls. A Go-side timeout cannot safely cancel a kernel I/O call and could create a late writer. C8 therefore uses the safer rule: if completion cannot be proven, ownership/recovery remains locked and no replacement motor writer may start. A true overlapped/cancellable transport belongs in D0/D4 and real HID stress remains a Stable gate.

## Stable release gates

Stable requires every field in `HARDWARE_CERTIFICATION.json` to be true with evidence, plus valid Authenticode signatures. The C8 alpha intentionally ships those evidence fields as false. See `HARDWARE_CERTIFICATION.md`.

## Next planned engineering stage

**D0 — Native Engine Foundation:** decompose the remaining system monolith, replace remaining stringly model/mode boundaries, converge all hardware commands on a single cancellable transport/queue, and formalize WheelManager/DeviceRegistry/Input/Output/Safety/Profile/Adapter interfaces before broader hardware promotion.

## Final packaging verification

The final source passed hardware-independent OpenG27 tests, Windows/amd64 vet, Windows system/app test compilation, x64 application build, installer vet/build, ARM64 compile validation and workflow/JSON parsing. The release root carries `PACKAGE_VERIFICATION.txt` plus `SHA256SUMS.txt` after artifact packaging. Windows runtime/hardware and Authenticode evidence remain external gates.
