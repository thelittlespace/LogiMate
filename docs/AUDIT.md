# LogiMate product audit — 0.2.0-alpha

> **C7 / 0.2.8-alpha current state:** the G27 Modern path now defaults to **LogiMate Native**. External OpenG27 is an explicit fallback only. Historical audit lines below that describe OpenG27 as the normal production FFB/profile runtime are retained as chronology, not current architecture. Real C3/C6 G27 motor/LED/reconnect certification remains open.

This audit distinguishes features implemented in code from work that still requires external hardware, certificates or deeper platform integration.

## Implemented in 0.0.3

- Driving Force GT (C29A) detection plus conservative C294 fallback selection.
- Read-only G27 Direct-HID input built into LogiMate; external OpenG27 is optional fallback; read-only input coexistence is possible, but motor output remains single-owner. WinMM remains fallback.
- Embedded LogiMate multi-resolution icon used across the native UI.

- Optional three-page setup guide in the same Fluent/window-material style as “Was ist neu?”. Model/mode selection is non-destructive; the final reviewed page can explicitly launch Modern/Legacy migration behind confirmation + UAC. The startup offer can be skipped, disabled, re-enabled and the guide can always be opened manually.
- Four appearance choices: Dark, Gray, Light and System. System follows the Windows app theme; Gray remains a dedicated LogiMate theme.
- Keyboard focus traversal for the custom-painted navigation/actions/settings/theme choices, Enter/Space activation and arrow navigation.
- Visible focus rings plus Windows High Contrast and reduced-motion awareness.
- `WM_DPICHANGED` handling, DPI-scaled fonts/minimum window metrics and Windows suggested-rect handling. Some page geometry still uses the established base-coordinate layout rather than a fully resource-driven layout engine.
- Pedal-learning workflow with Gas/Brake/Clutch mappings persisted separately per wheel model and Legacy/Generic-HID mode.
- Native SetupAPI as the primary present-device enumeration path; PowerShell/WMI is compatibility fallback only.
- Raw Input Logitech-wheel enumeration as a second identity signal. SetupAPI/PnP native PID remains strongest; Raw Input native PID can disambiguate a PnP C294 path. For a confirmed G27, LogiMate Direct HID is preferred for live values even without OpenG27; WinMM remains fallback. external OpenG27 can remain installed as fallback; C7 keeps motor output single-owner and LogiMate Native is the normal Modern runtime.
- Conservative multi-controller WinMM selection: explicit model-consistent G25/G27/Driving-Force-GT names may win; broad Driving Force/Racing Wheel names are correlated only when they are the sole WinMM controller and PnP/Raw Input already confirmed a supported wheel.
- Theme-adaptive Windows material: Dark uses Desktop Acrylic, Gray uses Mica Alt and Light uses Mica; solid-theme fallbacks remain available through the material toggle/Safe UI.
- Page actions provide visible in-app success/progress feedback or an explicit error path instead of silently launching external folders/tools.
- Native Registry APIs for Windows app-theme preference, user startup and HVCI state.
- Native Toolhelp process enumeration for LCore/OpenG27 process status.
- Native process-wait helper for installed/portable update handoff; Portable/Standalone updates no longer create temporary PowerShell updater scripts.
- Rotating startup/update logs and richer privacy-aware diagnostic metadata.
- Driver backup SHA-256 manifests and restore validation.
- Windows x64 release build plus an experimental ARM64 compile validation in CI.


## Acrylic hotfix in 0.0.6

- Main-window rendering now uses a 32-bit top-down DIB backbuffer with explicit alpha repair before presentation to the full-client DWM glass sheet.
- DWM material changes stay on the locked UI thread and roll back completely if backdrop, frame extension or Windows 11 24H2 redirection-alpha activation fails.
- High Contrast and allocation failures fail safely to the opaque renderer. Renderer/build/material details are included in diagnostics.

## Audit batch 11–20 — implemented in 0.0.6

