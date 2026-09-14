# D5.2 — OpenG27-style Detection Report

**Release:** 0.5.2-alpha  
**Scope:** wheel discovery only; no new FFB protocol or driver migration behavior.

## Why D5.1 was not enough

D5.1 fixed the real `PnPVerified` escaping regression and added a SetupAPI HID pass, but the normal state collector still launched device-tree, Raw Input and HID scans independently. During Logitech's C294 → native USB re-enumeration those scans could observe different moments and produce a short-lived "wheel missing / not actionable" result. The HID reader also had historically depended too much on Windows enumeration order.

OpenG27's G27-only connection path is simpler and robust: it enumerates HID directly, checks native C29B first, ranks candidates by maximum output report length, falls back to C294, sends the native switch, waits for USB re-enumeration and opens C29B again. It also exposes a direct HID device list for rescue/diagnostics.

## D5.2 design

D5.2 adopts the reliable parts without copying the unsafe G27-only assumption into a multi-model manager:

1. **HID-first discovery.** SetupAPI HID interfaces are probed directly for attributes/caps/product/serial.
2. **Capability ranking.** Largest output report wins, then input report size and joystick/gamepad usage.
3. **Coherent snapshot.** HID → PnP devnodes → Raw Input are collected as one detection cycle rather than unrelated parallel snapshots.
4. **Settle loop.** A C294/native overlap for one stable wheel, or HID visibility ahead of the PnP tree, causes a short bounded rescan.
5. **Native-switch reopen.** C294 → native waits on direct HID enumeration first; Raw Input is supplemental.
6. **Session HID identity.** An explicit G25/G27 HID product string can authorize the current C294 session. Generic compatibility strings cannot.
7. **Conservative history.** A truly observed native C299/C29A/C29B identity is remembered, but automatic reuse requires the same Windows session/container, a strong hardware fingerprint, or fresh matching model evidence. USB location alone is not identity.
8. **Direct HID diagnostics.** Devices can show the exact candidate PID/product/report lengths/path seen by the low-level scanner.

## Expected G27 flow

### Already native

`C29B HID visible` → rank best HID collection → correlate PnP → model=G27 → Direct HID input/output ready.

### Cold power-on / compatibility mode

`C294 HID visible` → correlate PnP → identify current session from HID product, WinMM corroboration, explicit user confirmation, or safe previous-native binding → if Modern is remembered, send guarded G27 selector → wait on direct HID scan → `C29B` appears → reopen and continue.

At no point does a generic C294 automatically become G27 merely because it occupies a USB port once used by a G27.

## Hardware validation requested

On the real G27, capture these cases:

- start with G27 already C29B;
- power-cycle so it returns as C294, then start LogiMate;
- start LogiMate first, then plug in the wheel;
- unplug/replug while LogiMate stays open;
- verify Devices → Direct HID detection lists C294/C29B and report lengths;
- verify the wheel remains selected/actionable after C294→C29B re-enumeration;
- verify Direct HID input and Native Output can coexist.

These are hardware evidence items, not claimed PASS by the cross-build environment.
