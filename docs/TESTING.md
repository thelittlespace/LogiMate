# D3 / 0.3.3-alpha software gate

Before packaging D2, run:

1. `go test ./internal/wheelengine/...` on the host — canonical model/session engine.
2. `go test ./internal/openg27port/...` — retained provenance/parity core.
3. Windows x64 `go vet -unsafeptr=false ./...`.
4. Windows x64 compile of `internal/system` and `internal/app` tests when not executing on Windows.
5. Windows x64 `cmd/logimate` build.
6. Windows x64 installer vet/build with the final application embedded.
7. Windows ARM64 compile validation.
8. Validate `HARDWARE_CERTIFICATION.json` and GitHub workflow YAML.
9. Test every output archive and verify `SHA256SUMS.txt`.

D1/D2 add targeted assertions for model adapters, per-model native input fixtures, runtime session invalidation, telemetry adapter alias migration, native protocol/scheduler/Pino parity and the Native Engine core gate.

**Physical gate:** the above does not replace real G25/G27/DFGT output, crash/reconnect, migration or HID-stress tests. Those remain D1/Stable evidence.

---

# LogiMate hardware test matrix

A compile-successful binary is not enough for a driver-management utility. Every release candidate that changes driver or device logic should be tested on real hardware.

## G27 matrix

Test at minimum:

- stock G27 wheel + pedals + H-shifter
- old Logitech Gaming Software/WingMan installed
- Generic HID state
- PID_C294 compatibility state
- PID_C29B native state
- Memory Integrity/HVCI off
- Memory Integrity/HVCI on

Required checks:

1. Detect G27 while Logitech driver is active.
2. Create a three-package WingMan backup where applicable.
3. Validate backup manifest and INF presence.
4. Switch to Generic HID without touching unrelated Logitech mouse/keyboard packages.
5. For G27 Modern, use LogiMate Native without requiring external OpenG27.
6. Detect C29B after LogiMate native-mode preparation; external OpenG27 is fallback-only.
7. Validate steering, pedals, buttons, paddles and H-shifter in Hardware Test.
8. Export a diagnostic ZIP and verify paths are redacted where expected.
9. Restore the old Logitech drivers.
10. Verify LGS works again after required restart.

## G25 matrix

Test at minimum:

- stock G25 wheel + pedals + H-shifter
- old Logitech driver active
- PID_C294 compatibility state
- PID_C299 native state when available
- HVCI off/on where Windows permits

Required checks:

1. Detect C299 as Logitech G25.
2. Treat bare C294 as ambiguous unless device text clearly identifies G25/G27.
3. Save a manual G25 preference and verify it survives restart.
4. Confirm native C299 overrides an incorrect/manual ambiguity label.
5. Back up the old Logitech packages.
6. Switch to Generic HID.
7. Ensure LogiMate does **not** automatically launch upstream OpenG27 for a confirmed G25.
8. Validate axes, pedals, shifter and buttons through Hardware Test.
9. Restore old Logitech drivers.
10. Verify no unrelated Logitech drivers were removed.

## Failure-injection tests

- UAC cancelled
- backup folder unwritable
- pnputil returns non-zero
- no wheel connected
- wheel unplugged during refresh
- no internet during OpenG27 update
- invalid/corrupt OpenG27 ZIP
- stale OpenG27 path
- OneDrive or redirected Desktop
- PowerShell cmdlets unavailable; WMI fallback path

## Release policy

G25, G27 and DFGT user-mode FFB/native HID engine functionality remains Experimental until each model passes LogiMate's own evidence-based hardware certification matrix. OpenG27 reference history does not count as D2 physical certification.

## Updater / changelog regression checks

Before a release, verify on a Windows x64 runner or test machine:

1. Fresh settings: first stable launch opens **Was ist neu?** once.
2. Second launch of the same version does not reopen it automatically.
3. Changing the embedded version causes **Was ist neu?** to appear again.
4. Manual **Updates prüfen** reports current state without blocking the UI.
5. Stable channel ignores GitHub pre-releases; Preview channel can select alpha/beta/RC releases.
6. An update asset with a wrong SHA-256 must be rejected and deleted.
7. Portable update waits for the running process to exit, replaces the package, restarts with `--post-update`, and preserves `LogiMateData`.
8. Installed update waits for the old PID before replacing Program Files and updates the Windows uninstall `DisplayVersion`.
9. Canceling UAC leaves the current installation intact.
10. Offline / GitHub 404 / rate-limit failures are non-fatal; automatic checks remain silent except for status/logging.

## WinMM detection regression checks

1. `joyGetNumDevs` must never be interpreted as the number of physically attached controllers.
2. Empty WinMM slots must be rejected by `joyGetPosEx` before `joyGetDevCapsW` is used for display metadata.
3. With multiple WinMM controllers, only an explicit model-consistent product name (G25/G27, or Driving Force GT for a confirmed native DFGT) may be selected automatically; broad “Driving Force”/“Racing Wheel” names are not enough to guess.
4. A single generic/Driving-Force WinMM controller may be correlated only when SetupAPI/Raw Input already reports a supported G25/G27/Driving-Force-GT/C294 wheel.
5. With multiple generic controllers and no explicit wheel name, LogiMate must report ambiguity instead of guessing.
6. Test with an unrelated Logitech controller (for example a gamepad/joystick) attached alongside the wheel.
7. Test Legacy and Generic-HID names, including generic `USB Input Device` exposure.
8. Validate X/Y/Z/R/U/V raw values, reported axis ranges, button mask, active button list and centered POV (`0xFFFF`).
9. The Windows `joy.cpl` button must open Game Controllers without elevation.
10. Diagnostic ZIP/report must include the selected WinMM device and selection reason.

