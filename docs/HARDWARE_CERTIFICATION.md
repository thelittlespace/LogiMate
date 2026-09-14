# LogiMate hardware certification gate

Current release: **0.5.9-alpha / D5.9 HID Stress / Adverse-I/O Hardening**

This file describes the evidence required before LogiMate may be published as a stable release. The machine-readable state lives in `HARDWARE_CERTIFICATION.json`. D0 deliberately keeps all fields false: software gates are complete, but no automated build may invent physical hardware evidence.

## Required stable evidence

1. **Hardware matrix** — real G25, G27 and Driving Force GT on supported Windows versions; native/compatibility mode, reconnect, multiple wheels, buttons/pedals/shifter, range and supported outputs recorded.
2. **Crash recovery** — process kill, forced crash, USB removal, shutdown and sleep/resume with proof that no motor force remains latched and recovery markers behave fail-closed.
3. **Migration** — Modern → Legacy → Modern including interrupted operations, rollback, HVCI configured/effective state and restart boundaries on real Windows.
4. **UI/accessibility** — keyboard navigation, High Contrast, reduced motion, DPI changes and destructive-flow confirmations verified on Windows.
5. **HID stress** — repeated output start/stop, reconnect and deliberately adverse I/O conditions. D0 routes production writes through one serialized transport with deadline/cancellation and fail-closed poisoned-handle behavior; the Stable gate must prove that behavior on real wheels, including adverse I/O.
6. **Authenticode** — stable `LogiMate.exe` and `LogiMate-Setup-x64.exe` must be signed and successfully verified before packaging.

## Alpha policy

Preview/alpha builds may be produced with these certification fields false. They must remain clearly labelled experimental for motor-driving features. The GitHub workflows and `build.ps1` block a version without a prerelease suffix when the machine-readable certification is incomplete. The release workflow additionally requires Authenticode signing secrets for Stable. D0 software independence does not waive any physical-output certification gate.


## D1 evidence rule

From 0.3.2-alpha onward, Stable also requires `docs/HARDWARE_CERTIFICATION.json` to contain `perModel.g25`, `perModel.g27` and `perModel.dfgt` with `validated: true` and non-empty evidence lists. Synthetic fixtures and successful builds never satisfy this hardware gate.


## D5.7 evidence collection

0.5.7-alpha adds an in-app Hardware Certification Assistant. Its local per-wheel evidence under `LogiMateData\Certification` is intended to produce the evidence required by this file. It does not automatically edit or promote `docs/HARDWARE_CERTIFICATION.json`; Stable promotion remains a reviewed release action.
## D5.8 accessibility evidence rule

0.5.8-alpha adds a native Windows accessibility-provider bridge for LogiMate's custom-painted main window. Software/build evidence may prove that the bridge is present and internally coherent, but it must **not** set `uiAccessibilityValidated=true`. Stable accessibility evidence must include a reviewed Windows run covering Narrator/UI Automation discovery and invocation, full keyboard traversal, High Contrast, reduced motion, 100% and scaled/mixed DPI, plus the remaining custom secondary dialogs/setup/changelog.



## D5.9 HID stress evidence

0.5.9-alpha adds deterministic adverse-I/O policy stress, native HID transport metrics, a non-motor 100-cycle reopen/write/input probe and a guided USB-yank window with Markdown/JSON evidence export. Partial writes and failed OVERLAPPED completion confirmation now poison/close the writer fail-closed. These software/runtime tools do **not** automatically set `hidStressValidated=true`; reviewed physical G25/G27/DFGT evidence remains required before Stable.

## D5.10 Stable publication boundary

Hardware certification flags are necessary but not sufficient for Stable. After all physical/recovery/migration/UIA/HID-stress evidence is reviewed, the Stable build must still pass D5.10 Authenticode signing: exact VERSION-to-manifest match, signed/verified application before installer embedding, signed/verified final installer, matching publisher thumbprint, timestamped signing and final release attestation review.
