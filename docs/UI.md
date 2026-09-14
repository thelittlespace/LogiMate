# LogiMate UI system — 0.1.0 profile hub

LogiMate uses a native Win32/GDI interface styled as a modern Windows 11 profile/dashboard application. The selected design language is a stable translucent sidebar, large rounded cards, strong page hierarchy and page-specific dashboard composition.

## Navigation

- Default width: 228 logical px; the rail remains wide instead of expanding on hover.
- Optional compact mode: 82 logical px, selected explicitly in Settings.
- Main destinations: Startseite, Lenkrad, System, Diagnose, Über mich.
- Einstellungen is visually separated at the bottom.
- Acrylic shows through the rail when supported; Safe UI and High Contrast stay opaque.

## Page layouts

- Startseite: recommendation hero, four state cards, quick access and system health.
- Lenkrad: global status band, persistent live instruments, optional multi-device/detection panel above the instruments.
- System: dedicated cards for driver backup, operating mode, Windows security and Wheel Engine.
- Diagnose: detection/renderer/Windows metrics plus support workflow.
- Einstellungen: theme cards and grouped toggles.
- Über mich: profile hero + quick facts, biography + current work, projects + favorite areas + support, plus the Indicana project overlay.

## Project hub

The profile page has one Indicana project tile. Opening it displays a native in-app panel containing LogiMate, Indicana Tools, StromPilot, Green-ITea and TheLittleCyclist. This avoids squeezing every project into the primary profile grid while keeping all of them one click away.

---

LogiMate uses a compact, icon-first navigation rail inspired by modern app builders while remaining a native Win32 application.

## Navigation

Six user-facing categories form the primary navigation:

1. **Overview** — current wheel, driver mode, HVCI, backup health and safe next-step guidance. No driver switching lives on the landing page.
2. **Wheel** — LogiMate Direct-HID diagnostics for a confirmed native G27 (OpenG27 optional), WinMM fallback diagnostics, Windows Game Controllers test, G25/G27/Driving-Force-GT identity and model-appropriate profile/engine access.
3. **System** — legacy driver packages, backup/restore, Generic HID and OpenG27 installation/update.
4. **Diagnostics** — support report, privacy-aware ZIP export and clipboard copy.
5. **Settings** — Dark/Gray/Light/System appearance, Windows material/transparency, motion, sidebar behavior, auto-refresh, OpenG27 launch behavior, Windows autostart, updates and LogiMate data/recovery tools.
6. **About** — Markus Kleine, the broader project family, Open-Source credits, GitHub project links and optional PayPal support.

The rail is 76 px wide at rest and expands to 224 px while hovered. Icons remain available in the compact state; labels appear while the rail expands. A compact hover label is painted next to the active item before the full label has room to render.

## Motion

- The main window is shown immediately; startup deliberately avoids a blocking fade.
- Sidebar width uses a short eased 16 ms animation with edge hysteresis to avoid hover oscillation.
- Page changes keep the small horizontal slide, now rendered into an off-screen back buffer before one final blit.
- Motion is deliberately short and functional; it does not delay actions.

## Backdrop

- Windows 11 22H2+ uses a theme-adaptive documented system backdrop: Dark = Desktop Acrylic (`DWMSBT_TRANSIENTWINDOW`), Gray = Mica Alt (`DWMSBT_TABBEDWINDOW`), Light = Mica (`DWMSBT_MAINWINDOW`).
- Windows 10 / early Windows 11 use the stable opaque fallback matching the selected/effective palette; the undocumented legacy composition path is intentionally not used.
- If composition is unavailable, LogiMate paints the selected theme as a normal opaque background.

Cards use contrast surfaces appropriate to the selected theme. The material itself also follows the theme so Gray/Light are not painted over a Dark Acrylic sheet.

## Typography and iconography

- Segoe UI Variable Display/Text are requested for Windows 11.
- Windows automatically substitutes Segoe UI on systems without the variable font.
- Navigation glyphs use Segoe MDL2 Assets for Windows 10/11 compatibility.


## Premium visual system

