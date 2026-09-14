# LogiMate Post-D6.2 Master Audit & Roadmap

**Baseline:** 0.6.2-alpha — D6.2 FFB Verification / Control-Panel Rework  
**Purpose:** define what is complete, what is physically unproven, and the shortest path from the current Alpha to a trustworthy Stable/1.0 release.

## 1. Current architecture status

The foundational architecture work C8/D0–D5 remains the correct baseline: one native wheel engine, one selected-device/session identity model, one central OutputLease, fail-closed recovery, one productive game FFB scheduler, and no external OpenG27 runtime dependency. D6 adds a user-facing control plane; it must not become a second engine.

D6.2 specifically establishes a stronger rule: every visible FFB setting must be classifiable as **live-effective**, **hardware-transactional**, **condition-only**, **test-only**, or **unavailable**. A setting is not allowed to look active merely because a value can be persisted.

## 2. Current risk ranking

### P0 — must be proven/fixed before any Beta claim

1. **Real G27 FFB matrix.** Verify Constant, Spring, Damper, Friction, Autocenter, STOP and 270/360/540/720/900° on the actual wheel. Capture requested/applied/status evidence for each.
2. **Game FFB effective-gain chain.** Verify 0/25/50/75/100% at Wheel Master, Wheel Constant and D4 Game Constant while a supported telemetry game is producing force. Confirm 0 really mutes and each level is monotonic until the hard safety cap clips it.
3. **D4 shaping behavior.** Verify Deadband, Minimum Force, Response Curve, Low-Pass, Smoothing and Pre-Safety Limit against live telemetry and the monitor. Each should change the expected stage and never bypass the hard safety mixer.
4. **USB/reconnect during output.** Yank/reconnect during a bounded test and during Game FFB; no stale writer may resume and unresolved recovery must block new output.
5. **C294 -> native activation.** Automatic G27 activation must succeed cold/replug without manual model confirmation before `native-mode-switch` can pass.
6. **Modern -> Legacy -> Modern rollback.** Driver/profile migration must end in the expected mode and not strand the wheel or an output lease.

### P1 — required before Stable/1.0

7. Repeat the required hardware/effect matrix on real G25 and DFGT.
8. Finish D5.8 Narrator/UIA, complete keyboard navigation, High Contrast, reduced motion and mixed-DPI validation.
9. Finish D5.9 100-cycle HID stress and real USB-yank/adverse-I/O evidence on each required model.
10. Validate process-kill, suspend/resume and Windows session-end recovery with an active bounded output.
11. Review every certification export and promote repository flags only from real evidence.
12. Build Stable on the trusted Windows signing host and verify Authenticode, hashes and `RELEASE_ATTESTATION.json` independently.

### P2 — product-quality improvements after the P0 path is green

13. Consolidate the overlapping **Wheel profile / Game profile / D4 shaping** concepts into a single visible “effective settings stack” so users always know which layer supplies each value.
14. Add model-capability-driven hiding/disable states instead of showing controls that a G25/DFGT/G27 cannot support.
15. Add optional FFB history graph/export for diagnosis, but keep it out of the 250 ms core paint path.
16. Add named user presets with import/export and a safe “Reset to model defaults”.
17. Add per-game overrides only after the effective-setting precedence model is explicit and tested.
18. Revisit hard Alpha motor caps only from reviewed physical evidence; never raise them because a UI slider has a larger range.

## 3. Recommended Wheel-page tab structure

The most coherent long-term layout is:

| Wheel-page tab | Purpose | Recommendation |
|---|---|---|
| **Live-Test** | live steering/pedals/buttons/shifter + advanced raw data | Keep |
| **Force Feedback** | D6.2 Basis/Effekte/Signalformung/Live & Test | Keep |
| **Profile** | active Wheel profile, Game profile, D4 preset, precedence, per-game overrides, import/export | **Best next tab** |
| **Kalibrierung** | steering/pedal calibration, button/shifter learning, reset | Add after Profile; moves wheel-specific learning closer to the wheel |
| **Diagnose** | PID/mode/HID path, transport health, report counters, certification evidence, copy/export | Add after calibration; keep advanced/raw details reachable without cluttering normal Live-Test |

RPM LEDs should stay a capability-specific subsection (Profile or Diagnose), not a universal top-level tab, because only supported models should see it.

## 4. Recommended next development block

Do **not** add another motor engine or another writer. The highest-value next software block is a **Profile / Effective Settings** layer:

- show active Wheel profile, active Game profile and D4 preset together;
- calculate/display the effective pre-safety gain chain;
- expose exactly which layer owns a value and why;
- allow a game-specific override without silently modifying global wheel settings;
- show “live”, “next session”, “hardware write required” and “not supported” status per row;
- include reset/diff/import/export;
- preserve the central D6.2 live-refresh API and OutputLease.

This solves the largest remaining usability problem: multiple correct gain layers can currently multiply each other without being obvious to the user.

## 5. Suggested real-G27 D6.2 test order

1. Start with Native Output off; confirm every setting persists after restart without motor movement.
2. Enable Native Output; HID preflight must report ready/protected or ready/shared with no unknown writer conflict.
3. Test rotation 270 -> 540 -> 900 and STOP/neutral.
4. Set low Test Strength and test Constant, Spring, Damper, Friction, Autocenter separately; verify monitor switches from Game telemetry to Hardware-Test mode and shows requested/applied values.
5. For Autocenter confirm a real centering spring now appears after the new SpringEnable sequence and disappears after STOP/watchdog.
6. Start supported Game FFB and vary Wheel Master and Wheel Constant downward first; Tuning Revision must increase without restarting FFB.
7. Set D4 Game Constant to 0; output must become zero while telemetry Raw still moves. Restore to 100.
8. Test Deadband/Minimum Force/Response/LPF/Smoothing/Pre-Safety Limit one at a time and watch Raw -> Shaped -> Applied.
9. Verify Spring/Damper/Friction do not falsely claim to change a Constant-only game source; test those gains through their dedicated hardware tests instead.
10. Run Emergency Stop, USB-yank/reconnect and then the D5.9 stress workflow.

## 6. Beta / Stable definition from here

A reasonable **Beta** threshold is: G27 P0 matrix green, automatic native activation green, reconnect/recovery green, D6.2 no-flicker UI confirmed, and no known fail-open motor/output defect. G25/DFGT can remain explicitly uncertified only if the product does not claim them Stable-supported.

A **Stable/1.0** claim for all three currently advertised classic wheels requires reviewed G25 + G27 + DFGT physical evidence, D5.8 accessibility, D5.9 adverse I/O, crash/suspend/migration recovery and D5.10 signed-release evidence. Compilation or synthetic tests cannot substitute for those gates.
