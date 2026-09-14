# D5 Modern-mode Independence Report — 0.5.0-alpha

## Goal

Make the supported Modern workflow for Logitech G25, G27 and Driving Force GT fully owned by LogiMate so no second wheel application is required.

## Result

**Software-side D5 independence: PASS.**

The production executable now owns:

- device discovery and stable/session identity;
- C294 model confirmation and native-mode transition;
- Direct HID / WinMM input and calibration;
- wheel capabilities and model adapters;
- game/telemetry adapters and profiles;
- native protocol generation and Advanced FFB;
- output lease, crash recovery and HID transport;
- Modern↔Legacy migration and rollback.

## OpenG27 boundary after D5

OpenG27 is **not a runtime dependency**. LogiMate does not search for an OpenG27 executable, does not download/install it, does not launch it and does not poll its process state to decide ownership.

`internal/openg27port` remains in the source tree only for attributed historical reference/parity tests and provenance. The production dependency graph of `cmd/logimate` is explicitly gated so that package cannot be linked into the shipped runtime.

Old OpenG27 `game-profiles.json` files remain supported as an **offline legacy import format**. They are parsed by LogiMate-owned compatibility structs; importing them never starts motor output and does not make OpenG27 a runtime requirement.

## Logitech Legacy boundary

Logitech LGS/Profiler remains supported only when the user deliberately selects **Original Logitech / Legacy**. It is a compatibility mode, not a hidden dependency of Modern mode.

## D5 code changes

- removed OpenG27 executable path/state from `system.State`;
- removed OpenG27 discovery/path persistence and GitHub downloader/installer;
- removed OpenG27 launch/fallback actions from normal UI;
- removed named OpenG27 process checks from native output, FFB and game-output workers;
- renamed the last native input defaults/calibration helpers so they are LogiMate-owned and deterministic;
- removed live reads of `%APPDATA%\OpenG27\config.json`;
- removed productive imports of `internal/openg27port` from system/game-profile code;
- retained old adapter/profile IDs only as read-compatible migrations;
- added D5 tests and a `go list -deps` build gate to prevent runtime dependency regression;
- updated Engine Health and diagnostics to describe the standalone Native Engine.

## What D5 does not claim

D5 proves software independence, not physical Stable certification. Real hardware is still required to certify:

- G25/G27/DFGT complete input layouts;
- every advertised motor effect and rotation range;
- C294↔native transitions and reconnect behavior;
- USB removal, process kill, suspend/resume and stuck-force recovery;
- Modern↔Legacy↔Modern rollback;
- HID stress behavior on real Windows machines;
- final accessibility and Authenticode Stable gates.

These remain explicit release gates rather than artificial PASS results.