11. **Block Modern / Generic HID without a supported connected target** — the UI and privileged migration path both require an explicitly selected supported wheel; multi-wheel ambiguity fails closed.
12. **Show disabled/current-mode states** — Modern/Legacy remain visible, but the active mode and unsafe/unavailable transitions are disabled visually, for mouse and for keyboard focus.
13. **No PowerShell from paint/UI summary paths** — Profiler summary access is cache-only; full probing stays on worker collection, never in paint or page rendering.
14. **Split the heavy 7-second refresh** — periodic auto-refresh now uses native/light probes and preserves the heavy driver/Profiler inventory. Full collection runs at startup, explicit refresh, device-settle refresh and safety-critical operations.
15. **React to `WM_DEVICECHANGE`** — device events invalidate input caches and are debounced on the UI timer; an event remains pending while another scan is busy.
16. **Cache WinMM capabilities and poll only the selected device** — capability discovery is cached; the 250 ms live loop calls `joyGetPosEx` only for the mapped joystick and re-enumerates once if that slot disappears.
17. **Surface Direct-HID `CreateFile` failures** — opening errors keep their `CreateFile` context and are exposed through the live input status instead of silently disappearing.
18. **Bound Direct-HID reconnect retries** — failed opens/reads use exponential backoff capped at five seconds rather than aggressive reopen loops.
19. **Distinguish HID connection states** — live input now exposes Connected, Idle, Malformed, Read error, Open error and Reconnecting states; malformed reports are counted without treating an idle wheel as disconnected.
20. **Model multiple wheels explicitly** — `State` now contains `[]WheelDevice` plus persisted `SelectedWheelID`. Multiple supported wheels are never guessed; the Wheel page can move the explicit diagnostic target, while destructive driver-package transitions require exactly one attached supported wheel because one package can affect several devices.

## Audit batch 21–30 — implemented in 0.0.7

21. **Explicit multi-device chooser** — when more than one wheel is detected, the Wheel page opens a real chooser instead of cycling implicitly.
22. **Reset detection without touching operational data** — “Erkennung zurücksetzen” clears the selected target and manual C294 identity only; driver backups, OpenG27, operating preference and pedal calibration remain intact.
23. **Never silently replace a missing selected wheel** — a persisted physical target that disappears leaves LogiMate in “Auswahl erforderlich” even if another single wheel is still connected.
24. **Distinguish scan failure from no hardware** — strict SetupAPI/PowerShell collection exposes a detection error and blocks device-changing actions instead of reporting an empty device set.
25. **Per-device C294 model identity** — manual G25/G27/DFGT confirmation is keyed by physical instance ID, so two C294 devices cannot inherit one global guess.
26. **Per-device pedal mapping** — Gas/Brake/Clutch mappings are keyed by physical wheel ID + model + mode, with fallback to pre-0.0.7 mappings.
27. **Stable labels for identical wheels** — chooser, Wheel view and diagnostics include model, PID and a shortened instance-ID suffix.
28. **Hard input handoff on target change** — changing the selected wheel closes the preferred HID handle and invalidates WinMM/HID caches before refreshing.
29. **Visible selection/detection health** — SelectionStatus and DeviceDetectionError are surfaced in UI state and diagnostic exports rather than remaining implicit.
30. **Regression guard for multi-wheel safety** — tests cover no silent target jump, explicit multi-wheel selection state, reset semantics, independent C294 identity and independent pedal mappings while destructive driver changes remain single-wheel only.

## Audit batch 31–40 — implemented in 0.1.4

31. **Direct HID foundation for G25 and Driving Force GT** — confirmed native C299/C29A targets now try a model-aware, read-only Direct-HID reader before WinMM. These parsers remain explicitly hardware-validation gated.
32. **Raw-data mode for report validation** — the Wheel page can expose raw HID bytes and input telemetry so differing hardware revisions can be inspected without guessing semantic mappings.
33. **Button learning** — choose a LogiMate control name, capture one newly pressed physical button and persist the mapping per stable wheel ID. Multiple simultaneous new presses are rejected.
34. **H-shifter learning** — G25/G27 can capture Neutral, 1–6 and Reverse. Duplicate signatures abort with a hardware/decoder warning instead of silently accepting an invalid map.
35. **Steering-angle calibration** — capture left, center and right and map the raw axis to a selected 270/360/540/720/900-degree working range.
36. **Pedal calibration** — capture rest/full travel for each available pedal, infer inversion, select deadzone and response curve, and persist calibration per physical wheel + model + mode. DFGT omits clutch.
37. **Report-rate visibility** — Direct HID exposes actual reader Hz and last-report age; WinMM exposes the application's selected-slot polling Hz.
38. **USB/input reconnect counter** — Direct-HID reopen after an unexpected read/open failure and WinMM recovery are counted and surfaced without counting intentional target changes as failures.
39. **Last HID/WinMM error** — the latest input error remains visible after recovery so intermittent faults do not disappear from diagnostics.
40. **Raw report snapshot** — the current Direct-HID payload can be copied as indexed bytes plus hexadecimal data for support and hardware validation.

