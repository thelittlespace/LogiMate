# OpenG27 provenance ledger

LogiMate's long-term migration follows **Path C → Path D**:

1. port hardware-independent OpenG27 Core behavior to native Go with explicit provenance and parity tests;
2. port/compare protocol and scheduler behavior behind shadow/dry-run gates;
3. once parity is proven, consolidate and improve the architecture into LogiMate-native implementations.

## Upstream baseline

- Project: `Jabelius/OpenG27`
- Upstream commit: `12c9421d2c9e3de05c3ff543ed7748f8d6162994`
- Upstream version at that commit: `1.0.4`
- License: MIT
- Copyright: `(c) 2026 jabelius`

## C1 — pure Core port

| OpenG27 source | Blob SHA | LogiMate target | Classification | Hardware impact |
|---|---|---|---|---|
| `src/App/Logic/ForceScaling.cs` | `d45e8c25c8743073640135a256e793d963170659` | `internal/openg27port/core.go` | translated/derived | none |
| `src/Core/ForceFrame.cs` | `f7894aeb5d26c9bef8c0741ab6f7aa8a680faf46` | `internal/openg27port/core.go` | translated/derived | none |
| `src/Core/ForceMixer.cs` | `10be321d7e174cf769225a75451fde459bbc94f9` | `internal/openg27port/core.go` | translated/derived | none |
| `src/Core/AxisCalibration.cs` | `ef64b7e7f15ffba3456f32cff660ac824981f6cc` | `internal/openg27port/core.go` | translated/derived | none |
| `src/Core/PedalCalibration.cs` | `97bcdc00bd59d28c7130ad1eca2af5a6939ddf53` | `internal/openg27port/core.go` | translated/derived | none |
| `src/Core/RangeTracker.cs` | `dbe580fb1e4540d7c708dfa6dfc82bd94d1b77cd` | `internal/openg27port/core.go` | translated/derived | none |

## C2 — lg4ff report parity

| OpenG27 source | Blob SHA | LogiMate target | Classification | Hardware impact |
|---|---|---|---|---|
| `src/Core/Lg4ffReports.cs` | `7e0aad168eaefaa68de8e99492cd680ce39cca0b` | `internal/openg27port/lg4ff.go` | translated/derived | none in C2; shadow only |
| `tests/Core.Tests/Lg4ffReportsTests.cs` | `07be7bfa61f600ba722ca6c40bb64592e209612d` | `internal/openg27port/lg4ff_test.go` | translated golden vectors | none |

C2 intentionally keeps the translated report builders disconnected from live HID routing. `internal/system/fusion_c2_windows.go` compares the existing LogiMate production builders with the OpenG27-derived reference bytes and exposes differences through diagnostics.

### Porting rules used in C1

- No WPF/UI code is copied.
- No OpenG27 persistence layout is adopted.
- No translated code can send HID output in C1.
- Tests use golden behavior vectors derived from the upstream implementation.
- C# rounding semantics are preserved explicitly: the port uses midpoint-to-even where OpenG27 relies on `Math.Round`.
- Existing LogiMate calibration/FFB code is not replaced yet. C1 runs as a parity/reference layer until later fusion gates promote it.


## C3 — FFB scheduler / WheelOutput parity

| OpenG27 source | Blob SHA | LogiMate target | Classification | Hardware impact |
|---|---|---|---|---|
| `src/Core/FfbEngine.cs` | `d53b4d14897c9550b84402e0734beb86eeb32546` | `internal/openg27port/scheduler.go` | translated/derived | live only through explicit G27 C3 test gate |
| `src/Core/WheelOutput.cs` | `5eedfeadba1343a31c8286c8e1ea03e6a5550ca1` | `internal/openg27port/scheduler.go` | translated/derived | live only through explicit G27 C3 test gate |
| `src/Core/WheelOutputOptions.cs` | `d6aed106759d42a6a12e5e7a7bebdd957f23bebd` | `internal/openg27port/scheduler.go` | translated/derived | none by itself |
| `src/Core/IFfbSource.cs` | `a1509c8663902af1507cfda9bccf4489e494a0af` | `internal/openg27port/scheduler.go` | translated/derived | none by itself |
| `tests/Core.Tests/FfbEngineTests.cs` | `4f5440991006d188b8bef05fae42a2e79fa5ffac` | `internal/openg27port/scheduler_test.go` | translated behavioral vectors | none |

C3 keeps OpenG27's parity-first scheduler semantics (`1000 / 150` integer milliseconds -> 6 ms) and `PumpOnce` behavior. LogiMate deliberately layers stricter policy above the port: stable-wheel ownership, explicit G27-only opt-in, ±5% live cap, converted slew-rate budget, crash marker, OpenG27 interlock, device-change/shutdown cancellation and a 900 ms outer hard stop. The existing LogiMate FFB implementation remains available for A/B comparison and is not silently replaced.

## C4 — G27 report parser / device lifecycle parity

