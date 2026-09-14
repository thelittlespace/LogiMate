# LogiMate 0.1.0-alpha design review

## Goal
Rebuild the complete LogiMate shell around the selected modern profile-hub reference while preserving all wheel/driver safety logic.

## Implemented
- Stable wide Acrylic-friendly sidebar with compact opt-out.
- Bottom-aligned Settings destination.
- Dedicated dashboards for Overview, System, Diagnostics and About.
- Existing Wheel instruments preserved instead of replaced.
- Multi-wheel warning/selection panel retained above live instruments.
- Profile hub for Markus Kleine with project, current-work, interest and support sections.
- Native Indicana project overlay with all major projects.
- Project overlay can be closed with Escape; navigation remains usable.
- Larger default window and safer minimum geometry.

## Deliberate deviations from the concept image
- No fabricated portrait is embedded. The UI uses an `MK` avatar until Markus explicitly supplies a photo suitable for the app.
- No fake search box is shown because there is not yet a real global search/command palette behind it.
- No decorative mountain photography is embedded into the executable; the design relies on Windows Acrylic and native cards rather than shipping unrelated stock imagery.

## Validation still requiring real Windows hardware
- DWM Acrylic appearance on Windows 11 with 100%, 125%, 150% and mixed-DPI monitors.
- Real G27/G25 input, multi-device selection and hotplug behavior.
- Keyboard/tab traversal across every new page at runtime.
- Visual checks in Dark, Gray, Light, System, High Contrast and Safe UI.