## Audit batch 41–50 — implemented in 0.1.5

41. **Native output transport abstraction** — UI code never writes raw packets; one system layer owns validated Set_Output_Report writes for native G25/DFGT/G27 targets.
42. **Experimental output opt-in** — Native Wheel Output is disabled by default and must be explicitly enabled in Settings.
43. **Strict output target gate** — exactly one physical wheel, selected target, confirmed model, PnP verification, native PID and Generic-HID mode are required before any output report.
44. **OpenG27 ownership interlock** — LogiMate native output refuses to run while OpenG27 is active; launching OpenG27 from LogiMate first stops LogiMate output.
45. **Rotation range output** — bounded 40–900° classic range packets with user presets 270/360/540/720/900° for G25/DFGT/G27.
46. **G27 rev-LED test** — explicit 5-bit LED-mask builder plus 0–5 LED UI test; unsupported models fail closed.
47. **Bounded autocenter test** — experimental autocenter is capped at 30% and ramp 0–7; the UI exposes only 10/20/30% momentary tests.
48. **Output watchdog** — every non-zero autocenter test is automatically returned to zero after two seconds.
49. **Emergency output stop** — manual stop, application shutdown and OpenG27 handoff zero autocenter and clear G27 LEDs.
50. **Output diagnostics** — last command/error/write time plus command, emergency-stop and watchdog-stop counters are visible in diagnostics and the output status dialog.

## Audit batch 51–60 — implemented in 0.1.6

51. **Per-wheel FFB safety profile** — master gain, constant-force ceiling, future condition gains, slew rate and watchdog are stored per stable physical wheel ID with atomic writes.
52. **Bounded constant-force protocol** — classic slot-0 start/update/stop reports are implemented with a hard ±10% public test ceiling.
53. **Serialized FFB worker** — one bounded worker owns one HID handle per test session and updates every 20 ms.
54. **Master gain + hard clamp** — gain can reduce but never expand the hard experimental force ceiling.
55. **Slew limiter** — force changes ramp in bounded steps instead of jumping immediately to target.
56. **Heartbeat/watchdog** — active sessions expose heartbeat/frames and self-stop within a clamped 500–2000 ms window.
57. **Condition-effect protocol basis** — spring/damper/friction builders and effect-slot stop reports are pure/tested but remain hardware-validation gated.
58. **Single-owner interlock** — OpenG27 and LogiMate native FFB never intentionally own the same wheel output simultaneously; OpenG27 is rechecked during bounded tests.
59. **Zero-force handoff** — active force stops on app exit, feature disable, OpenG27 handoff, wheel-selection change, detection reset and `WM_DEVICECHANGE`.
60. **Native FFB diagnostics** — requested/applied force, effect, frames, slew limiting, heartbeat, safety config, watchdog/emergency stops and last error are exported.

## Audit batch 61–70 — implemented in 0.1.7

61. **Dedicated four-slot effect manager** — Constant=0, Spring=1, Damper=2 and Friction=3 with slot-specific start/update/stop commands.
62. **Bounded Spring hardware test** — slot-1 Spring is available only behind the strict native-output safety gate and watchdog.
63. **Bounded Damper hardware test** — slot-2 Damper uses the same short-session ownership rules.
64. **Bounded Friction hardware test** — slot-3 Friction uses the same short-session ownership rules.
65. **Deterministic force mixer** — effect gain, engine-profile master, safety master and hard effect ceilings are applied in a fixed pure/testable order.
66. **Clip telemetry** — hard clamps are counted and applied four-channel values are exported.
67. **Per-wheel Wheel-Engine profiles** — Gentle/Balanced/Direct plus Custom persist rotation and effect-character gains independently from hard safety settings.
68. **Transactional profile activation** — rotation must succeed before active-profile persistence; profile activation never starts motor force.
69. **Generation-safe session ownership** — stale FFB workers cannot overwrite newer session state after cancellation or effect handoff.
70. **Four-slot stop + Engine Dry Run** — Emergency Stop clears slots 0–3 and a no-motor dry run validates mixer/builders/stop packets.

