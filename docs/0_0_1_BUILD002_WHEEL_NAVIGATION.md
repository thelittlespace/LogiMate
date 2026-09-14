# LogiMate 0.0.1-alpha · Build 002 — Wheel Navigation Rework

## Goal

Remove the overloaded `Kalibrieren & Lernen` entry point from the normal Wheel workflow and make wheel-owned functions discoverable in one stable information architecture.

## Implemented

- Wheel page now owns five fixed in-page tabs: `Live`, `Force Feedback`, `Kalibrierung`, `Profile`, `Gerät`.
- Existing Live and D6.2 FFB surfaces remain in place; no second main navigation item or modal control center was created.
- Calibration, profile and device actions are routed through the same existing persistence/HID owners.
- Wheel page top actions no longer expose the old catch-all learning menu.
- UI Automation overlay exposes all five Wheel tabs as native accessible controls.

## Safety boundary

This build changes navigation only. It does not add a HID writer, motor scheduler or output ownership path.
