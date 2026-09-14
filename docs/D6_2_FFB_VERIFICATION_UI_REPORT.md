# D6.2 FFB Verification & Control-Panel Rework

**Release:** 0.6.2-alpha  
**Scope:** verify every user-visible D6 FFB control against the actual runtime path, remove misleading controls, close known manual-test protocol gaps, and stop live UI refreshes from repainting the complete window.

## Executive result

D6.0/D6.1 had a valid control-panel foundation, but the audit found several reasons why a user could move a control and perceive no change:

1. Running Game FFB captured the Native Engine profile and D4 tuning at session start. The UI persisted new values, but the live session kept the old copy until restart.
2. D4 `ConstantGain=0` / `TransientGain=0` was normalized back to `1.0`, so zero did not actually mute the channel.
3. `Transient Gain` was presented as active even though no currently registered game adapter publishes a semantic Transient force channel.
4. Spring/Damper/Friction are real condition-effect gains, but the current telemetry game path publishes semantic Constant Force only. Those controls therefore cannot change a pure Constant telemetry stream.
5. The manual Autocenter test sent the FE/0D spring-set report but omitted the required lg4ff/OpenG27 SpringEnable report.
6. The manual Constant Force test still used the historical slot-style packet even though productive Game FFB already uses the canonical wheelengine/OpenG27 constant-force packet.
7. The 250 ms input/live timer invalidated much more UI than required, producing visible redraw/flicker risk.
8. Manual changes could mark both the Native Engine profile and the D4 preset as `Custom`, even when only one layer was edited, obscuring which setting owned the change.

D6.2 closes the software-provable parts of those findings. Real motor feel and model-specific behavior remain physical certification items.

## FFB setting audit

| Control | Persistence | Running Game FFB | Hardware/live-test effect | D6.2 status |
|---|---|---|---|---|
| Native Output | UI/system setting | Gate only | Required for direct hardware commands/tests | VERIFIED gate; no force is sent by enabling it |
| FFB Pipeline | Per-wheel D4 config | **Live** via runtime refresh | Does not alter bounded direct hardware tests | FIXED live apply |
| Overall / Master | Per-wheel Native Engine profile | **Live** | Scales Constant + all condition effects before hard safety caps | FIXED live apply |
| Constant Force gain | Per-wheel Native Engine profile | **Live** on current telemetry Constant path | Scales Constant live test | VERIFIED; manual test now canonical packet |
| Spring gain | Per-wheel Native Engine profile | Live if a future/current source requests Spring | Scales Spring condition test | VERIFIED semantics; current telemetry does not generate Spring |
| Damper gain | Per-wheel Native Engine profile | Live if a source requests Damper | Scales Damper condition test | VERIFIED semantics; current telemetry does not generate Damper |
| Friction gain | Per-wheel Native Engine profile | Live if a source requests Friction | Scales Friction condition test | VERIFIED semantics; current telemetry does not generate Friction |
| Rotation | Per-wheel Native Engine profile | Not a force-shaping setting | Written transactionally only with authorized Native Output | VERIFIED; otherwise saved/pending |
| Game Constant Gain | Per-wheel D4 config | **Live** | Game telemetry path only | FIXED; newly exposed and 0% now truly mutes |
| Transient Gain | Per-wheel D4 config | No active source channel today | None | Correctly disabled in UI until an adapter publishes Transient |
| Deadband | Per-wheel D4 config | **Live** | Game telemetry path only | VERIFIED in pure pipeline tests |
| Minimum Force | Per-wheel D4 config | **Live** | Game telemetry path only | VERIFIED; remains below Output Limit + hard safety mixer |
| Response Curve | Per-wheel D4 config | **Live** | Game telemetry path only | VERIFIED in pure pipeline tests |
| Low-Pass | Per-wheel D4 config | **Live** | Game telemetry path only | VERIFIED; 0 = off, otherwise bounded 5–100 Hz |
| Smoothing | Per-wheel D4 config | **Live** | Game telemetry path only | VERIFIED |
| Pre-Safety Limit | Per-wheel D4 config | **Live** | Game telemetry path only | VERIFIED; cannot raise physical safety caps |
| Test Strength | Session-only | None | Bounded direct test input only | VERIFIED |
| Autocenter test | Session-only | None | SpringSet + **SpringEnable**, watchdog then SpringOff | FIXED protocol sequence |
| Emergency Stop | Recovery/safety path | Stops active output | Global stop + neutralization/lease recovery | HARD safety path retained |

