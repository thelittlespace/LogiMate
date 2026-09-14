# D5.5 Release Readiness — 0.5.5-alpha

## Software status

- Duplicate SetupAPI/HID devnode evidence enrichment: PASS (code + regression compile gate)
- Explicit HID product evidence reaches automatic current-session model confirmation: PASS (code + regression compile gate)
- Existing D5.4 OpenG27-compatible G27 switch path retained: PASS
- Production runtime remains independent from `internal/openg27port`: PASS
- Windows x64 system/app compile gates: PASS before packaging

## Remaining physical gate

The user's real G27 must confirm that **manual model confirmation is no longer required**. Expected cold-start sequence: C294 → automatic session confirmation from `G27 Racing Wheel` → automatic switch → C29B.
