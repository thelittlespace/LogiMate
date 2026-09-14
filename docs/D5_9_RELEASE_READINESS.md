# D5.9 Release Readiness

**Release:** 0.5.9-alpha  
**State:** HID stress software tooling complete; physical Stable evidence pending.

## Software acceptance

- Native HID transport exposes lifecycle/error/latency counters.
- Partial writes fail closed and poison/close the transport.
- Failed OVERLAPPED completion confirmation fails closed and poison/closes the transport.
- Compatibility fallback remains limited to three synchronous not-queued error classes.
- D5.9 deterministic policy stress is included in Engine Health.
- Guided 100-cycle non-motor live HID stress exists.
- Guided USB-yank adverse-I/O test exists.
- Physical evidence exports to Markdown + JSON under `LogiMateData\\Certification`.
- Live tests use the existing central output lease and never start a motor effect.
- Stable runtime gate now explicitly includes HID-stress validation.
- Production runtime remains independent of `internal/openg27port`.

## Still pending before Stable

- Run D5.9 normal + USB-yank/reconnect evidence on the real G27.
- Repeat required adverse-I/O evidence on real G25 and DFGT hardware.
- Complete the D5.7 per-model physical matrix.
- Complete D5.8 Narrator/UIA, High Contrast, reduced-motion and mixed-DPI validation.
- Complete Authenticode signing/verification.

`docs/HARDWARE_CERTIFICATION.json` therefore keeps `hidStressValidated=false`.
