# LogiMate 0.0.1-alpha · Build 006 — First-run Setup 2.0

## Goal

Replace the short text-heavy setup flow with a progressive workflow that does not claim completion before the wheel can actually be used.

## Implemented

Eight steps:

1. wheel detection,
2. operating mode,
3. safety/readiness checks,
4. Modern/Legacy setup execution,
5. live input validation,
6. calibration follow-up,
7. low-risk FFB follow-up,
8. final summary.

- Successful driver/mode migration advances to input validation instead of immediately setting setup complete.
- `Später erinnern` remains a real snooze: setup stays incomplete and resume step is persisted.
- Setup can open the new Calibration or Force Feedback tab directly and remembers where to resume.
- Enter/keyboard primary action follows the same migration boundary as mouse input; arrow navigation cannot silently skip the destructive setup step.
- While the setup window is open, fresh main-state scans are synchronized into the setup view.
