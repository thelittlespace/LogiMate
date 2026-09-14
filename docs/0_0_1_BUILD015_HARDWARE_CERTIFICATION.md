# LogiMate 0.0.1-alpha · Build 015 — Hardware Certification

## Real G27 evidence carried forward

Build 010/011 hardware feedback confirms on a real Logitech G27:

- RPM LED output ping reaches the wheel.
- Constant Force responds.
- Spring responds.
- Damper responds while steering.
- Friction responds while steering.
- Autocenter responds.

This is recorded as evidence, **not** as complete model certification. Reconnect, USB removal, process-kill recovery, suspend/resume, Modern↔Legacy rollback, full input/calibration/range and game-FFB evidence remain open.

## Build 015 changes

- Guided five-effect FFB certification suite added.
- Certification motor tests use an explicit 15% diagnostic request.
- Errors while persisting certification results are no longer silently ignored.
- Diagnostics shows live certification progress for the currently selected wheel.
- Repository certification manifest contains the confirmed G27 FFB/LED evidence while all Stable flags remain false.
