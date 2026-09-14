# D3 Adapter Ecosystem Report — 0.3.3-alpha

## Goal

Make games and telemetry extensible without allowing game-specific code to own Logitech wheel hardware. D3 therefore creates a semantic adapter boundary above the D0/D2 Native Wheel Engine.

## Implemented architecture

`internal/gameadapter` now owns a typed adapter contract with:

- stable adapter ID and descriptor;
- declared semantic capabilities;
- deterministic start/stop lifecycle;
- health, packet/frame counters and errors;
- normalized frame snapshots and a bounded recent-frame ring;
- loopback-only UDP transport for the two current reference adapters.

Reference implementations:

1. `wreckfest2-pino` — Wreckfest 2 / Pino, automatic lifecycle, default UDP 23123;
2. `logimate-json` — generic normalized LogiMate JSON, manual lifecycle, default UDP 27100.

`openg27-pino` is only a migration alias and resolves to `wreckfest2-pino`.

## Safety boundary

Adapters publish semantic data only. They do not open the Logitech HID device, create force reports, acquire output ownership or bypass recovery. Native Game Output checks adapter capabilities and then routes output through the existing profile mixer, central output lease, recovery marker and serialized native HID transport.

D3 adds an architecture regression test that scans adapter source for direct HID/output primitives. The Go package boundary also prevents an adapter implementation from importing the Windows system package without creating an import cycle.

## Game profile schema

`game-profiles.json` is schema 2. It adds adapter port and priority fields while preserving old profiles. A future unknown schema is rejected for writes/import instead of being silently downgraded.

## Diagnostics

Telemetry status now exposes packet/frame rates, last-frame age, stale state and last error. The UI contains an adapter catalog with capabilities and setup instructions.

## Runtime result

Native Game Output no longer tests for Pino as a prerequisite. It consumes any registered adapter that provides the required semantic capabilities. Wheel-specific LEDs remain capability-gated, and motor force still requires the same central motor lease as all other Native output.

## Not claimed

D3 does not claim physical motor certification. G25/G27/DFGT hardware evidence remains a Stable release gate. Additional games should be added in later D3.x work only as new adapter implementations plus fixtures; they must not change the motor core unless a genuine engine defect is demonstrated.
