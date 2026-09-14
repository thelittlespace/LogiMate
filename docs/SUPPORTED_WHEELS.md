# Supported wheels — LogiMate 0.0.1-alpha

LogiMate uses one capability-driven native wheel engine for G25, G27 and Driving Force GT. **Implemented in software** and **physically validated** are deliberately separate claims.

| Wheel | Native PID | Direct input | Native range/FFB | Extra controls | Physical validation status |
|---|---|---|---|---|---|
| Logitech G25 | C299 | Implemented | Implemented / alpha | clutch, H-shifter, sequential mode | Real-hardware validation pending |
| Logitech G27 | C29B | Implemented | Implemented / alpha | clutch, H-shifter, rev LEDs | LED output and FFB live tests confirmed; full matrix pending |
| Logitech Driving Force GT | C29A | Implemented | Implemented / alpha | sequential shifter; no clutch/rev LEDs | Real-hardware validation pending |

## Shared C294 compatibility mode

`VID_046D / PID_C294` is ambiguous. C294 stays non-actionable until the physical model is proven by an authoritative native PID or sufficiently strong current-session evidence. USB topology alone is not model identity.

Model-specific native selectors are centralized in `internal/wheelengine`:

- G25 → C299 / selector `0x02`
- Driving Force GT → C29A / selector `0x03`
- G27 → C29B / selector `0x04`

A C294 G25/G27 may become session-confirmed when direct HID product evidence or independent PnP/WinMM evidence agrees. DFGT C294 remains manual/native-PID-only until C29A is observed. Remembered native identity may be reused only with matching strong device/session evidence; USB location alone is not sufficient.

## Modern mode

The normal Modern path requires no external wheel application. LogiMate handles detection, migration, Direct HID/WinMM fallback, calibration, profiles, output ownership, FFB, supported game telemetry and model-specific extras itself.

The old Logitech LGS/Profiler stack is required only when the user deliberately chooses **Original Logitech / Legacy**. External OpenG27 is not a Modern runtime dependency and is not discovered, installed or launched by LogiMate. Attributed reference/parity code remains in source/tests for provenance.

Memory Integrity/HVCI is read-only from LogiMate's perspective. If Legacy is incompatible while it is active, setup fails closed and can open the relevant Windows Security page; LogiMate does not toggle that Windows security setting.

## Multiple wheels and ownership

Device selection is fail-closed. Native output remains one physical target/one owner at a time. A selected device/session mismatch invalidates runtime state and blocks writes. Input diagnostics may enumerate multiple wheels, but productive output never creates a second motor writer.

## Physical validation still required

Before Stable, each model must be tested on real hardware for:

- C294↔native transition
- steering/pedals/buttons/shifter layout
- supported rotation ranges and every advertised force effect
- STOP/watchdog/recovery behavior
- reconnect and repeated output start/stop
- process kill, USB removal and power-state transitions
- Modern↔Legacy migration/rollback
- long HID stress where applicable

Software paths for G25 and DFGT must not be described as physically certified until real-device evidence exists.
