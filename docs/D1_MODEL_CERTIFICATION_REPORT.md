# D1 Model Certification & Refinement Report — 0.3.1-alpha

## Result

D1 is **software-complete, hardware-certification pending**. LogiMate now has explicit model adapters for Logitech G25, G27 and Driving Force GT, and model-specific input layouts are exercised by platform-independent fixtures. No wheel is marked physically certified merely because the code builds or a synthetic fixture passes.

## Software refinements completed

- Central model adapters define native PID, C294 selector, input layout ID, minimum native report length and capability set.
- G25/G27/DFGT native HID decoding moved into `internal/wheelengine/native_input.go`.
- Windows input code now performs I/O/session/calibration projection rather than duplicating each model's byte layout.
- G27 Direct-HID samples are explicitly tagged `direct-hid` and `g27-c29b-v1`; valid native samples no longer inherit WinMM metadata.
- G25 and DFGT use their adapter-owned layout IDs.
- A certification contract requires passed evidence for every required physical test before a model can be considered complete.
- Stable packaging requires per-model certification evidence in `docs/HARDWARE_CERTIFICATION.json`.

## Model matrix

| Model | Native PID | Adapter | Native input fixtures | Physical certification |
|---|---|---|---|---|
| G25 | C299 | `g25-c299-v1` | PASS | PENDING |
| G27 | C29B | `g27-c29b-v1` | PASS | PENDING D0/D2 revalidation |
| Driving Force GT | C29A | `dfgt-c29a-v1` | PASS | PENDING |

## Required physical evidence per model

Each applicable item must have reproducible evidence, not just a checkbox:

1. C294 → native mode transition and correct re-enumeration.
2. Steering full range/center and calibration persistence.
3. Pedals and inversion/range behavior.
4. Buttons, D-pad, paddles and model-specific controls.
5. H-shifter/sequential behavior where applicable.
6. Rotation-range command.
7. Constant force.
8. Spring.
9. Damper.
10. Friction.
11. Autocenter.
12. Game FFB through Native Game Output.
13. Reconnect without stale input/output state.
14. USB removal during output.
15. Process kill during output and recovery on next launch.
16. Suspend/resume.
17. Modern → Legacy → Modern rollback cycle.

## Important unresolved hardware questions

- The G27 historical LED packet has a known last-byte difference between older LogiMate code and the OpenG27/lg4ff oracle. D1/D2 deliberately does **not** guess which variant should be promoted without a physical capture/test.
- Synchronous HID compatibility behavior must be stress-tested on all three wheel models; transport safety remains fail-closed if completion is not proven.
- G25/DFGT effect semantics are implemented from the shared classic Logitech protocol/capabilities but remain Experimental until real motor-direction/effect tests pass.

## Promotion rule

A discovered hardware difference is fixed only by updating the model adapter/capability/protocol implementation and adding a regression fixture. Do not add one-off UI branches that bypass the engine.
