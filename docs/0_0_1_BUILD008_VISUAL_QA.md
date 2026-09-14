# LogiMate 0.0.1-alpha · Build 008 — Visual QA / Contrast / Layout

**Visible version:** `0.0.1-alpha`  
**Internal revision:** `Build 008`

## Scope

Static project-wide review of the current custom Win32 renderer for clipped text, weak foreground/background contrast, fixed-height dynamic text, selected-state readability and short-window scrolling. This pass deliberately does not modify the native wheel/FFB ownership model.

## High-impact findings fixed

1. Warning text reused warning-surface colors; in Light mode the old warning foreground was effectively pale-on-pale. Foreground and surface are now separate roles.
2. Gray/Dark secondary text was below the 4.5:1 target on normal panel surfaces; `muted2` was raised.
3. Several selected chips/tabs used accent text on an accent-soft surface with <4.5:1 contrast; selected text now uses `readableTextColor`.
4. Fixed brand/project accents (purple/pink/orange) can be unreadable in Light/Gray; decorative border color is preserved while text is blended toward the theme text color until readable.
5. Multiple tinted surfaces used `blendColor` with the blend percentage in the wrong visual direction, producing near-accent backgrounds with same-color headings. Those panels now use subtle surfaces.
6. `modernCard` painted its accent stripe before the opaque card surface and therefore hid the stripe. Paint order is corrected.
7. `paintInfoRows` assumed two lines even when callers provided only ~14 px/row. It now switches to compact key/value rows when height is constrained.
8. Settings rows were only 52 px and descriptions were one-line; rows/groups are taller and descriptions use fitted multi-line text.
9. Wheel Calibration/Profile/Device and the inline calibration wizard can now scroll instead of losing lower controls in shorter clients.
10. Diagnostics Problem/Test/Log rows now allocate real detail space and calculate row count from available height.
11. FFB long notes/status/runtime text, About hero text, setup status and changelog/sidebar/page headings gained fitted/ellipsis handling.

## Automated regression gates

`build008_visual_qa_windows_test.go` checks all semantic foreground colors (`text`, `muted`, `muted2`, `accent`, `warning`, `good`, `bad`) against Background/Panel/Panel2 for Dark, Gray and Light at a minimum 4.5:1 contrast, plus `onAccent` against Accent. It also protects the minimum Settings row/group space needed for title + description.

## Remaining physical validation

The code-level audit cannot replace screenshots from actual Windows font rasterization. Before calling UI accessibility complete, validate Dark/Gray/Light at 100%, 125%, 150% and 200% scaling, Windows High Contrast, Narrator/UI Automation and mixed-DPI monitor movement. Any remaining clipping should be captured with page, scaling, theme and screenshot so it can be tied to a specific geometry owner.
