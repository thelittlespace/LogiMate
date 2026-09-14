# LogiMate 0.0.1-alpha · Build 013 — Responsive / DPI / Visual QA

## Implemented

- High-DPI minimum window size is capped to the physical desktop instead of becoming larger than the screen.
- Narrow windows automatically use the compact sidebar.
- Content rectangles no longer invent a 420 px width outside the real client area.
- Scroll offsets are clamped after resize/DPI changes.
- Added regression tests for 96-DPI preferred size, 200%-DPI small-screen behavior and compact content geometry.
- Existing Build 008 theme contrast/fitted-text safeguards remain active.

## Manual gate still required

Real Windows rasterization must still be visually checked at 100/125/150/200%, Dark/Gray/Light/System/High Contrast. Software tests may not mark that physical UI matrix as certified.
