# OpenG27 Fusion C1 — LogiMate 0.2.1-alpha

## Goal

Start Path C without changing real wheel output. C1 ports only hardware-independent OpenG27 Core behavior into a separate Go package and proves parity with automated golden vectors.

## Included

- Force slider -> lg4ff force-byte scaling.
- ForceFrame/ForceMixer numeric behavior.
- Centered steering-axis calibration including asymmetric and inverted endpoints.
- Pedal calibration including inverted axes, independent deadzone and sensitivity/gamma.
- RangeTracker min/max capture.
- Cross-platform Go unit tests.
- LogiMate internal self-test hooks.
- Full MIT notice and per-file provenance ledger.

## Deliberately NOT included

- No new HID writes.
- No replacement of LogiMate's current FFB engine.
- No replacement of current LogiMate calibration persistence.
- No WPF/OpenG27 UI code.
- No OpenG27 runtime dependency added.

## Test on Windows

1. Start LogiMate 0.2.1-alpha normally and verify the same wheel detection/input behavior as 0.2.0.
2. Open **Kalibrieren & Lernen** and run the existing internal self-tests.
3. Confirm these new checks are PASS:
   - OpenG27 C1 force scaling parity
   - OpenG27 C1 steering calibration parity
   - OpenG27 C1 pedal calibration parity
   - OpenG27 C1 range tracker parity
4. Confirm existing Direct HID, device selection and Experimental Native Output behavior did not change.

## Next checkpoint

C2 ports OpenG27 `Lg4ffReports` into the isolated parity package and compares its output byte-for-byte with LogiMate's existing native report builders in Shadow/Dry-Run mode. No translated report builder will control hardware until parity is explicit.