## Theme / material regression matrix

For **Dunkel**, **Grau**, **Hell** and **System**, test both Windows material on and off:

1. Main title/chrome and sidebar match the selected theme.
2. Body/secondary text is readable on panel and elevated surfaces.
3. Primary, neutral and warning buttons have distinct normal/hover/pressed states.
4. Toggle on/off states remain clear.
5. Selected navigation and theme cards are visible without excessive saturation.
6. Hardware-test steering, axis, button and POV instruments remain readable.
7. Scrollbars, status dots, warning/good/bad states and the “Was ist neu?” window follow the same palette.
8. Switch repeatedly Dark → Light → Gray → Dark with Windows material enabled; no stale DWM material/chrome should remain.
9. Repeat with Windows material disabled and with `--safe-ui`.
10. Verify at least 100% and 150% Windows scaling and move the window between mixed-DPI monitors to validate WM_DPICHANGED behavior.


## 0.0.3-alpha first-run / accessibility / pedal regression

1. Fresh settings: the setup offer appears only after the first device-state refresh.
2. Skip disables future automatic offers; re-enable in Settings makes the offer eligible again.
3. Manual Overview → Einrichtung prüfen always opens the guide regardless of the startup preference.
4. Tab/Shift+Tab traverses visible navigation/actions/settings; Enter/Space activates the focused item.
5. High Contrast uses system colors and Windows reduced-motion disables LogiMate motion.
6. Pedal learning selects three distinct moved axes and persists mappings independently for Legacy and Generic HID.
7. Raw Input may corroborate a Logitech wheel but must never override authoritative PnP VID/PID model/mode.
8. Startup/update logs rotate once the configured size threshold is exceeded.


## Driving Force GT matrix

1. Verify `VID_046D&PID_C29A` resolves to Logitech Driving Force GT.
2. Verify a C294 device named “Driving Force GT” stays ambiguous until native evidence or explicit setup choice exists.
3. Verify choosing DFGT in setup never overrides a later C299/C29A/C29B native PID.
4. Verify DFGT WinMM diagnostics work without enabling OpenG27 actions.

## G27 Direct-HID / OpenG27 coexistence matrix

1. With OpenG27 installed and G27 in C29B, keep OpenG27 running and open LogiMate simultaneously.
2. Verify LogiMate selects “LogiMate Direct HID” before WinMM, both with OpenG27 closed and while OpenG27 is running.
3. Verify steering and Gas/Bremse/Kupplung move correctly and OpenG27 FFB/profile operation continues.
4. Change an OpenG27 pedal calibration/role, save it, and verify LogiMate reflects it after the config-cache refresh.
5. Stop/remove the native HID path and verify LogiMate falls back to WinMM without freezing.

## Guided Modern / Legacy migration matrix

1. Start from a working Logitech Legacy/LGS installation and choose Modern in the guide. Verify the **LogiMate Native core gate** runs before any driver removal and no external OpenG27 download/start is required.
2. Verify Profiler settings backup is created and driver backup is complete before optional Profiler uninstall.
3. Verify cancelling/failed Profiler uninstall leaves the legacy driver packages untouched.
4. Complete Modern setup and verify the stored operating preference is `modern`; after a wheel power-cycle LogiMate may restore C29B automatically only because Modern was explicitly chosen.
5. Return to Legacy and verify backup SHA-256 validation occurs before driver install.
6. Test Legacy with Profiler restore OFF (drivers only) and ON (settings restored; application installer only when a real installer was backed up).
7. With Memory Integrity/HVCI enabled, verify Legacy fails closed, offers to open Windows Security > Core isolation, never changes the setting itself, and reports the reboot requirement after a user-made Windows change.

## Native G27 controls matrix

1. With Direct HID active, verify all six wheel buttons and both paddles illuminate independently.
2. Verify all eight H-shifter buttons and each D-pad direction.
3. Verify H-pattern positions 1–6, neutral and reverse highlight the correct graphical gate position.
4. Keep OpenG27 closed and repeat the complete steering/pedal/button/shifter test to prove LogiMate does not depend on OpenG27 for input.
5. Start OpenG27 and repeat; both applications must remain responsive and input must continue.

## Reliability regression