## 0.0.8 UX hardening pass

- **State-driven Overview** — the primary command is derived from the current safe next step and the matching recommendation card is shown first.
- **Unified device management** — wheel selection, full re-detection and detection-state reset live in one TaskDialog; missing persisted targets are explained explicitly.
- **Attention states (0.0.8 behavior)** — detection/selection problems were moved into explicit explanatory states. In 0.0.9 the dashboard is intentionally kept visible underneath a compact attention panel instead of being replaced entirely.
- **Contextual safety semantics** — HVCI and backup status colors depend on the active workflow; the System page always states why a mode transition is available or blocked.
- **Dynamic primary emphasis** — System highlights backup only when it is actually required, then moves emphasis to the forward Modern/LogiMate Native action.
- **Cleaner page density** — Diagnostics, Settings and About skip the redundant wheel-status strip and gain usable vertical space.
- **About / project identity** — a native About page introduces Markus Kleine, related projects, OpenG27 credit, GitHub/Indicana links and optional PayPal support.
- **Support safety** — external actions are fixed HTTPS targets; PayPal is presented as voluntary project support rather than a tax-deductible charitable donation.
- **Regression guard** — Windows-target compile tests cover next-action decisions, About content, content geometry and human-readable mode-block reasons.

## 0.0.9 Acrylic / Wheel-dashboard recovery

- **Backdrop-visible sidebar** — when DWM material is active, the navigation rail no longer paints an opaque base layer over Acrylic/Mica. High Contrast, Safe UI and material-off modes remain opaque.
- **Persistent hardware instruments** — Wheel instrumentation is always rendered; attention states no longer replace it with a full empty-state card.
- **Multi-wheel overlay** — multiple detected wheels show a dedicated panel above the instruments with the active target or a clear selection requirement.
- **Native Engine end-state recorded** — the project now has a staged plan for absorbing the OpenG27 feature set into LogiMate while keeping upstream credit and the external engine as a fallback until hardware parity is proven.


## 0.1.3 wheel identity / setup state-machine hardening

- Persisted wheel identity is separated from the current Windows device session: `ContainerId` groups current devnodes; stable USB location/topology identifies the selected physical target across Logitech PID re-enumeration.
- The known 0.1.2 stale-container failure has a narrow migration rule requiring exactly one native wheel plus a model-consistent old hint. Arbitrary stale IDs never attach to another wheel.
- Machine-global C294 model fallbacks are retired. Manual identity is per stable physical wheel only.
- C294 is explicitly split into readable vs actionable state. Generic input diagnostics may run while model identity is unresolved, but native-mode commands and driver changes cannot.
- C294 names that explicitly identify unsupported Logitech models are fail-closed rather than offered as G25/G27/DFGT targets.
- G25, DFGT and G27 native-mode selectors share one guarded implementation. G27 Direct HID remains model-specific; G25/DFGT use WinMM for live diagnostics until their own native readers are hardware-validated.
- The native-mode writer tries all C294 Raw-Input collections belonging to the one authorized physical target instead of trusting enumeration order.
- All driver-changing UI entry points converge on the tracked setup migration. Deprecated direct admin switch commands are rejected.
- Non-elevated driver-inventory uncertainty is no longer confused with device-detection failure; destructive work still requires a strict elevated re-check.
- Setup model confirmation updates the UI snapshot immediately only after durable per-wheel persistence; privileged setup independently re-enumerates before changes.

### Remaining hardware gates before stable 1.0

- Full physical G27 matrix: C294/C29B, clean Generic HID, Legacy LGS, rollback, pedals/H-shifter, Direct HID, OpenG27 coexistence and power-cycle restoration.
- Full physical G25 matrix: C294/C299 selector, WinMM mapping, Legacy restore and all controls.
- Full physical Driving Force GT matrix: C294/C29A selector, WinMM mapping, Legacy restore and controls.
- Authenticode signing and complete Microsoft UI Automation semantics remain independent release requirements.

## P0 — still required before stable 1.0

### Authenticode signing

SHA-256 release verification protects integrity but does not authenticate the publisher to Windows or SmartScreen. A real publisher certificate is required before LogiMate can honestly claim signed releases.

### Physical G27 / G25 / Driving Force GT validation

