# LogiMate 0.0.1-alpha · Build 007 — Final Post-Rework Audit

**Visible version:** `0.0.1-alpha`  
**Internal revision:** `Build 007`  
**Engine baseline:** D0–D6.2 native architecture retained  
**Audit scope:** Builds 002–007, code/logic/UX consistency, packaging and remaining release gates

## Executive result

The planned UI/UX sequence is complete without introducing a second wheel engine, HID writer or motor-output path. The largest product-level issue after Build 001 was not another protocol rewrite; it was that calibration, profiles, diagnostics and setup were distributed across catch-all dialogs. Build 007 establishes the information architecture needed for continued real-hardware validation.

## Completed sequence

| Build | Scope | Result |
|---|---|---|
| 002 | Wheel navigation | PASS — five fixed wheel tabs, catch-all button removed from normal workflow |
| 003 | Calibration 2.0 | PASS — inline state machines, valid-sample/session checks, atomic final persistence |
| 004 | Profiles 2.0 | PASS — Wheel/Game/D4 precedence and effective result visible |
| 005 | Diagnostics 2.0 | PASS — centralized issues/tests/log/export/release views |
| 006 | Setup 2.0 | PASS — resumable eight-step flow; migration no longer equals setup completion |
| 007 | About + UX consistency | PASS — structured About and targeted live invalidation |

## Code/logic findings closed in this sequence

1. **Overloaded Wheel action menu:** normal workflow no longer hides unrelated calibration/profile/diagnostic tools in one command dialog.
2. **Modal calibration chains:** new primary calibration path is inline and non-modal.
3. **Calibration transaction truth:** data is committed only after a complete valid capture; pedal source/session/layout continuity is checked.
4. **Gain-precedence ambiguity:** Profiles shows effective master/constant chains and the hard Safety cap.
5. **Effective Settings zero-value bug:** an intentional D4 Constant `0 %` is displayed as mute rather than defaulting to `100 %`.
6. **Fragmented diagnostics:** runtime errors are normalized into one problem list and UI warnings/errors are retained in a session event log.
7. **Setup false-completion risk:** driver/mode migration advances the wizard but does not mark the entire onboarding complete.
8. **Setup stale-state risk:** the setup window receives fresh application state after device scans.
9. **Unnecessary live repaint:** Profile/Device/static Calibration pages no longer repaint at 4 Hz; only true live surfaces do.
10. **Version drift:** visible version remains pinned and the source/build defaults now identify Build 007.

## Architecture invariants preserved

- one central OutputLease/motor owner;
- one bounded native HID output transport;
- emergency neutralization remains separate from normal acquire validation;
- D4 shaping remains before hard `NativeFFBConfig` safety limits;
- productive runtime does not import `internal/openg27port`;
- Legacy Logitech mode remains optional and separate from Modern;
- persistence schema downgrade/corruption protection from Build 001 remains active.

## Remaining P0 — real G27 evidence

These items cannot be truthfully closed by cross-build/static tests:

- cold/replug automatic C294→C29B without manual model confirmation;
- steering/pedal/button/H-shifter capture on the physical G27;
- 270/360/540/720/900° range behavior;
- Constant/Spring/Damper/Friction/Autocenter at low test strength;
- gain 0/25/50/75/100 behavior and D4 shaping one control at a time;
- Emergency Stop from every active effect;
- USB yank/reconnect while output owns the wheel;
- process-kill recovery and suspend/resume;
- Modern→Legacy→Modern rollback/recovery.

## Remaining global release gates

- real G25 matrix;
- real G27 matrix;
- real DFGT matrix;
- Narrator/UI Automation, High Contrast and mixed-DPI evidence;
- D5.9 physical HID stress/adverse-I/O evidence;
- Authenticode publisher signing for Stable.

## Recommended next phase

Do **not** add another broad UI layer first. Use Build 007 as the stable Alpha UX baseline and run the G27 P0 matrix. Any real-wheel discrepancy should be fixed narrowly in its owning adapter/transport/calibration layer. After G27 is green, repeat the capability matrix on G25 and DFGT, then close Accessibility/HID-stress/signing gates.

The visible version remains `0.0.1-alpha` throughout this validation unless the project explicitly decides otherwise.
