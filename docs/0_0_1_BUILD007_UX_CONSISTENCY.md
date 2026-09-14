# LogiMate 0.0.1-alpha · Build 007 — About & UX Consistency

## Goal

Finish the post-audit UX foundation without changing the native wheel/output architecture.

## Implemented

- About page is product-first: LogiMate hero, version/build, purpose, developer, Open Source/Credits, privacy, Alpha status, projects and support.
- Removed stale roadmap wording such as old C8→D0 next-step copy.
- Removed duplicate About action clutter; the page and bottom actions no longer compete with repeated links.
- Non-live Wheel tabs are not invalidated by the 250 ms telemetry timer.
- FFB `Live & Test` repaints only its live monitor card.
- Inline Calibration repaints only its live rows.
- Live Wheel remains double-buffered and invalidates only the content surface rather than sidebar/header/footer.
- Public version remains `0.0.1-alpha`; internal identity advances to `Build 007`.

## UX result

The product now has one clear wheel information architecture, one global diagnostics center, one resumable setup flow and a stable visible version while hardware validation continues.
