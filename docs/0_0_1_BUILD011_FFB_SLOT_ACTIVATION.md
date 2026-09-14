# LogiMate 0.0.1-alpha · Build 011 — FFB Slot Activation & Adjustable Test Strength

## Hardware evidence entering this build

The real G27 flashes its RPM LEDs when Build 010 runs `Output prüfen`. The unified HID transport is therefore physically confirmed. Only the old Spring live test produced a brief motor response; Constant, Damper, Friction and Autocenter were not perceptible.

## Root causes fixed

- Build 009/010 condition helpers did not use the real new-lg4ff slot map. Damper and Friction were effectively sharing Spring slot 1.
- Constant used the simplified direct packet instead of the slot-0 start/update/stop lifecycle used by new-lg4ff.
- Autocenter wrote the UI ramp value (`2`) into the FE/0D clip byte, producing an almost zero saturation.
- The D6 slider was capped at 10% and its number was then scaled through Wheel/Game gains, so it was not an honest hardware-test strength.
- The old 1200 ms timeout made a valid response very brief.

## Build 011 behavior

| Test | Protocol | Slot | Start / Update / Stop |
|---|---|---:|---|
| Constant | lg4ff slot | 0 | 0x11 / 0x1C / 0x13 |
| Spring | lg4ff condition | 1 | 0x21 / 0x2C / 0x23 |
| Damper | lg4ff condition | 2 | 0x41 / 0x4C / 0x43 |
| Friction | lg4ff condition | 3 | 0x81 / 0x8C / 0x83 |
| Autocenter | FE/0D + enable | special | clip 0x80, SpringOff neutralization |

Every manual test sends fixed-loop mode off (`0x0D/0`) before slot activation, remains on the Build-010 unified G27 HID session and is bounded to three seconds. STOP/Emergency Neutralize remains authoritative.

## Strength semantics

The Force Feedback → Live & Test slider now spans **1–30%** and defaults to **15%**. This is a direct diagnostic request. Wheel profile and Game profile gains do not scale a manual hardware test. Persistent/game FFB still uses the existing downstream safety mixer and its stricter caps.

## Physical test notes

Damper and Friction are velocity/resistance effects: move the wheel during their three-second test window. At rest, little or no torque is expected.
