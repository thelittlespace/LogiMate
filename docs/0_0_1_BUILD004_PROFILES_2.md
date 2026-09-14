# LogiMate 0.0.1-alpha · Build 004 — Profiles 2.0

## Goal

Make gain precedence understandable instead of forcing the user to mentally combine Wheel profile, game profile, D4 signal shaping and hard motor limits.

## Implemented

- Dedicated Wheel/Profile tab.
- Current per-wheel Native Engine profile card.
- Current auto-applied game profile card.
- `Effektive Einstellungen` card computes Wheel Master × Game Master and the Constant chain through the D4 pipeline.
- Hard `NativeFFBConfig` cap is shown after the user-facing gains.
- D4 preset/deadband/minimum-force/smoothing state is shown alongside the effective result.
- Intentional D4 Constant Gain `0 %` remains `0 %` in the effective-settings display.
- Existing profile management and JSON export remain available without duplicating profile storage.
