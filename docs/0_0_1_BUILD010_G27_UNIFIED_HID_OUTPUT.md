# LogiMate 0.0.1-alpha · Build 010 — G27 Unified HID Output

## Trigger

Real G27 feedback after Build 009: none of the Force Feedback live tests produced a physical effect. Because Constant, Spring, Damper, Friction and Autocenter all failed together, the common transport/session path was treated as the primary fault domain rather than patching individual effect packets again.

## Root architectural mismatch

The current OpenG27 G27 implementation opens one shared HidSharp stream and attaches both its continuous input read loop and its output transport to that same stream. LogiMate still opened the Direct-HID reader separately from every output transport. A successful CreateFile/WriteFile on that second path proved Windows I/O completion, but did not prove that the command affected the active native G27 session.

## Build 010 changes

1. Native G27 C29B Direct-HID opens one `GENERIC_READ | GENERIC_WRITE`, `FILE_SHARE_READ | FILE_SHARE_WRITE`, OVERLAPPED session.
2. The input reader uses OVERLAPPED ReadFile on that session.
3. G27 native output borrows that exact session; no second G27 writer is opened.
4. Preferred output is OVERLAPPED WriteFile. A synchronous unsupported-write result may switch to HidD_SetOutputReport on the **same** handle only.
5. Ambiguous write state invalidates the unified session and forces discovery/reconnect.
6. OutputLease, OS-wide writer mutex, watchdog, recovery marker and Emergency Neutralize remain authoritative.
7. `Output prüfen` adds a physical, motorless RPM-LED pulse so the user can distinguish logical handle access from actual hardware delivery.

## Physical validation sequence

1. Connect G27 and confirm native PID C29B / Direct HID input.
2. Force Feedback → Live & Test → **Output prüfen**. All five RPM LEDs should light briefly.
3. If LED ping works: run Constant, Spring, Damper, Friction, Autocenter at the default low test strength, then STOP.
4. If LED ping does not work: do not increase force caps; capture Diagnostics → Problems/Log and HID transport metrics.
5. If LEDs work but all motors remain inert: the next investigation is effect protocol/activation, not HID ownership.

## Status

Software-complete; physical G27 evidence pending. Visible SemVer remains `0.0.1-alpha`; internal build `010`.