| OpenG27 source | Blob SHA | LogiMate target | Classification | Hardware impact |
|---|---|---|---|---|
| `src/Core/G27ReportParser.cs` | `2f8562969bf09e7922c2b3692d2bd9e309444136` | `internal/openg27port/g27_parser.go` | translated/derived | none; read-only shadow |
| `tests/Core.Tests/G27ReportParserTests.cs` | `2134caa656f4cf9c3cf46adfc6351f34a3935f7d` | `internal/openg27port/g27_parser_test.go` | translated golden vectors | none |
| `src/Core/G27Device.cs` | `852b45b5eacaf1ebcb2faa4aafce672ec40c5cd9` | `internal/openg27port/g27_device_lifecycle.go` + existing LogiMate Direct-HID lifecycle | reference/translated policy | no new output |

C4 does **not** copy HidSharp/WPF infrastructure. OpenG27's lifecycle rule (prefer
already-native C29B; switch C294 only when native is absent) is modeled as a
pure plan and compared against LogiMate. LogiMate deliberately retains its own
Win32 shared-HID reader, StableWheelID, exact PnP/Raw-Input correlation,
reconnect backoff and fail-closed multi-wheel selection.

## Planned next entries

- **C3:** implemented in 0.2.4-alpha; ported scheduler/WheelOutput with dry-run plus G27-only bounded live validation.
- **C4:** implemented in 0.2.5-alpha; parser shadow parity + G27 lifecycle-policy comparison, with LogiMate StableWheelID remaining authoritative.
- **C5:** implemented in 0.2.6-alpha; GameProfile/import parity and safe translation.
- **C6:** implemented in 0.2.7-alpha; Wreckfest 2 Pino, RPM LEDs and explicit telemetry-driven constant-force adapter.
- **C7:** external OpenG27 becomes fallback-only after the G27 physical parity matrix is green.
- **D1+**: progressively replace translated modules with generalized LogiMate-native implementations for G25/G27/DFGT and future wheels.


## C5 — GameProfile / import parity

| OpenG27 source | Blob SHA | LogiMate target | Classification |
|---|---|---|---|
| `src/Core/GameProfile.cs` | `cf0966a548a9b3cf1c32dcc9136429bf0aada26c` | `internal/openg27port/game_profile.go` | translated/derived |
| `src/Core/GameProfilePresets.cs` | `d55cf0870a85c4333db30099cea8d0f5a6a56069` | preset parity + translator | translated/derived |
| `src/Core/GameProfileStore.cs` | `0f97d95ab46509e61049b7b95606f8a6f4ee9e26` | `internal/system/game_profiles_windows.go` | schema/path reference + translated import |
| `src/Core/GameDetector.cs` | `d07d55601a095159d75a770c0444f2d4072175ea` | existing LogiMate GameSession polling | behavioral reference |
| `tests/Core.Tests/GameProfileTests.cs` | `4ac2d22ae193f5ae594a4965a31f68877dba24c9` | `internal/openg27port/game_profile_test.go` | translated vectors |

## C6 — Wreckfest 2 Pino

| OpenG27 source | Blob SHA | LogiMate target | Classification |
|---|---|---|---|
| `src/Core/PinoTelemetry.cs` | `e4e2777cc0dc4af06e26adf2fb5d0cc4aea2f17a` | `internal/openg27port/pino.go` | translated/derived |
| `src/Core/Wf2Telemetry.cs` | `2b047a22e7125369a087b527fe91130c41d66839` | `internal/openg27port/pino.go` | translated/derived |
| `src/Core/TelemetryListener.cs` | `dcd3598d0dae4265b7436c51dd1fd2e276e0c78a` | `internal/system/telemetry_windows.go` | translated policy integrated into common hub |
| `src/Core/RpmLedSource.cs` | `483d16f007a4179f2bc2fff03e70f0928aedc7e1` | GameProfile LED policy + C6 output | behavioral reference |
| `tests/Core.Tests/PinoTelemetryTests.cs` | `d5733085f1f8c8d4d326986bb655dbb1c17e679e` | `internal/openg27port/pino_test.go` | translated golden field vectors |
| `tests/Core.Tests/fixtures/wf2-main-packet.hex` | `040883b4ae9a592dc40e3c74b79ebd084d6205df` | field expectations documented/tested with deterministic packet | reference fixture |

C6 deliberately does not copy WPF UI. The listener is loopback-only and the motor path remains opt-in and bounded by LogiMate safety policy.

## C7 cutover note — 0.2.8-alpha

C7 does not introduce another copied OpenG27 implementation module. It promotes the already documented C1–C6 translated/derived components into the normal G27 Modern runtime behind LogiMate safety and parity gates.

- No OpenG27 WPF/UI code is embedded.
- The pinned upstream reference remains OpenG27 1.0.4 / commit `12c9421d2c9e3de05c3ff543ed7748f8d6162994`.
- External OpenG27 remains an optional fallback/A-B oracle and import source.
- Existing compatibility reading of OpenG27 configuration is not treated as ownership transfer; Path D will consolidate duplicate persistence into LogiMate-native StableWheelID storage.
- Copyright and MIT attribution remain in `THIRD_PARTY_NOTICES.md`.
