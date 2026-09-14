# D5.3 G27 Native Activation Recovery Report

**Release:** 0.5.3-alpha  
**Scope:** real-hardware G27 C294 detection → C29B activation

## Evidence

The user's D5.2 Direct HID diagnostics shows one supported Logitech interface:

- PID: C294
- Product: G27 Racing Wheel
- Input report length: 8
- Output report length: 8
- Usage: 0001/0004

The main UI already classifies the wheel as Logitech G27 but remains in Generic HID/C294. This proves the remaining issue is not HID visibility; it is the automatic native-mode promotion/rebind path.

## Root causes fixed

1. `maybePrepareRememberedModernWheel` previously required `OperatingPreference == modern`. On first run or after a reset, the preference can be empty even when the wheel is unambiguously confirmed and Generic HID is the active mode. That prevented the automatic C294→C29B transition.
2. LogiMate's generic Logitech switch helper sent a prelude report before the EXT_CMD9 native switch. OpenG27's proven G27 path sends only `F8 09 04 01 00 00 00` and then waits for re-enumeration. D5.3 uses that exact G27 sequence.

## New policy

- Exactly one supported/actionable wheel is required.
- Explicit Legacy preference always blocks auto-promotion.
- If no preference exists and Windows is already in Legacy mode, no automatic write occurs.
- Otherwise, an actionable C294 G27/G25/DFGT in Generic HID may be promoted to its native PID.
- G27 uses the exact OpenG27 single-report switch sequence.
- G25/DFGT retain the previous sequence pending physical certification.
- After the write, LogiMate polls direct HID discovery for the expected native PID and binds the native reader when it appears.

## Physical acceptance test

For the user's G27:

1. Power-cycle/replug so it appears as C294.
2. Start LogiMate 0.5.3-alpha with OpenG27/LGS closed.
3. Direct HID may initially show C294 / G27 Racing Wheel.
4. Within a few seconds, the wheel should re-enumerate as C29B.
5. The main Wheel page should show native G27/Direct HID live input.
6. If it remains C294, capture the exact `Automatischer Native-Mode-Switch fehlgeschlagen:` message plus Direct HID diagnostics.

Stable remains blocked until this physical transition and subsequent input/FFB certification pass.
