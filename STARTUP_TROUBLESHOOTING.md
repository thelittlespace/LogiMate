# LogiMate startup troubleshooting

## v0.3.4-alpha startup + rendering hardening

This release specifically addresses startup freezes seen in v0.3.0-alpha.

Key fixes:
- Win32 UI lifecycle pinned with `runtime.LockOSThread()`.
- Main window shown before Desktop Acrylic initialization.
- DWM/Acrylic setup moved out of the UI message pump.
- Undocumented Acrylic fallback removed from automatic startup.
- Blocking startup animation removed.
- Windows/PowerShell probes have hard timeouts.
- Independent probes run concurrently.
- Background notifications/refreshes are marshalled back to the UI thread.
- `startup.log` records startup milestones.
- `startup.pending` triggers automatic Safe UI after an incomplete prior launch.
- GitHub Actions runs `LogiMate.exe --smoke-test` on Windows before packaging.
- Installer no longer launches the main app elevated after setup.

## Safe UI
Run `LogiMate.exe --safe-ui`.

The installer creates **LogiMate - Safe UI** in the Start Menu. The portable package includes `LogiMate-SafeMode.cmd`.

## Startup log
Installed: `%LOCALAPPDATA%\LogiMate\startup.log`

Portable: `LogiMateData\startup.log` beside the EXE.

- Update checks start only after the main Win32 message loop has reached the stable-start milestone; network access is never part of window creation.


## Fluent action buttons

Since 0.3.4-alpha the four main page actions are painted inside LogiMate's shared double buffer instead of native owner-drawn BUTTON child windows. If only action buttons appear visually corrupted, verify that the running binary reports 0.3.4-alpha or newer in the sidebar / Was ist neu window before collecting a diagnostic report.
