# D5.5 Automatic Detection Fix Report

**Release:** 0.5.5-alpha  
**Scope:** real-hardware G27 automatic confirmation before the already-working D5.4 native handshake.

## Real-hardware symptom

On the user's physical G27, Direct HID correctly reported:

- PID C294
- product `G27 Racing Wheel`
- input report length 8
- output report length 8

Manual model confirmation immediately allowed the wheel to continue to the native activation path, proving that HID discovery and the D5.4 C294→C29B handshake were no longer the primary failure.

## Root cause

LogiMate obtains wheel evidence from both the all-class SetupAPI device-tree pass and the direct SetupAPI HID-interface pass. Both passes can describe the **same HID devnode**.

`mergeDeviceEvidence` used the device instance ID as a de-duplication key. When the all-class PnP record was already present, the HID-interface record was skipped completely. The skipped record contained the direct HID `ProductString` (`G27 Racing Wheel`) and optional HID serial.

This created an inconsistent state:

- Direct-HID diagnostics (which scan HID interfaces directly) showed `G27 Racing Wheel`.
- The fused state used for automatic authorization contained only the generic PnP record.
- `applySessionHIDProductConsensus` therefore had no product evidence and left `ModelConfirmed=false`.
- `shouldAutoPrepareNativeWheel` correctly refused a model-specific C294 command.
- A manual confirmation supplied the missing authorization, which is why the wheel then worked.

## D5.5 fix

Duplicate device records are now **merged, not discarded**. The PnP/topology record remains authoritative for service, INF and physical identity, while non-empty HID evidence enriches it:

- `HIDProduct`
- `HIDSerial`
- missing parent/container/location/stable/physical identity fields
- HID-derived supported model when the PnP model is only generic C294

After fusion, the existing safe session-only G27 consensus sees `G27 Racing Wheel`, sets `ModelConfirmed=true`, and the existing D5.4 auto-native path can queue the C294→C29B switch without manual confirmation.

## Safety boundary

D5.5 does **not** make every C294 a G27. Automatic model authorization still requires current, explicit HID model evidence (or the other already-approved evidence paths). A generic C294 product remains unconfirmed.

## Regression coverage

New D5.5 tests cover:

1. duplicate PnP/HID instance IDs retain HID product and serial metadata;
2. the merged C294 G27 record is current-session confirmed automatically;
3. the resulting logical state satisfies `shouldAutoPrepareNativeWheel` without a manual confirmation.

## Expected real-hardware result

Cold/power-cycled G27:

`C294 visible → direct HID product G27 Racing Wheel → evidence merged → session model confirmed → automatic D5.4 switch queued → C29B appears → native Direct HID binds`.