1. Start a setup migration and cancel UAC: the guide must stay incomplete and the migration journal must end as cancelled.
2. Force a privileged migration failure after backup creation: the guide must remain open and show the failure instead of storing setup success.
3. Verify new Profiler backups, modify one payload byte, and confirm restore refuses the tampered backup.
4. Confirm settings/model/mode/pedal files never leave a `.tmp` file after a successful atomic replacement.
5. Disconnect/reconnect the wheel and verify `WM_DEVICECHANGE` triggers state refresh without waiting for the normal timer.
6. Deny Direct-HID access temporarily: the last open error must be visible and reconnect attempts must back off; after access returns the connection must recover.
7. Simulate legacy driver inventory failure: Modern/Legacy destructive actions must stop instead of interpreting it as an empty driver store.
8. With Memory Integrity/HVCI enabled, verify no Legacy driver mutation starts. After the user manually changes the Windows setting and reboots, force a Legacy restore failure and verify rollback never writes HVCI state.
9. Test a failed self-update activation/restart and verify the previous EXE is restored.
10. Keep an old LogiMate process alive beyond installer wait timeout and verify the installer aborts without overwriting the running installation.

## Acrylic regression

1. Start on Windows 11 22H2/23H2 with Dark + Windows material: main content, cards, text and sidebar must remain visible over Desktop Acrylic.
2. Repeat on Windows 11 24H2+: diagnostic report must show `Windows material active: true` and an Alpha-DIB renderer.
3. Toggle Windows material off/on repeatedly; no empty acrylic-only client, stale black sheet or half-applied title-bar state may remain.
4. Switch Dark → Gray → Light → System while material is active; every switch must repaint content and material consistently.
5. Start with `--safe-ui`; renderer must stay opaque and fully readable.
6. Force/observe a DWM material rejection (unsupported build / remote session / composition edge case): LogiMate must roll back to opaque rendering.
7. Resize to minimum size, maximize, restore and move between 100%/150% DPI monitors; the DIB must be recreated without disappearing content.
8. Rapidly unplug/replug the wheel: multiple `WM_DEVICECHANGE` broadcasts should collapse into a settled refresh and never be lost while another scan is busy.
9. Copy the diagnostics report repeatedly and verify clipboard output while watching for stable process handle/memory usage.
10. Verify the diagnostics report includes Windows build, renderer, material active state and renderer detail.


## 0.0.6-alpha audit 11–20 regression

1. Start with no supported wheel and verify Modern/Legacy are disabled; direct invocation must still fail closed.
2. With a supported wheel already in Generic HID, verify “Modern aktiv” is visible and not clickable/focusable; repeat for Legacy.
3. Leave LogiMate open for several 7-second cycles and verify no periodic PowerShell/driver-store/Profiler inventory process is spawned; manual refresh may perform the full scan.
4. Rapidly unplug/replug the wheel and verify `WM_DEVICECHANGE` settles to one coherent refresh without losing the event while another scan is busy.
5. On the Wheel page, move controls continuously and verify the cached WinMM mapping polls the selected ID; unplug it and verify one re-enumeration/recovery.
6. Force Direct-HID `CreateFile` failure and verify the visible error contains `CreateFile` context and reconnect state.
7. Unplug during Direct-HID reading and verify Read error/Reconnecting plus bounded retry backoff; reconnect and verify Connected returns.
8. Leave a connected G27 untouched for >3 seconds and verify Idle rather than disconnected; inject/observe malformed reports and verify Malformed is distinct from read failure.
9. Attach two supported Logitech wheels. Verify both appear in the logical target list, selection works for diagnostics, and all driver-changing actions remain blocked even after selecting one target.
10. Use “Geräte verwalten”, select another wheel, and verify `SelectedWheelID` changes/persists, restart LogiMate, then disconnect all but the intended wheel and confirm destructive validation only becomes available for that single supported target.


## 0.0.7-alpha audit 21–30 / multi-device regression

1. Attach two supported wheels; verify “Geräte verwalten” lists both with model, PID, mode and distinct ID suffixes.
2. Select wheel A, restart, and verify A remains selected.
3. Unplug A while B remains connected; verify LogiMate does not select B automatically and shows “Auswahl erforderlich”.
4. Use “Geräte verwalten” and explicitly choose B; verify live input caches/handles reconnect to B.
5. Use “Erkennung zurücksetzen”; verify selected ID and manual C294 identity are cleared, while backups, OpenG27, operating preference and pedal calibration remain.
6. After reset with one wheel attached, verify that sole wheel may be selected automatically; after reset with two wheels, verify no wheel is guessed.
7. With two C294 wheels, assign different manual models and verify each physical instance retains its own identity.
8. Learn different pedal mappings on two same-model wheels and verify each mapping returns only for its own selected physical device.
9. Force SetupAPI and PowerShell fallback failure; verify the UI reports a detection error rather than “no wheel”, and driver-changing actions remain blocked.
10. With multiple wheels connected, verify diagnostics/selection work but Modern/Legacy driver package transitions remain blocked until exactly one supported wheel is connected.

## 0.0.8-alpha UX / About regression

1. Start with no wheel: Overview primary action is **Geräte verwalten** and the first content card explains the next step.
2. Force device-detection failure: primary action becomes **Erkennung neu starten** and the header chip no longer says Ready.
3. Connect two supported wheels: **Geräte verwalten** lists both with stable identifiers, plus **Neu erkennen** and **Erkennung zurücksetzen**.
4. Select wheel A, disconnect it while wheel B remains: LogiMate must explain that the selected wheel is absent and must not silently switch to B.
5. With Legacy drivers but no backup: Overview primary action becomes **Treiber sichern** and Backup shows missing/warning.
6. With Generic-HID G27, the primary action is **Hardware testen** regardless of whether external OpenG27 is installed; OpenG27 appears only as explicit fallback under System.
7. System page always explains why Modern/Legacy actions are enabled or disabled.
8. Settings, Diagnostics and About use the taller content surface and remain scrollable at minimum window size / 150% DPI.
9. About shows Markus Kleine, all listed projects, OpenG27 credit, visible PayPal address and the four external/support actions.
10. Verify keyboard navigation reaches About and all four About actions; browser actions use the default system browser and never execute arbitrary schemes.


