# LogiMate 0.0.8-alpha — UX review

> **Superseded note (0.0.9):** the 0.0.8 full empty-state decision was intentionally revised after hardware-page feedback. The current Wheel page keeps instruments visible and places detection/selection guidance above them.

This review focuses on whether a first-time user can understand state, choose a physical wheel, test it, recover from ambiguity and make driver changes without accidental destructive actions.

## Core user journey

1. **Overview answers “what should I do next?”** The first card and primary command are state-driven.
2. **Wheel answers “is my hardware really working?”** A real empty state replaces fake zero-value instruments until a target exists.
3. **Manage devices answers “which wheel are we talking about?”** Selection, rescan and detection reset live in one dialog.
4. **System answers “what can safely be changed?”** Every blocked mode transition has a textual reason; backup is emphasized only when it is actually needed.
5. **Diagnostics answers “what do I send when something is wrong?”** Export/copy/folder remain one-click actions and raw state retains selection/detection details.
6. **Settings answers “how should LogiMate behave?”** Appearance, wheel behavior, updates and startup remain grouped and get more vertical room.
7. **About answers “who made this and what else exists?”** Markus Kleine, related projects, upstream credits and voluntary support are separated from operational controls.

## Multi-device UX invariants

- A persisted physical wheel is never silently replaced by another device.
- Selection is safe for diagnostics; driver-package changes still require exactly one supported attached wheel.
- Resetting detection does not erase driver backups, OpenG27, operating preference or pedal calibration.
- C294 identity and pedal mappings stay per physical device.
- Missing target, no hardware, scan failure and ambiguous model are distinct user-visible states.

## Visual hierarchy decisions

- Overview, Wheel and System retain the four-tile hardware status strip because it helps decisions there.
- Diagnostics, Settings and About start directly below the page header to avoid redundant hardware chrome.
- Primary emphasis moves with the safe workflow; a disabled action is never intentionally left as the only visually-primary command.
- Backup and HVCI colors are contextual, and important states are always accompanied by text.

## Trust / external actions

- External project/support actions are fixed HTTPS URLs and open through the default Windows browser.
- PayPal is described as voluntary project support, not a tax-deductible charitable donation.
- OpenG27 remains explicitly credited as upstream work by Jabelius.
- LogiMate has no own usage telemetry; network activity is limited to the configured update/download workflows in the current source.

## Still required before a serious 1.0 claim

- Authenticode publisher signing / SmartScreen reputation.
- Full physical regression matrix on real G27 and G25 hardware across Legacy/Generic HID, HVCI on/off, pedals and shifter.
- Microsoft UI Automation providers for complete screen-reader semantics of the custom-painted controls.
- Full resource-backed German/English localization.
- Broader physical validation before advertising more Logitech wheel models as stable.

These remaining items are deliberately not hidden by the UI polish: compile validation cannot replace hardware, accessibility-provider or publisher-certificate validation.
