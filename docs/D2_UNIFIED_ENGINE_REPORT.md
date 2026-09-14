# D2 Unified Native Wheel Engine Report — 0.3.2-alpha

## Goal

D2 removes the remaining productive runtime dependence on the historical OpenG27-port layer while preserving provenance, import compatibility and an independent parity oracle.

## Result

**Software consolidation complete.** The normal Modern runtime for G25, G27 and Driving Force GT is LogiMate-owned from model identification through input decoding, protocol generation, FFB scheduling and supported Wreckfest/Pino telemetry.

External OpenG27 is not required. `internal/openg27port` remains in source for attribution, historical profile import and reference/parity tests; it is not the productive motor/input/game runtime.

## Runtime ownership after D2

```text
Windows SetupAPI / Raw Input / HID / WinMM
                  │
                  ▼
          internal/wheelengine
   ┌──────────────┼─────────────────┐
   │              │                 │
model adapters  native input   native protocol
   │              │                 │
   └──────── canonical state ────────┤
                                    │
                         FFB scheduler / Pino
                                    │
                                    ▼
                       centralized output lease
                                    │
                                    ▼
                         serialized HID transport
```

## Code moved behind the native boundary

- G25/G27/DFGT native report decoding → `wheelengine.ParseNativeInputReport`.
- Classic Logitech native report builders → `wheelengine` protocol builders.
- Force-frame resolution and FFB scheduler → `wheelengine`.
- Wreckfest/Pino packet parser and LED threshold semantics → `wheelengine`.
- Model descriptors/adapters/capabilities → `wheelengine`.
- Productive C6-equivalent game output → Native Game Output using the central lease/transport.

## What remains in `internal/openg27port`

These are intentional and **not runtime dependencies**:

- C1–C7 historical parity/oracle tests and diagnostics.
- Golden protocol comparisons.
- OpenG27 profile-schema parsing/import.
- Provenance/licensing reference implementation.

A static review of normal productive paths confirms that telemetry, Native Game Output and Direct-HID parsers no longer import the OpenG27-port package.

## Compatibility decisions

- Existing profile adapter ID `openg27-pino` is still accepted when reading data and normalized to `wreckfest2-pino`.
- Historical Fusion names/functions may remain as compatibility wrappers or diagnostics; they are not separate production engines.
- External OpenG27 process detection remains a conservative safety interlock during Alpha, even though OpenG27 is not required.
- Original Logitech LGS/Profiler remains the deliberately selected Legacy alternative.

## Known intentionally unresolved item

The old G27 LED builder and the reference oracle differ in one trailing byte. D2 does not silently rewrite physically unverified behavior. Hardware evidence from D1 certification decides the final canonical packet.

## Definition of done achieved by software

- One typed model/capability layer: PASS.
- One native input decoding layer: PASS.
- One productive game-FFB scheduler/protocol layer: PASS.
- One Wreckfest/Pino parser: PASS.
- One central output ownership/transport boundary: PASS.
- OpenG27-port absent from productive runtime: PASS.
- Reference parity retained with MIT attribution: PASS.
- Real hardware certification: PENDING and deliberately separate.
