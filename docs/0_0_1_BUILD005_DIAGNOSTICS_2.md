# LogiMate 0.0.1-alpha · Build 005 — Diagnostics 2.0

## Goal

Turn Diagnose into the single place for current problems, test entry points, session errors and support export.

## Implemented

Six views:

1. `Übersicht` — active error/warning counts, HID transport, input/FFB/release state.
2. `Probleme` — normalized issues from discovery, driver inventory, Profiler, HVCI, Raw Input, Wheel input, Native FFB, Native Output and HID transport.
3. `Tests` — Engine Health, HID Stress, Hardware Certification and status refresh.
4. `Protokoll` — up to 200 UI error/warning events captured from the common notice path.
5. `Export` — diagnostics ZIP, text report and diagnostics folder.
6. `Release` — current Authenticode/publisher/thumbprint and Stable signing requirement.

## Design rule

A transient UI error is not lost after its dialog closes: warning/error notices are also recorded in the current diagnostic session timeline.
