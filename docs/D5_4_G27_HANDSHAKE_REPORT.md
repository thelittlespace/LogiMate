# D5.4 G27 Handshake Report

## Evidence from the physical G27

LogiMate 0.5.2/0.5.3 sees the wheel directly as Logitech VID 046D / PID C294 with HID product `G27 Racing Wheel`, input report length 8 and output report length 8. Detection is therefore proven. The remaining failure is the compatibility-to-native handshake.

## Difference found versus OpenG27/HidSharp

OpenG27 opens the selected HID read+write non-exclusively, prepends report ID 0x00, writes a full 8-byte output report and uses HidSharp's multi-second write timeout. LogiMate's normal runtime output transport intentionally used stricter writer sharing and a 350 ms deadline. Those safety defaults are appropriate for continuous FFB but are not an exact match for OpenG27's short-lived C294 switch.

## D5.4 changes

- Dedicated OpenG27-compatible switch transport for G27 only.
- `FILE_SHARE_READ | FILE_SHARE_WRITE` for the short-lived C294 handle.
- OVERLAPPED WriteFile retained.
- 3 second write deadline.
- Byte-exact report `00 F8 09 04 01 00 00 00`.
- 10 second C29B re-enumeration window.
- Stale persisted Legacy preference no longer blocks a live Generic-HID wheel.
- Manual Devices action exposes the same handshake with exact phase/error text.

## Physical pass criterion

Cold/power-cycled G27 starts as C294, LogiMate writes the switch, Windows removes C294, C29B appears, and LogiMate rebinds Direct HID without OpenG27 or LGS.