- **Dark** preserves the original deep Acrylic identity.
- **Gray** uses a graphite surface system with slightly brighter text and calmer contrast.
- **Light** switches both application surfaces and DWM title-bar chrome to a light Fluent palette.
- Theme selection is a four-card segmented control at the top of Settings and persists in `settings.json`.
- Segoe UI Variable remains the single UI type family; Regular is used for body copy and Semibold for page/section hierarchy.
- Overview, System and Diagnostics render their text as scannable information cards instead of one continuous text block.
- The Wheel page uses live instruments: steering dial, pedal/axis meters, a button matrix and POV compass. For a confirmed G27, LogiMate Direct HID is preferred even without OpenG27; steering, pedals, wheel buttons and H-shifter controls are decoded semantically. WinMM remains fallback for legacy/generic controller paths.
- Informational/non-destructive dialogs prefer the native Windows TaskDialog API when available; destructive prompts keep the existing safe default-button behavior.

## Accessibility principles

- Text remains present for all important actions; icons are not the only meaning carrier.
- The active navigation item has both a background state and an accent marker.
- Status cards use text plus color rather than color alone.
- Destructive/compatibility-sensitive actions keep explicit confirmation dialogs.
- D5.8 gives the custom-painted main window native Windows accessibility peers: navigation, actions, theme choices, Settings toggles and Wheel Advanced View expose standard roles/names/focus/checked state through UI Automation/MSAA.
- Accessibility peers are transparent to pointer hit testing and invoke the same app actions as the painted UI; they are not a second state model.
- A native status surface exposes current page title/subtitle/body context to screen readers. Full Stable validation still requires Narrator/UIA, High Contrast, reduced-motion, mixed-DPI and secondary-dialog checks on Windows.


## Rendering stability

- Every frame is rendered into a reusable compatible back buffer and copied to the window in one operation.
- The extended glass client surface is explicitly cleared before each frame so old animated text cannot remain in the DWM redirection surface.
- Text uses grayscale antialiasing on the translucent surface rather than ClearType subpixel rendering.
- Per-monitor DPI awareness v2 is requested where Windows supports it, with the previous DPI fallback retained.
- Action buttons are no longer hidden/shown during every sidebar animation frame.
- Common GDI brushes and pens are cached instead of recreated repeatedly during motion.


## Was ist neu / updates

The first launch of a version opens a separate rounded Windows-material changelog window after startup is marked stable. The main Settings page exposes update toggles and the current updater status. Automatic checks/downloads are background-only and never run in the window-creation path.

## Fluent command buttons

- Main page actions are painted directly into the same reusable back buffer as the page surface.
- No `BUTTON` child HWNDs, `WM_DRAWITEM`, owner-draw repaint, hide/show cycle or per-frame `MoveWindow` calls are used for main actions anymore.
- Primary actions use an accent-tinted command card with a blue icon chip; neutral actions use the standard panel surface; compatibility-sensitive/destructive actions use the existing warm warning palette.
- Hover and pressed states alter only the next buffered frame, so text and button chrome cannot lag behind a page-slide animation.
- Mouse capture keeps press/release semantics correct if the pointer leaves the button before release.
- Button chrome, content and animation share one buffered frame; no native child-button redraw is required.

## Placement rules

- **Overview never changes drivers.** It explains status and routes the user to the correct specialist page.
- **Wheel changes no Windows driver state.** It is for input, model identity, Windows `joy.cpl` and wheel/profile interaction.
- **System owns privileged/driver operations.** Backup is visually primary; switching/restoring remains explicit and confirmed.
- **Diagnostics owns export/support actions.**
- **Settings owns LogiMate itself**, not wheel-driver state.
- **About owns identity, project links, credits and voluntary support**, never device state or privileged actions.

This separation is deliberate so a first-time user cannot mistake a low-level driver switch for a normal setup button.

## Theme integrity

The three appearance modes are intentionally different products, not brightness variants:

- **Dark** — deep blue-black surfaces + Desktop Acrylic, vivid blue action accent and cool contrast.
- **Gray** — neutral graphite/silver surfaces + Mica Alt, desaturated steel accent and much less blue bias.
- **Light** — cool light canvas + Mica, white elevated surfaces, dark typography and light Windows chrome.
- **System** — follows Windows Light/Dark and therefore resolves to the matching Dark/Light material; Gray remains explicit.

Interaction roles (hover, pressed, selected, toggle-off, toggle-knob, badge, info-chip and on-accent foreground) are defined per theme. Code should not branch on `theme == light` to choose hardcoded black/white button text; use the theme role instead.

Theme switching resets the previous DWM backdrop before applying the new chrome/material. This avoids stale Dark Acrylic behind Mica/Light surfaces during rapid theme changes.


## Guided setup, accessibility and System appearance

- **System** follows Windows Light/Dark while Gray remains an explicit LogiMate mode.
- The optional first-run setup offer appears only after the main UI and first state refresh are stable. Skipping it disables future automatic offers until the user re-enables the Settings toggle.
- The three-page guide uses the same visual language as “Was ist neu?”, includes a local Windows-user greeting, a graphical 2×2 model chooser (Automatic / Driving Force GT / G25 / G27), then a graphical Modern/OpenG27 vs Original/Legacy choice, and finally a full execution plan.
- The first two pages are non-destructive. The final page can explicitly start the reviewed migration plan; UAC and a second confirmation remain required before driver changes.
- Custom controls support visible keyboard focus, Tab/Shift+Tab, Enter/Space and arrow navigation.
- High Contrast replaces the custom palette with Windows system colors, and disabled client animations suppress LogiMate motion.
- DPI changes rebuild font metrics and use Windows' suggested window rectangle; a complete logical-unit geometry engine remains future polish.
- Learned pedal roles annotate the graphical axis meters without hard-coding driver-dependent axis identities.

## 0.0.8 task-oriented guidance

- Overview puts **Recommended next step** first and mirrors it in the primary command button. The recommendation is derived from actual state rather than a static shortcut.
- Wheel exposes one stable **Manage devices** entry point for selection, rescan and detection reset. A missing previously-selected wheel is explained rather than silently substituted.
- System leads with **Safety status** so disabled driver actions always have a textual reason.
- Settings, Diagnostics and About start directly below the page header; the four-tile wheel status band is reserved for Overview, Wheel and System where it has decision value.
- Backup and HVCI indicators are contextual. No backup is neutral when no legacy package exists; HVCI is a warning only when it conflicts with a Legacy workflow.


## 0.0.9 Acrylic rail and persistent Wheel dashboard

- With Windows material enabled, the navigation rail deliberately leaves its base unpainted so Desktop Acrylic/Mica is visible again. Selected, hover and focus surfaces are still painted for contrast.
- Safe UI, High Contrast and material-disabled rendering keep the opaque rail.
- The Wheel page always retains steering/axis/button/shifter instruments. Detection and selection problems are shown in a compact attention panel above the instruments.
- Multi-wheel attention never implies that destructive driver operations are safe; those remain blocked until exactly one supported physical wheel is attached.
## D5.7 Hardware Certification Assistant

The certification workflow is reached from **Lenkrad → Kalibrieren & Lernen → Hardware Certification Assistant**. It is intentionally not a new navigation page. Progress is shown in **Lenkrad → Erweiterte Ansicht** next to the raw HID/input/output truth so users can correlate a PASS/FAIL result with the actual live state.


## D6 Force Feedback control panel

The `Lenkrad` page now owns two in-page modes directly below its heading: `Live-Test` and `Force Feedback`. The main navigation is unchanged. `Live-Test` retains the existing graphical steering/pedal/button/shifter dashboard and Advanced View. `Force Feedback` presents the Native Engine and D4 controls as direct visual sliders, preset chips, a live force pipeline monitor and bounded effect-test controls.

The FFB panel follows the same custom-painted Fluent surface as the rest of LogiMate. Mouse dragging and keyboard arrow adjustment share one configuration path. D5.8 transparent native accessibility peers cover the D6 tabs, toggle, presets, sliders and test controls so the additional surface is included in the eventual Narrator/UIA validation matrix.
