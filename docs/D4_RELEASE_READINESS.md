# D4 Release Readiness — LogiMate 0.4.0-alpha

## Software status

**READY FOR ALPHA RELEASE**

D4 Advanced FFB is software-complete for the Alpha branch.

### Passed software gates

- platform-independent `wheelengine` tests, including D4 pipeline math;
- `gameadapter` tests;
- Windows x64 system test compilation;
- Windows x64 app test compilation;
- Windows x64 application build;
- Windows-target `go vet -unsafeptr=false` for system/app/main;
- D4 safety-order regression: advanced shaping cannot bypass the hard constant-force ceiling;
- D4 future-schema overwrite protection;
- D4 preset normalization/bounds checks.

## External gates still intentionally open

The following are **not** claimed from this build environment:

- real G25 motor/condition-effect behavior;
- real G27 regression against D4 OVERLAPPED `WriteFile` and compatibility fallback;
- real Driving Force GT motor/condition-effect behavior;
- USB yank during active FFB;
- process kill during pending output;
- suspend/resume during active output;
- repeated Modern ↔ Legacy hardware migration;
- Windows accessibility/manual UI certification;
- Stable Authenticode signing.

Stable packaging remains gated by `docs/HARDWARE_CERTIFICATION.json` and the existing release pipeline.

## D4 safety decision

D4 presets may alter the shape of semantic force, including optional minimum-force compensation, but they are always evaluated before `MixNativeEffects`. The hard per-wheel `NativeFFBConfig` remains the final motor ceiling.

## Next stage

**D5 — Modern-mode independence / historical dependency retirement**

D5 should remove or quarantine remaining historical OpenG27-facing user workflows that are no longer needed for normal Modern operation, while keeping license provenance, explicit profile import and test-oracle code where useful. Legacy Logitech/LGS remains a deliberately selectable fallback mode rather than a Modern dependency.