## 0.0.9-alpha Acrylic / persistent Wheel dashboard regression

1. With Acrylic enabled on Windows 11, verify the navigation rail visibly shows the system backdrop instead of a solid opaque strip.
2. Disable Acrylic and start with `--safe-ui`; verify the rail becomes fully opaque and all text remains readable.
3. Connect one selected wheel: no attention panel is shown and live instruments behave normally.
4. Connect a second supported wheel: instruments stay visible and a multi-wheel panel appears above them.
5. With two wheels and no selected target, the panel explicitly requests selection while instruments remain visible with neutral/no-live state.
6. Disconnect the persisted target while another wheel remains: LogiMate must not silently switch; the panel explains the missing target.
7. Force a device-detection error: the panel reports detection failure and Diagnostics contains the technical error; the hardware dashboard still renders.
8. Verify scrolling keeps the panel/instruments layout stable at 100%, 125% and 150% DPI.

## 0.1.3-alpha wheel identity / setup state-machine regression

- Friendly-name C294: expose the wheel for read-only diagnostics but verify `ModelConfirmed=false`, `PnPVerified=false` and block every model-specific/driver-changing action until explicit confirmation.
- Native C299/C29A/C29B: verify `ModelConfirmed=true` and `PnPVerified=true`; per-wheel C294 confirmation sets `ModelConfirmed=true` but leaves `PnPVerified=false`.

1. Reproduce the 0.1.2 report: persist an old `container:*` G27 selection + old G27 model hint, then expose exactly one C29B G27 in a new ContainerId. Verify LogiMate migrates to `usbloc:*`/`usbslot:*`, selects it, reports Generic HID / Modern and allows live input.
2. Repeat the previous test without a matching old model hint. Verify LogiMate fails closed and asks for an explicit selection rather than attaching stale state to an arbitrary wheel.
3. Connect one driverless C294 wheel. USB + HID child nodes must produce one logical wheel. It may be auto-selected for diagnostics, but Modern/Legacy and model-specific HID writes remain blocked until G25/G27/DFGT is safely known.
4. Put an old machine-global `wheel.model=Logitech G27` on disk, then connect a new unknown C294 wheel. Verify the wheel stays ambiguous, the global file is retired, and no per-device confirmation is created automatically.
5. Confirm a C294 wheel as G25, G27 and DFGT in separate runs. Verify the confirmation is stored under the stable physical ID and survives refresh/restart; native C299/C29A/C29B must override it authoritatively.
6. For each integrated model, exercise Modern: C294→C299 (G25 selector 0x02), C294→C29A (DFGT selector 0x03), C294→C29B (G27 selector 0x04). If Raw Input exposes several C294 collections, verify LogiMate tries writable collections and does not assume the first path.
7. Connect two real wheels. Different Windows ContainerIds remain separate logical wheels; driver-changing actions stay blocked even after selecting one. Disconnect the selected wheel and verify LogiMate never jumps to the other wheel.
8. Test Legacy on (a) an already-working LGS install, (b) a verified LogiMate driver backup and (c) a fresh PC with no backup. Case (c) is allowed only with explicit Profiler/LGS choice and a valid Logitech Authenticode signature on the official installer.
9. Force read-only Driver Store inventory failure in the normal process. The wheel must still be detected/testable and the setup guide may open; the elevated process must repeat a strict inventory and abort before destructive work if it still cannot verify the driver state.
10. Verify old direct admin actions (`switch-open*`, `switch-legacy*`) fail closed. Only `setup-modern*` / `setup-legacy*` may change drivers.
11. Immediately after confirming an ambiguous C294 model in the wizard, continue without waiting for the background refresh. The current UI snapshot must already be actionable and the elevated process must revalidate the hardware.
12. Move the same wheel to another physical USB port. Treat the new topology as a new physical identity and require explicit confirmation instead of silently importing the previous port's C294 identity.

## 0.1.5-alpha audit 31–40 input regression