The three integrated wheel paths must be exercised on real hardware across compatibility/native PIDs, Legacy/Generic-HID transitions, rollback and all available controls. G25 and DFGT native-mode switching is implemented but remains experimental until physical C299/C29A validation. Compile tests cannot replace USB/driver/FFB hardware tests.

### Full Microsoft UI Automation semantics

D5.8 adds standard native Windows accessibility peers for the custom-painted **main window**, including names, roles, focus, enabled state and checked/selected state. The Stable gate remains open until this bridge and the remaining secondary dialogs/setup/changelog are validated on real Windows with Narrator/UI Automation, High Contrast, reduced motion and mixed DPI. Compilation alone does not set `uiAccessibilityValidated=true`.

## P1 — high-value next improvements

- Complete resource-backed German/English localization; the settings schema already reserves a language preference but the full UI is still German-first.
- Move the remaining PowerShell compatibility/shell conveniences (fallback inventory and `.lnk` creation) to native APIs where worthwhile.
- Expand DPI work from font/min-window/suggested-rect handling to a fully logical-unit layout engine for every card/instrument coordinate.
- Add min/max/inversion/deadzone capture to the pedal learning workflow.
- Add signed MSI/WiX/Inno packaging once code signing is available.

## Release QA matrix

Before each public release, manually inspect Dark/Gray/Light/System with Windows material on/off, expanded/collapsed sidebar, keyboard navigation, hardware instruments, setup guide, Changelog, Safe UI, 100% and >100% DPI. Hardware-affecting releases must additionally exercise the physical G27 matrix and, before G25 stable claims, the full G25 matrix in `docs/TESTING.md`.

## Audit batch 71–80 — implemented in 0.2.0

71. **Versioned game profiles** — validated per-game profiles bind executables to LogiMate Wheel-Engine profiles and bounded gain policy.
72. **Native process inventory** — Toolhelp-based process discovery removes shell dependency from game detection.
73. **Foreground gate** — profiles may require the matched game to own the foreground window.
74. **Safe Auto-Apply** — automatic activation may set profile/range but can never start motor FFB.
75. **Bounded game gains** — game tuning stays subordinate to the per-wheel FFB safety config.
76. **LED policy per game** — off/manual/telemetry policy is persisted explicitly.
77. **Telemetry adapter binding** — protocol selection is profile metadata rather than engine hard-coding.
78. **Validated profile import/export** — bounded JSON import normalizes executable identity and rejects invalid entries.
79. **Game-session state** — current process/profile/engine/apply state is visible and diagnosable.
80. **No implicit motor ownership** — game detection never claims FFB output by itself.

## Audit batch 81–90 — implemented in 0.2.0

81. **Normalized telemetry frame** — RPM, redline, speed, force and control-state use one adapter-neutral model.
82. **Loopback-only JSON adapter** — the first native adapter listens only on 127.0.0.1:27100.
83. **Strict parser safety** — bounded packets, finite numbers and force clamping fail closed.
84. **Stale-data detection** — frames older than 750 ms are marked stale.
85. **RPM LED mapping** — deterministic five-stage G27 LED mask generation is pure and testable.
86. **Force control gate** — force is zero unless physics is active and the human player is in control.
87. **Bounded history** — telemetry retains at most 256 frames.
88. **Adapter registry** — protocols have stable IDs/capabilities; Wreckfest/OpenG27 Pino is reserved for the fusion stage.
89. **Profile/adapter association** — game profiles can select a telemetry source without coupling UI to a game.
90. **Telemetry diagnostics** — listener state, bad frames, staleness and last safe preview values are reportable.

## Audit batch 91–100 — implemented in 0.2.0

91. **State invariants** — internal consistency violations are explicit rather than inferred from UI strings.
92. **Internal self-tests** — protocol builders, parser, mixer and atomic persistence can be validated without hardware.
93. **Config snapshots** — supported state files are archived with SHA-256 manifests.
94. **Validated restore** — restore rejects traversal, unknown files, oversize content and hash mismatch.
95. **Output crash marker** — active motor tests leave a durable marker until they stop cleanly.
96. **Startup zero-force recovery** — an actionable Generic-HID target can be emergency-stopped after an unclean session.
97. **Data-schema versioning** — migration state is explicit and backup-first.
98. **Readiness report** — code health has a transparent score and issue list.
99. **Stable-release gate** — physical validation, signing and complete UIA remain first-class external blockers.
100. **100-point audit complete** — completion is not misrepresented as stable 1.0 certification.
