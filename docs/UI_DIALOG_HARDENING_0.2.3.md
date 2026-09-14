# LogiMate 0.2.3-alpha — UI & Dialog Hardening

## Root cause

Several UI paths tested `TaskDialogIndirect.Find()` in the wrong direction. In Go's `syscall.LazyProc`, `Find() == nil` means the Windows procedure **exists**. Some LogiMate paths treated this as unavailable, so command menus were skipped exactly on normal Windows 11 systems.

Affected patterns included:

- `Kalibrieren & Lernen`
- nested command selectors (`chooseCommand`)
- wheel/device management

0.2.3 removes direct `TaskDialogIndirect` routing from these primary UI paths instead of only flipping the condition.

## New dialog system

A LogiMate-owned Win32 dialog surface now handles:

- information
- warnings/errors
- yes/no confirmations
- scrollable command selections
- keyboard navigation (arrows/tab/enter/space/escape)
- High Contrast / Safe UI fallback through the existing theme/material stack

The visual structure mirrors the established `Was ist neu?` / setup guide language: brand header, subtitle, rounded content card, accent rail and Fluent-style action cards.

Classic MessageBox remains only as a last-resort emergency fallback if the custom window class itself cannot be created. Admin/uninstall/very-early startup paths create temporary dialog GDI resources so they can normally use the same surface even before the main window exists.

## Live Modern/Legacy migration

The setup guide now changes into a live progress surface while an elevated migration runs. It shows:

- current migration phase
- approximate phase progress plus live activity indicator
- recent durable migration journal steps
- done/warning/failed/running state colors
- expandable **Befehlsansicht** containing admin action, journal ID, timestamps and step details

The window intentionally cannot be closed while a driver migration is in progress.

## Standalone installer

`LogiMate-Setup-x64.exe` now has its own modern window instead of a MessageBox chain. It shows safe stages for:

1. waiting for the previous process during update
2. creating the install directory
3. writing `.new`
4. preserving `.previous` rollback copy
5. activating the new EXE
6. shortcuts
7. uninstall registry entry
8. final verification

The command view exposes the corresponding operation/command summaries.

## Fusion impact

OpenG27 Fusion C2 protocol shadowing is unchanged in 0.2.3. The scheduler milestone becomes **C3 / 0.2.4-alpha** so UI reliability can be validated independently before changing live FFB routing.