## Why a slider can still appear to have little effect

The final motor value is deliberately layered:

`Telemetry force × Game Profile gains × D4 shaping × Wheel profile gains × NativeFFBConfig hard safety caps`

The current Alpha hard limits are intentionally conservative (for example Constant Force is capped independently of user-facing gains). When the signal is already clipping at a hard safety ceiling, raising an upstream slider cannot increase the physical output further. D6.2 exposes clipping/runtime state rather than pretending every gain is unlimited.

The active Game Profile also owns `GameFFBGainPercent`, `MasterGainPercent` and `ConstantGainPercent`. D6.2 surfaces those active values in the FFB runtime header so a hidden game-profile gain can no longer silently explain unexpectedly weak/strong behavior.

## Runtime live-tuning fix

`RefreshNativeGameOutputTuning` now updates the already-running per-wheel engine profile and D4 pipeline instead of waiting for the game session to restart. A monotonic tuning revision is shown in the panel. Rotation remains a separate hardware transaction and never piggybacks on a gain edit.

## Protocol fixes

### Manual Constant Force

Productive Game FFB already used `wheelengine.ConstantForce`, which is parity-tested against the attributed OpenG27/lg4ff reference. D6.2 moves the manual Constant live test onto that same canonical report. The historical slot builder remains only for historical diagnostics/tests.

Stopping the Constant live test now sends the canonical global `StopAllEffects` command followed by an explicit neutral Constant Force report before the output lease is completed.

### Autocenter

The previous live test only sent SpringSet (`FE 0D ...`). D6.2 follows it with SpringEnable (`14 00 ...`), matching the retained OpenG27 reference. Emergency neutralization uses SpringOff (`F5 00 ...`) and now also includes the global stop command.

## Control-panel structure

The Force Feedback page is split into four in-page views instead of one long mixed list:

- **Basis** — Wheel preset, overall gain, rotation and effective-state/safety summary.
- **Effekte** — Constant, Spring, Damper and Friction with explicit scope badges.
- **Signalformung** — D4 game-telemetry shaping and presets; unavailable Transient is visibly disabled.
- **Live & Test** — runtime force monitor, HID state, bounded hardware tests and STOP.

A manual change only marks the owning layer as `Custom`: wheel gains dirty the Wheel profile; D4 shaping dirties the D4 preset. The unrelated layer remains unchanged.

## Flicker / live-rendering changes

The 250 ms wheel timer no longer invalidates the whole top-level window while the Force Feedback panel is open. On `Live & Test`, only the live monitor rectangle is invalidated. On the other three FFB views, the live timer performs no visual repaint at all.

The Win32 renderer now honors `PAINTSTRUCT.rcPaint`, clips drawing to that dirty rectangle and blits only the changed region from the existing double buffer. Accessibility overlay HWNDs are moved only when geometry actually changes, and live accessibility status publication on the Wheel page is throttled to one update per second.

This removes the main sources of visual flicker without slowing the actual hardware/input polling.

## Software verification

- host `wheelengine` tests: PASS;
- host `gameadapter` tests: PASS;
- Windows amd64 System test compile: PASS;
- Windows amd64 App test compile: PASS;
- Windows amd64 `go vet` for System/App/LogiMate: PASS;
- D4 zero-gain behavior covered by regression test;
- D4 schema-v1 zero-value migration covered by Windows regression tests;
- D6.2 control ownership/focus/unavailable-Transient behavior covered by Windows regression tests;
- canonical Constant Force packet and Autocenter Enable/Off packet bytes covered by Windows regression tests;
- physical G27/G25/DFGT force behavior: PENDING.

## Physical truth boundary

Do not mark any effect as hardware-certified from this software audit alone. On the user's real G27 the next test must verify, at low strength, Constant → Spring → Damper → Friction → Autocenter, while watching requested/applied values and checking that every STOP returns the wheel to neutral. The same model matrix eventually has to be repeated for G25 and DFGT.
