# Build 018 — Neu-Audit / Hardening

Visible version: **0.0.1-alpha**  
Internal build: **018**

Build 018 was restarted from the verified Build 017 source baseline.

## Security and correctness fixes

- Removed all application paths that automatically disabled or re-enabled Memory Integrity/HVCI. Legacy setup now fails closed while HVCI is active.
- Removed the unused native HVCI registry writer so LogiMate retains read-only HVCI handling.
- Memory Integrity status is clickable and opens Windows Security Core Isolation; LogiMate never toggles the setting.
- Portable updater authorization now binds the staged update to the exact authorized `LogiMate.exe` target.
- GitHub update downloads require HTTPS and an approved GitHub download host.
- External browser links are HTTPS-only.
- Installer/uninstaller destructive removal is restricted to the exact managed Program Files path.
- Diagnostic export applies a final redaction pass for user paths, HID/USB identifiers and GUID-like identifiers; sidecar hash write failures are surfaced.
- Refresh-sensitive global application-state reads/writes were moved behind the state lock/snapshot path where identified by the audit.
- Current architecture/testing documentation was updated to remove obsolete release-phase terminology and obsolete HVCI instructions. Historical audit/release reports remain historical records.

## Toolchain baseline

- Go: 1.27.1 (stable; `go.mod` and CI)
- actions/checkout: v7.0.1, pinned to full commit SHA
- actions/setup-go: v7.0.0, pinned to full commit SHA
- actions/upload-artifact: v7.0.1, pinned to full commit SHA
- github/codeql-action: v4.37.5, pinned to full commit SHA
- actions/attest-build-provenance: v4.2.2, pinned to full commit SHA
- PowerShell is used as the Windows runner/build shell and selected Windows integration helper; it is runner-provided rather than vendored by LogiMate.

## Evidence boundary

Software gates do not replace physical hardware certification. Existing real G27 LED and FFB-live-test evidence remains valid, while USB-yank/reconnect, process-kill, suspend/resume, long HID stress, mixed-DPI/accessibility and G25/DFGT physical validation remain hardware/Windows execution gates.
