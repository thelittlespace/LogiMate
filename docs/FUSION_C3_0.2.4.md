# OpenG27 Fusion C3 — LogiMate 0.2.4-alpha

## Goal

Port OpenG27's FFB scheduling/output lifecycle into native Go without surrendering LogiMate's stricter ownership and safety model. C3 is the first phase allowed to route translated OpenG27 bytes to real hardware, but only through an explicit, short, G27-only validation path.

## Ported upstream behavior

- `IFfbSource` lifecycle (`Start`, `Stop`, `TryGetFrame`).
- `FfbEngine.PumpOnce`: source frame -> ForceMixer -> WheelOutput.SetForce -> WheelOutput.Tick.
- Single scheduler owner: a second `Start` is a no-op.
- Upstream parity scheduler cadence: integer `1000/150` -> 6 ms.
- `WheelOutput` clamp to normalized [-1,+1].
- Per-tick slew limiting.
- Source heartbeat watchdog that writes neutral if updates stop.
- `Panic`: global stop plus guaranteed convergence/final neutral force write.

## LogiMate hardening layered above the port

- Existing StableWheelID/PnP/model-confirmation gate stays authoritative.
- Live C3 is G27-only during Path C.
- Explicit Experimental Native Output opt-in still required.
- OpenG27 must be stopped and is rechecked during the live session.
- C3 shares LogiMate's generation owner with the existing FFB engine.
- Device change, target reset, app shutdown and Emergency Stop cancel C3 through the same owner channel.
- Existing Safety Profile remains authoritative.
- Existing slew policy is specified as percent/20ms, so C3 converts it to an equivalent 6-ms budget instead of reusing the numeric value and accidentally accelerating force ramping.
- Live C3 Constant Force is capped to ±5%.
- Every live C3 test hard-stops after 900 ms even while valid source frames continue.
- Runtime crash marker is active for the C3 hardware session and cleared only after the scheduler performs its stop/center handoff.

## User-visible validation

Under `Kalibrieren & Lernen -> Native Wheel Engine -> FFB-Effekttests`:

- Existing `Constant Force · LogiMate Legacy Engine` remains available.
- New `Fusion C3 Scheduler · G27` offers ±2% and ±5%.
- This is deliberate A/B testing; C3 does not silently replace the previous engine.

`Engine Health`, `FFB Engine Status` and diagnostic reports expose:

- scheduler generation/owner,
- pumps,
- source frames,
- HID writes,
- requested/applied percent,
- slew-limit count,
- source-watchdog trips,
- last heartbeat,
- stop reason and last error.

## Dry run

`FusionC3DryRun` opens no HID handle. It verifies:

1. a 0.5 source frame with 0.5 master gain resolves to the expected OpenG27 constant-force byte for 0.25 normalized torque;
2. the scheduler counters advance;
3. removing source frames and advancing deterministic time beyond the watchdog causes a neutral `0x80` force write;
4. Stop/Panic leaves the final report neutral.

## Intentionally deferred to C4/D

- Replacing all existing LogiMate live FFB routes with C3.
- G25/DFGT live routing through translated OpenG27 scheduler semantics.
- Jitter/latency optimization beyond upstream parity.
- Exact 150 Hz/adaptive scheduling instead of the parity 6-ms cadence.
- G27 input/report parser consolidation (C4).
