# LogiMate D4 — Advanced FFB Report

Version: **0.4.0-alpha**

D4 adds signal shaping and diagnostics on top of the D0–D3 single-output architecture. It does **not** move safety limits into game adapters and it does **not** allow any adapter to write HID directly.

## Pipeline order

```text
Game adapter semantic force
        ↓
Per-game gain / invert
        ↓
D4 Advanced FFB pipeline
  - constant/transient gain
  - deadband + range rescale
  - response exponent
  - optional minimum force
  - 1-pole low-pass
  - optional exponential smoothing
  - normalized pre-safety output limit
        ↓
Native engine profile gains
        ↓
Hard per-wheel NativeFFBConfig safety mixer
        ↓
Slew/watchdog/output ownership
        ↓
Serialized HID transport
```

The order is deliberate: minimum-force and response shaping happen **before** the hard per-wheel safety ceiling. D4 therefore cannot raise the final motor limit above `NativeFFBConfig`.

## New pure FFB core

`internal/wheelengine/ffb_pipeline.go` is platform independent and covered by host tests. It provides:

- symmetric deadband with full-range rescaling;
- minimum-force compensation;
- response curves using bounded exponents;
- dynamic first-order low-pass filtering;
- additional exponential smoothing;
- separate constant/transient semantic gains;
- pre-safety normalized output limit;
- resettable state;
- input/output clipping counters;
- deadband/minimum-force counters;
- average/max semantic sample latency.

## Presets

D4 ships code-safe tuning presets for G25, G27 and Driving Force GT:

- **Neutral** — no force shaping beyond diagnostics;
- **Smooth** — 35 Hz low-pass + 12% smoothing, no force boost;
- **Responsive** — 60 Hz low-pass + 4% smoothing and a mild response curve;
- **Compensated (Experimental)** — 1% deadband + 3% minimum force, still before the hard safety cap;
- **Disabled** — retains the D3 force behavior.

These presets are software presets, not a claim of physically optimal tuning. Hardware-specific tuning remains part of the real-wheel certification matrix.

## Per-effect tuning

Wheel Engine custom profiles now allow individual Constant, Spring, Damper and Friction gains. These gains remain upstream of the independent hard safety profile.

D4 also keeps separate constant/transient shaping in the semantic game-force pipeline, ready for future adapters that publish transient force components.

## Diagnostics

Native Game Output now reports:

- raw semantic force;
- shaped force;
- final applied constant-force percent;
- pipeline clip count;
- hard safety-mixer clip count;
- deadband hits;
- minimum-force applications;
- average and maximum adapter-to-pipeline sample latency;
- scheduler pumps;
- HID write count;
- active HID backend.

This separates three common causes of weak/harsh FFB: source clipping, shaping, and hard safety limiting.

## D4 HID transport

The preferred Windows output backend now opens the selected HID interface with `FILE_FLAG_OVERLAPPED` and uses a real `WriteFile` OVERLAPPED operation. A timeout targets the exact pending operation with `CancelIoEx` and poisons the handle fail-closed because delivery to hardware can no longer be proven.

For classic HID stacks that synchronously reject OVERLAPPED `WriteFile` before any operation is queued, LogiMate may reopen the same selected path using the older `HidD_SetOutputReport` compatibility backend. That fallback is still deadline-bounded and poisons/closes its handle on an ambiguous timeout.

The backend is surfaced in D4 diagnostics. Physical G25/G27/DFGT validation is still required before Stable certification of the preferred transport/fallback matrix.

## Persistence

Advanced tuning is stored per physical wheel in `advanced-ffb.json` with schema versioning and future-schema overwrite protection.

## Safety invariants retained

D4 does not change these D0–D3 rules:

1. exactly one selected/actionable wheel owns output;
2. game adapters never own wheel HID;
3. motor output requires the central output lease and recovery marker;
4. hard safety ceilings are applied after all D4 shaping;
5. ambiguous or timed-out I/O fails closed;
6. Stable remains blocked until real hardware evidence exists for G25, G27 and DFGT.
