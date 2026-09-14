# LogiMate 0.1.3 — Wheel detection and setup state-machine audit

## Invariants

1. One physical wheel may have many Windows nodes, but only one logical LogiMate target.
2. Current Windows `ContainerId` groups a session; it is not a durable wheel identity across Logitech PID changes.
3. Durable selection/model confirmation uses stable USB topology (`usbloc:*`; `usbslot:*` fallback).
4. Native PID C299/C29A/C29B is authoritative for G25/DFGT/G27.
5. C294 is readable but not model-actionable until safely identified.
6. A manual C294 identity belongs to one stable physical wheel, never the whole PC.
7. Missing persisted targets never jump silently to a different wheel.
8. Multiple attached supported wheels may be diagnosed, but shared driver-package changes require exactly one attached actionable wheel.
9. Device scan failure is not “no wheel”; Driver Store inventory failure is not “wheel missing”.
10. Every destructive Modern/Legacy operation uses one elevated transaction path with fresh validation, backup/journal and rollback behavior.

## Supported state flow

### Fresh / no Logitech software
`PnP detect → group USB/HID → stable physical identity → native PID OR C294 one-time confirmation → choose Modern or Legacy`.

### Modern
- G27: Generic HID → guarded native C29B → LogiMate Direct HID; OpenG27 optional/current external engine until native LogiMate engine reaches parity.
- G25: Generic HID → guarded native C299 → conservative WinMM diagnostics.
- DFGT: Generic HID → guarded native C29A → conservative WinMM diagnostics.

### Legacy
`strict elevated inventory → existing Legacy OR verified LogiMate backup OR explicit fresh official LGS path → HVCI handling when required → rescan`.

## Migration compatibility

- old raw USB/HID selections are mapped to current stable aliases when exact correlation exists;
- old 0.1.2 `container:*` selection receives only a narrow native-PID + matching-model repair;
- old global `wheel.model` is never copied onto an ambiguous physical C294 target.

## Fail-closed cases

- ambiguous C294 for model-specific writes;
- explicitly unsupported C294 model name;
- multiple physical wheels for driver package changes;
- stale target with no exact/safe migration evidence;
- failed current device detection;
- failed strict elevated driver inventory before destructive work;
- invalid/tampered backups or unsigned/non-Logitech fresh Legacy installer.

## Validation boundary

Cross-compilation and static checks can validate code structure, but cannot prove real USB mode switching, old kernel-driver loading, force feedback, wheel report layouts or Windows Dev-channel behavior. These remain physical hardware tests for the release matrix.
