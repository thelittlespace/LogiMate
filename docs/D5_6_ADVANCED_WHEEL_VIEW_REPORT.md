# D5.6 — Advanced Wheel View

## Goal
Keep all wheel diagnostics on the existing **Lenkrad** page. No new navigation destination or modal raw-data tool is required for normal inspection.

## UI
The Wheel page now has a top-right **Erweiterte Ansicht** toggle. When enabled, the normal LogiMate wheel dashboard remains visible and the page expands downward with live diagnostic cards in the same visual language.

The expanded view includes:

- physical/logical wheel identity, session/container, PnP state, model confirmation, instance/interface/alias IDs, service/INF, persistent identity and fingerprint;
- live input source/layout, sample validity/generation/time, report rate/age, reconnects and errors;
- X/Y/Z/R/U/V raw values, min/max and normalized values plus steering/pedal calibration;
- complete 32-bit button mask and individual button states;
- semantic paddles, wheel buttons, shifter buttons, D-pad and gear;
- every Direct-HID report byte as index + hex + decimal + binary;
- direct HID candidate metadata including VID/PID, product, serial, report lengths, usage and path;
- Native Output / FFB / lease / C294→native activation state;
- pedal mapping/calibration and current detection errors.

## Architecture
HID candidate capabilities are captured during the existing background discovery scan and stored as a read-only `State.HIDCandidates` snapshot. The paint path never opens or probes HID devices. This preserves UI responsiveness while still exposing the same class of low-level information that made OpenG27 useful during hardware debugging.

The old `Rohdatenmodus` entry under **Kalibrieren & Lernen** now toggles the exact same expanded Wheel-page view. There is one source of UI truth instead of two raw-data surfaces.

## Safety
The expanded view is read-only. Opening or scrolling it sends no FFB, range, LED, native-mode or driver command.
