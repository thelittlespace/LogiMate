# D6.1 HID Sharing Fix & Control Panel UX

**Release:** 0.6.1-alpha

## Trigger

A real G27 produced `HID-Output konnte nicht geöffnet werden ... being used by another process` when starting a bounded FFB live test from the D6 panel. The generic error did not distinguish a true external exclusive owner from a compatible HID client whose sharing flags simply did not match LogiMate's stricter first open.

## Fix

- `openNativeHIDHandleAdaptive` first keeps the protected LogiMate writer policy (`FILE_SHARE_READ`).
- Only on Windows `ERROR_SHARING_VIOLATION`, and only when no known external wheel writer is running, it retries once with `FILE_SHARE_READ | FILE_SHARE_WRITE`, matching the cooperative semantics used by OpenG27/HidSharp for classic Logitech HID interfaces.
- The central OutputLease and OS-wide LogiMate writer ownership remain unchanged; this is not a second writer engine.
- Known external writers (OpenG27, OpenG27FFB, Logitech LCore) stay blocked and are named in diagnostics. Unknown exclusive owners still fail closed.

## Control-panel improvements

- Native Output activation performs a no-report HID writer probe immediately.
- Every motor test performs the same preflight before acquiring its real effect session.
- The panel reports protected/shared writer mode, backend and shared-fallback count.
- Known external conflicts are displayed inline.
- Failed preflight stops before a force report can be sent.

## Safety

D6.1 does not relax motor authorization, force caps, OutputLease ownership, runtime recovery markers, watchdogs or Emergency Neutralize. Shared-open fallback changes only Windows handle sharing semantics after a specific sharing violation.

## Physical validation still required

Re-test on the user's G27 with LogiMate as the only wheel utility. Verify HID probe, then Constant/Spring/Damper/Friction/Autocenter at low test strength, and finally USB/reconnect behavior.