1. **G27 Direct HID:** connect a native C29B G27; verify steering/pedals/buttons still update and the dedicated G27 parser remains preferred over WinMM.
2. **G25 Direct HID:** on real C299 hardware, verify the read-only Direct-HID source opens, steering/pedals update, and a failure falls back to WinMM without changing drivers. Record raw reports before trusting semantic button/shifter labels.
3. **DFGT Direct HID:** on real C29A hardware, verify steering/Gas/Brake update and no clutch calibration step is offered. A Direct-HID failure must fall back to WinMM.
4. **Raw-data mode:** toggle it from `Kalibrieren & Lernen`; verify the normal instruments stay visible and raw bytes/rate/report-age/reconnect/error telemetry appears without any HID output/FFB write.
5. **Button learning:** release all buttons, learn one target, then verify it follows only the learned physical bit. Press two previously-unpressed buttons during capture and verify the wizard refuses to guess.
6. **H-shifter learning:** on G25/G27 capture Neutral, 1–6 and R. Verify eight unique signatures are required and a duplicate aborts with a warning. Verify DFGT reports that H-pattern learning is not applicable.
7. **Steering calibration:** capture full left/center/full right at 900 degrees, verify the dashboard reads approximately -450 / 0 / +450 degrees, then repeat with a reversed raw axis and a smaller selected range.
8. **Pedal calibration:** capture released/full values for all available pedals, test 0/2/5/10% deadzone choices and Linear/Progressiv/Weich curves, and verify values clamp correctly outside the learned raw range.
9. **Telemetry/recovery:** unplug/replug during live Direct HID and cause a WinMM poll recovery where possible; verify reconnect count increments only for unexpected recovery and the historical last error remains visible. Intentional target switching must not increment Direct-HID reconnects.
10. **Raw snapshot:** copy a Direct-HID report and verify the clipboard contains source, byte count, indexed `0xNN` rows and a HEX payload. With WinMM-only input, verify LogiMate explicitly says no HID raw report is available.

**Release-gate note:** Windows cross-compilation exercises build correctness only. Points 2–3 and the exact G25/DFGT semantic report layout remain experimental until run on real hardware.


## 0.1.5-alpha audit 41–50 native-output regression

1. Default settings: Native Wheel Output is OFF and no native-output command link is shown.
2. Enable Experimental output with one confirmed native Generic-HID wheel: the Native Output entry appears under Kalibrieren & Lernen.
3. Repeat with two wheels, unresolved C294, Legacy binding or unverified target: every output command must fail closed before a HID write.
4. Run OpenG27 and retry: LogiMate must refuse native output. If LogiMate owns an active test and OpenG27 is launched from LogiMate, stop output first.
5. On physical G25/DFGT/G27, test 270/360/540/720/900° one at a time and verify the hardware range matches; record model/revision before removing Experimental status.
6. On physical G27 only, test LED levels 0–5 and verify cumulative bar order; G25/DFGT must reject LED output.
7. Autocenter: with hands clear, test 10%, then 20%, then 30%. Each test must return to zero automatically after two seconds. Do not promote beyond Experimental until all three wheel families are physically validated.
8. Trigger EMERGENCY STOP during/after a test: autocenter must be zero and G27 LEDs off.
9. Close LogiMate during an active autocenter test: shutdown path must attempt the same safe stop before process exit.
10. Diagnostic report must retain last output command/error, command count, emergency-stop count and watchdog-stop count after recovery.
11. Protocol unit tests must reject range >900°, LED mask >0x1F, autocenter >30% and ramp >7.

## 0.1.6-alpha audit 51–60 Native FFB Engine regression

1. Fresh settings: verify `Native Wheel Output (Experimental)` is OFF and no FFB test menu is reachable until explicitly enabled.
2. With one confirmed Generic-HID wheel selected, choose the Sanft/Standard/Audit-Maximum FFB safety presets; restart LogiMate and verify the per-wheel values persist.
3. Inspect/compile-test `BuildClassicConstantForceReport`: values outside ±10% must fail; left/neutral/right encoding must remain monotonic.
4. Inspect/compile-test slot-stop reports 0–3 and spring/damper/friction builder bounds; values above 30% condition gain must fail.
5. On physical hardware, run only the 5% constant-force test first. Verify force ramps in rather than snapping instantly and stops by watchdog without user action.
6. Repeat 10% left/right only after the 5% test is safe. Verify diagnostics show requested/applied values, frames and slew-limited updates.
7. Start OpenG27 during an active bounded test. LogiMate must stop its own force and record the ownership/interlock reason.
8. During an active test, trigger a USB device-change/replug. LogiMate must stop/release force before rebuilding device identity.
9. During an active test, select another wheel or reset detection. The previous FFB session must be stopped before target persistence/cache handoff.
10. Disable Native Wheel Output or close LogiMate during a test; no force may remain active. Re-open and confirm the diagnostic counters/history are sane.
11. Multi-wheel, ambiguous C294, Legacy driver binding or unconfirmed-model states must reject FFB startup before any motor report is written.
12. Spring/damper/friction remain protocol-only in 0.1.6: verify there is no persistent-game UI path that enables them yet.


## 0.1.7-alpha audit 61–70 Native Effect Manager regression

