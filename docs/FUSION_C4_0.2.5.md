# OpenG27 Fusion C4 — G27 Device / Report Parser Parity

Version: **0.2.5-alpha**

## Scope

C4 ports OpenG27's raw `G27ReportParser` and mirrors the decision contract of
`G27Device.Connect` without replacing LogiMate's production Direct-HID reader.
The goal is parity evidence first, promotion later.

## Implemented

- OpenG27 raw G27 report parser translated to `internal/openg27port/g27_parser.go`.
- Steering idx4/5 little-endian and pedal idx6/7/8 semantics covered by golden tests.
- idx9+ button bytes packed into a little-endian 64-bit diagnostic mask like upstream.
- Pure G27 lifecycle planner: prefer native C29B; use C294 native-switch only when C29B is absent.
- Every valid production G27 Direct-HID report is shadow-parsed by the C4 port.
- Live MATCH / DIFF / parse-error counters plus last raw values are exposed in Engine Health and diagnostics.
- LogiMate StableWheelID, exact selected-wheel correlation, fail-closed multi-wheel behavior, reconnect backoff and shared Win32 HID reader remain authoritative.

## Engine Health fix

Prior releases built the Engine Health text once and displayed it in a modal
message dialog. Values therefore looked "live" in wording but could not change
while the window remained open.

0.2.5 introduces a reusable live-content mode in LogiMate's modern dialog:

- refresh timer: 500 ms for Engine Health;
- content is regenerated in-place;
- C3 scheduler counters, Native FFB heartbeat/state, telemetry, C4 parser parity
  and state invariants visibly change without reopening the dialog;
- heavier self-tests are refreshed only every 4 seconds.

## Safety

C4 performs no new motor output and does not replace `parseG27NativeReport`.
Parser promotion will happen only after real G27 shadow data shows stable parity.
