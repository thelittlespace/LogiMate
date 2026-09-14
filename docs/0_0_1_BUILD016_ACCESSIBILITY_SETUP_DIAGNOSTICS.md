# LogiMate 0.0.1-alpha · Build 016 — Accessibility / Setup / Diagnostics

## Accessibility

- Fixed a real UI Automation bug: native overlays existed for all five Wheel tabs, but the command handler accepted only Live + Force Feedback. Calibration, Profiles and Device now activate correctly through UIA.
- Added native UIA button providers and keyboard focus IDs for Calibration, Profiles and Device actions.
- Added native UIA providers and keyboard navigation for Diagnostics tabs/actions.
- Keyboard focus now reaches the actionable controls inside the redesigned Wheel and Diagnostics pages instead of stopping at the tab strip.

## Setup

- The dashboard now surfaces a paused first-run guide with its exact resume step.
- “Später erinnern” remains a real pause, not completion.

## Diagnostics

- Lifecycle warnings/errors from Build 014 remain in the session event stream.
- Hardware certification progress from Build 015 is visible in Tests.

## Manual validation still required

Narrator, Accessibility Insights, High Contrast, keyboard-only navigation and 100/125/150/200% mixed-DPI testing remain real Windows acceptance tests. They are not automatically marked PASS by this build.