1. Compile-test slot opcodes: Constant slot 0, Spring slot 1, Damper slot 2, Friction slot 3. Verify start/update/stop commands differ by slot and condition effects reject slot 0.
2. Force mixer: feed extreme requests through Direct + Audit-Maximum and verify Constant never exceeds its safety cap and each condition never exceeds its own hard cap; clip count must reflect every clamp.
3. Force mixer gain order: with a 50% profile master and 50% effect gains, verify requests are reduced before the hard safety stage and do not spuriously count as clipping.
4. Profile persistence: save Custom on wheel A, activate it, restart/read back, and verify wheel B still defaults to Balanced.
5. Profile activation: on real hardware apply 900/720/540/360/270 profiles. If the range write fails, the new profile must not become active. Applying a profile must not start FFB.
6. Spring hardware test: start at 10% request with hands clear; verify slot 1 starts, diagnostics show Spring applied, and watchdog stops it. Increase only after physical behavior is confirmed.
7. Damper hardware test: repeat for slot 2; verify no Spring/Friction channel is reported as active.
8. Friction hardware test: repeat for slot 3; verify no Spring/Damper channel is reported as active.
9. Effect handoff: start one bounded test and immediately start another. The old worker must stop/release before the new generation becomes owner.
10. Stale generation regression: an old worker/status callback using generation N must be rejected after generation N+1 exists.
11. Start OpenG27, connect a second wheel, switch to Legacy, or make model identity ambiguous: all new effect tests must fail before a motor write.
12. EMERGENCY STOP must issue stop reports for all four slots and clear all applied effect channels.
13. Native Output status: a one-shot rotation command must leave `Active=false`; non-zero LEDs/autocenter may remain active until explicitly cleared/watchdog-stopped.
14. Engine Dry Run must report PASS without hardware and include slot0-constant, slot1-spring, slot2-damper, slot3-friction and stop-slots.
15. Diagnostic report must include active Wheel-Engine profile, FFB generation, Constant/Spring/Damper/Friction values, mixer clip count and transition count.

**Release-gate note:** cross-compilation validates code paths, not motor behavior. Spring/Damper/Friction remain Experimental until exercised on physical G25/G27/DFGT hardware.

## OpenG27 Fusion C1 — 0.2.1-alpha

C1 must not alter hardware behavior. Validate normal 0.2.0 wheel detection/input first, then run LogiMate's internal self-tests and require PASS for:

- OpenG27 C1 force scaling parity
- OpenG27 C1 steering calibration parity
- OpenG27 C1 pedal calibration parity
- OpenG27 C1 range tracker parity

Source-level gate: `go test ./internal/openg27port` must execute successfully on the host platform. Windows app/system test packages must compile for `windows/amd64`. No C1 function is permitted to open a HID handle or send a motor/output report.


## OpenG27 Fusion C2 — 0.2.2-alpha

### Hardware-independent gates

Run on the build host:

```bash
go test ./internal/openg27port
```

The suite must include OpenG27 golden vectors for Constant Force, global Stop, 200/900° Range, G27 Native Switch, SpringSet/Enable/Off, Damper clamp/off, Friction full-byte behavior/off, G27 LEDs and cumulative LED thresholds.

### Windows compile gates

Compile the Windows system/app test packages. `fusion_c2_windows_test.go` expects parity for the report paths that already match and deliberately expects DIFF for the legacy LogiMate G27 LED packet, Constant Force payload and missing SpringEnable sequence. A future promotion must update this test intentionally rather than accidentally.

### Manual 0.2.2 test

1. Verify G27 detection/input/calibration behave exactly as 0.2.1.
2. Open **Kalibrieren & Lernen → Engine Health**.
3. Locate **OpenG27 Fusion C2 · Protocol Shadow**.
4. Confirm the section renders MATCH/DIFF entries and hexadecimal reports without moving the wheel.
5. Export a normal diagnostic report and confirm the same Protocol Shadow section is included.
6. Native Output remains Experimental and C2 itself must not send any new reference report.

Expected C2 baseline: 6/9 representative comparisons match; LED, Constant Force and SpringEnable sequence remain visible differences pending C3 promotion/hardware validation.

## UI & Dialog Hardening — 0.2.3-alpha

Manual Windows validation:

1. Open **Lenkrad → Kalibrieren & Lernen**. The modern command window must open; it must no longer fall directly into pedal calibration.
2. Open nested selectors (button learning, steering range, pedal curve/deadzone, Native Output submenus). Every selector must open in the LogiMate Acrylic/Mica style.
3. Open **Geräte verwalten** with zero, one and multiple supported wheels. Device choices plus Refresh/Reset must appear in the same modern command surface.
4. Trigger information, warning, error and yes/no prompts. They should use the LogiMate branded dialog; long status text must scroll with the mouse wheel.
5. Start a Modern/Legacy migration (only on test hardware): the setup guide must switch to live progress, show new durable journal steps and allow **Befehlsansicht anzeigen** without allowing the migration window to close mid-transition.
6. Run `LogiMate-Setup-x64.exe`: verify ready screen → live progress → success/error screen and expandable command/log view. Update mode must preserve `.previous` until the new executable is activated.
7. Verify Safe UI / High Contrast still gets an opaque readable surface.
8. Re-run Fusion C2 Protocol Shadow; expected packet comparison remains unchanged from 0.2.2.

Compile gates:

- Windows x64 app and installer
- Windows app/system test binaries
- Windows ARM64 app compile validation
- `GOOS=windows GOARCH=amd64 go vet -unsafeptr=false ./...`
- `GOOS=windows GOARCH=amd64 go vet -tags installer -unsafeptr=false ./cmd/installer`


## OpenG27 Fusion C3 — 0.2.4-alpha

### Hardware-free checks

1. Open **Kalibrieren & Lernen -> Engine Health**.
2. Confirm `OpenG27 C3 scheduler dry-run` is PASS.
3. Confirm the `OpenG27 Fusion C3 · Scheduler` section reports a PASS dry run and a 6 ms parity cadence.
4. Export a diagnostic report and confirm the same C3 scheduler section is present.
5. Existing C2 Protocol Shadow must still be present and retain its known MATCH/DIFF results.

