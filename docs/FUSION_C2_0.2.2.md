# OpenG27 Fusion C2 — LogiMate 0.2.2-alpha

## Goal

Port OpenG27's pure `Lg4ffReports` command builders into the isolated Go parity
package and compare them byte-for-byte with LogiMate's pre-fusion output
builders without changing live hardware routing.

Upstream baseline remains:

- `Jabelius/OpenG27`
- version `1.0.4`
- commit `12c9421d2c9e3de05c3ff543ed7748f8d6162994`
- MIT license

## Ported in C2

`internal/openg27port/lg4ff.go` now contains hardware-independent translations
of:

- Constant Force
- global Stop
- G25/G27 rotation range
- G27 native-mode switch
- autocenter SpringSet / SpringEnable / SpringOff
- Damper / DamperOff
- Friction / FrictionOff
- G27 rev LEDs / LEDs off
- lg4ff cumulative LED bar thresholds
- Windows report-ID wrapper as a LogiMate helper

The translated functions themselves never open HID devices.

## Golden parity

The upstream `Lg4ffReportsTests.cs` vectors were translated into Go tests.
Representative expected reports include:

```text
Constant neutral : 11 08 80 80 00 00 00
Range 900°       : F8 81 84 03 00 00 00
G27 native switch: F8 09 04 01 00 00 00
Spring set       : FE 0D 07 07 80 00 00
Damper clamp     : 21 0C 0F 00 0F 00 80
Friction         : 21 0E 40 40 FF 00 00
LEDs all         : F8 12 1F 00 00 00 00
```

`go test ./internal/openg27port` executes these tests on the build host.

## Shadow comparison with existing LogiMate output

C2 deliberately does not silently replace pre-existing live output. Instead,
`FusionC2ProtocolComparisons()` compares the current LogiMate builders with the
ported OpenG27 reference and exports the results through:

- **Kalibrieren & Lernen → Engine Health**
- the normal LogiMate diagnostic report

At the C2 baseline the representative matrix is:

| Path | Result | C2 interpretation |
|---|---|---|
| Rotation 900° | MATCH | existing LogiMate range packet is already parity-compatible |
| G27 native switch | MATCH | model-specific switch packet matches; LogiMate still keeps its extra guarded preamble |
| G27 LED 0x1F | DIFF | old LogiMate report ends in `01`; OpenG27/lg4ff ends in `00` |
| Constant Force neutral/start | DIFF | existing LogiMate slot payload differs from OpenG27's `11 08 force 80 ...` layout |
| Autocenter SpringSet | MATCH | setup packet layout can be represented byte-identically |
| Autocenter SpringEnable sequence | DIFF | OpenG27 sends a second `14 00 ...` enable packet; old LogiMate path does not |
| Damper slot-1 layout | MATCH | wire layout matches after LogiMate's safety mapping chooses coeff/clip |
| Friction slot-1 layout | MATCH | wire layout matches after LogiMate's safety mapping chooses coeff/clip |
| Condition slot-1 stop | MATCH | existing slot stop matches OpenG27 Damper/Friction stop |

These differences are not treated as failures in C2 because changing motor
routing is intentionally deferred to the next hardware-gated stage.

## Safety contract

C2 MUST NOT:

- change which report builder drives a real wheel;
- remove LogiMate's existing force limits, watchdog or owner-generation model;
- automatically enable native output;
- send a parity/reference report merely because the reference differs.

C2 MAY:

- compute reference packets;
- compare packets in memory;
- expose differences in diagnostics;
- fail a build when an OpenG27 golden vector no longer matches the port.

## Gate to C3

C3 may begin only when:

1. the C2 pure Go golden tests pass;
2. Windows app/system test packages compile;
3. the user confirms 0.2.2 has not regressed wheel detection/input;
4. C3 introduces an explicit routing strategy rather than replacing every
   pre-existing packet builder at once.

C3 will port the OpenG27 scheduling model and introduce a selectable/shadowed
reference-output path while preserving LogiMate's stricter safety envelope.