### G27 bounded live validation

Prerequisites: exactly one confirmed G27, native C29B/Generic HID, Experimental Native Output enabled, OpenG27 not running.

1. Open **Native Wheel Engine -> FFB-Effekttests -> Fusion C3 Scheduler · G27**.
2. Start with **2% left**. Keep hands loose and Emergency Stop ready.
3. The wheel should apply a very light directional force and return to neutral automatically within 900 ms.
4. Repeat **2% right**. Verify direction changes and automatic stop.
5. Only if both are clean, optionally test ±5%.
6. Open **FFB Engine Status** and record C3 pumps/source frames/HID writes/stop reason.
7. Verify `StoppedReason` normally becomes `900ms-test-timeout` and Active becomes false.
8. Start a C3 test and press **EMERGENCY STOP** immediately; force must end and no residual torque may remain.
9. Start C3 and then launch external OpenG27; C3 must stop because dual output ownership is forbidden.
10. Disconnect/reconnect the wheel only after force has stopped; device-change handling must not leave C3 active.

Expected safety limits: live C3 is G27-only, requested force never exceeds ±5%, and the configured LogiMate slew rate remains equivalent to the previous percent/20ms safety profile.


## OpenG27 Fusion C4 + Live Engine Health — 0.2.5-alpha

### Hardware-free

1. Run `go test ./internal/openg27port` and confirm the G27 parser/lifecycle tests pass.
2. Cross-compile `./internal/system` and `./internal/app` Windows test packages.
3. Open **Engine Health** with no force test running. The top timestamp must change roughly twice per second without closing the window.
4. Confirm `OpenG27 C4 G27 report parser parity` is PASS in the self-test list.

### G27 live shadow

1. Connect the G27 in native C29B mode and open **Engine Health**.
2. Move steering and all three pedals. `C4 · G27 Parser Shadow` report count must increase live.
3. Expected result on the standard 12-byte native report: MATCH count increases and DIFF remains 0.
4. Press wheel/shifter controls and verify the displayed upstream button mask changes; this is diagnostic only.
5. Confirm Device Lifecycle says existing C29B is opened directly without a USB reset.
6. Export diagnostics and confirm C4 parser/lifecycle information is present.

### Live Engine Health

1. Keep Engine Health open and run the bounded C3 ±2% test from another pass after closing/reopening the command surface as needed.
2. Reopen Engine Health while C3 is active: pumps/writes/heartbeat must change without reopening Engine Health.
3. Start/stop the local Telemetry Hub and confirm the live telemetry line changes.
4. No live refresh may trigger HID output by itself.


## Fusion C5
- Put a valid OpenG27 `game-profiles.json` in `%APPDATA%\OpenG27`.
- Game-Profile -> OpenG27-Profile importieren. Confirm translated process, engine profile, Pino adapter and LED policy.
- Confirm Native FFB safety settings are unchanged and no motor force starts.

## Fusion C6
- Add/import the Wreckfest 2 OpenG27 preset.
- Start Wreckfest 2 with Pino telemetry configured to 127.0.0.1:23123.
- Engine Health must show adapter `wreckfest2-pino`, increasing packets/frames and non-stale RPM. A saved legacy `openg27-pino` profile must normalize to the same adapter.
- Preview must show the reference LED mask and physics/player-control gating.
- For hardware validation, explicitly start Native Game Output. On G27 verify Game-FFB + RPM LEDs; on G25/DFGT verify Game-FFB without G27-only LED writes. Verify direction at low safety limits first. Closing the game, losing telemetry, OpenG27 startup, Emergency Stop or device change must neutralize output.

## OpenG27 Fusion C7 — 0.2.8-alpha

### Software cutover
1. On a confirmed/PnP-verified G27 in Generic HID, open Engine Health and locate **OpenG27 Fusion C7 · Native Cutover**.
2. Verify C1–C6 Core, G27 target, Windows Modern Mode, Output Ownership and Native FFB Scheduler are PASS when healthy.
3. Known old C2 compatibility-builder differences may be WARN; they must not become hidden PASS. C3/C6 use the translated reference bytes.
4. If C4 reports any live parser DIFF or parse error, C7 must become BLOCKED.
5. Start external OpenG27: Output Ownership must become BLOCK and LogiMate native output must relinquish ownership.
6. In Legacy mode, current C7 readiness must be BLOCKED even though the hardware-free pre-migration core gate remains usable.

### Modern migration cutover
1. Start G27 Legacy -> Modern migration with no OpenG27 installed and networking unavailable. The migration must not require an OpenG27 download.
2. Verify **LogiMate Native Engine prüfen** completes before destructive driver-package steps.
3. Complete the transition and verify OpenG27 is not auto-started afterward.
4. If OpenG27 is already installed, verify it remains untouched and is described as optional fallback.
5. Open System -> **OpenG27 Fallback** and verify status/install-update/start are explicit manual actions.
6. Launch the fallback from LogiMate and verify any active native output is stopped before the fallback process starts.

### Physical warning matrix
The following remain WARN/certification work until physically recorded on the real G27: C3 both force directions and stop behavior, C6 Wreckfest force/RPM LEDs, USB power-cycle/reconnect, sleep/resume, game exit/telemetry loss and crash-recovery/no-stuck-force behavior. C7 must not describe these as stable PASS merely because the software gate passes.

## 0.2.9-alpha / Fusion C8 final regression and certification policy

C8 separates **software-verifiable regression** from **physical certification**. A local/cross build may pass the software gate without claiming that a wheel has been physically certified.

### Software regression gate

1. `go vet -unsafeptr=false ./...` for Windows/amd64.
2. Compile Windows system/app test binaries and the main x64 application.
3. Build the installer with the final application embedded.
4. Compile the application for Windows/arm64 as a portability/compile validation only.
5. Run the hardware-independent `internal/openg27port` tests on the host.
6. On Windows CI, run `go test ./...` and the `--smoke-test` executable path.
7. Verify all release ZIPs and SHA-256 manifests after packaging.
8. Confirm `VERSION`, executable linker version, installer version and release tag are derived from the same source.

### C8 safety regression

- A second LogiMate GUI instance must be refused.
- A second native-output owner must not acquire the global lease.
- A C294 PnP device without explicit current-device model confirmation must remain non-actionable for model-specific writes.
- A session-only C294 confirmation must disappear after PnP invalidation/restart; only a strong serial-derived fingerprint may support cross-session confirmation.
- Output marker creation must precede motor writes. Any failed/unconfirmed stop must retain lease/recovery state.
- LED-only telemetry must never acquire a motor lease or create non-zero FFB.
- Pino `FFBEnabled=false`, stale telemetry, `Physics=false` or `PlayerControl=false` must yield zero force.
- Corrupt critical JSON must be preserved and normal writes must be blocked until explicit repair/reset.
- Interrupted migration journals must remain visible and block a conflicting new migration until terminal state is verified.
- Automatic update application must reject a candidate without a valid same-publisher Authenticode signature.

### Physical Stable gate

The following are **not** asserted by the C8 alpha software build and must be recorded in `HARDWARE_CERTIFICATION.json` before Stable:

- real G25/G27/DFGT input/native-mode matrix,
- G25 H-shifter threshold verification,
- G25/G27/DFGT supported output protocol verification,
- force start/stop plus process-kill/crash/USB-yank/sleep/resume no-stuck-force tests,
- Modern↔Legacy/HVCI migration and rollback on real Windows,
- sustained HID I/O stress, including adverse/slow device behavior,
- complete Windows UI keyboard/High-Contrast/DPI/reduced-motion validation,
- Authenticode verification of the final application and installer.

C8 intentionally leaves these certification flags false. An Alpha package may be distributed for testing; a Stable package may not be produced by the supplied pipeline while any gate is false.


## D5.1 detection recovery matrix

1. Start with a real G27 already in C29B. Verify Windows `USB\VID_046D&PID_C29B...` / `HID\...` nodes produce `PnPVerified=true` and the wheel is actionable.
2. Cold-plug/replug a G27 that appears as C294. With one wheel and Modern remembered, verify matching PnP + explicit WinMM G27 evidence creates only a session model proof and LogiMate restores C29B automatically in the background.
3. Restart/replug with a generic/ambiguous C294 name and no matching G25/G27 WinMM model. Verify LogiMate remains read-only and requests explicit model confirmation.
4. Verify DFGT C294 never becomes actionable from WinMM alone.
5. Verify direct SetupAPI HID-interface enumeration still finds the wheel when Raw Input returns no path.
6. Verify USB and HID child devnodes for one wheel are grouped into one logical wheel even if ContainerID metadata is absent/incomplete.
7. Verify two wheels never collapse into one group and no target is guessed.
8. Verify G27 Direct-HID input remains readable while Native FFB/output opens its own writer handle.


## HID stress / adverse-I/O regression

1. Run **HID Stress -> Software Fault-Injection / Policy Stress**. It must PASS 10,000 iterations and must not touch hardware output.
2. Verify Engine Health includes `HID adverse-I/O policy stress: true` and live transport counters.
3. With one PnP-verified wheel in Modern/Generic HID and no active FFB/game output, enable Native Wheel Output and run the **100-cycle live stress**. It must complete all cycles, retain valid input samples and release the central output lease.
4. Confirm the live HID probe uses only the active range report; no Constant Force/Spring/Damper/Friction/Autocenter effect may start.
5. During **USB-yank adverse I/O**, unplug the wheel during the test window. A real HID error/disconnect must be observed and the non-motor output lease must still release.
6. Reconnect the wheel, wait for normal detection, then rerun the 100-cycle test. It must use the current session/path and must not resurrect a stale writer.
7. Force or fixture a partial write / failed completion query in Windows tests. The transport must become poisoned+closed and reject reuse.
8. Confirm only synchronous errno 1/50/87 can enter the HidD compatibility backend; access denied, invalid handle, operation aborted, I/O pending/incomplete and device-not-connected classes may not silently retry.
9. Export each physical run. Verify both `.md` and `.json` evidence are created under `LogiMateData\Certification`.
10. Do not set repository `hidStressValidated=true` until the physical evidence is reviewed for G25, G27 and DFGT as required by the Stable certification policy.
