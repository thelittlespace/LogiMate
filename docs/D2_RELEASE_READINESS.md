# D2 Release Readiness — 0.3.2-alpha

## Classification

**Alpha / software-ready for D3 development. Not Stable-certified for physical motor use.**

## Software gates

The D2 release must pass before packaging:

- host `wheelengine` tests including D1 model fixtures and D2 reference parity;
- historical `openg27port` parity tests;
- Windows amd64 `go vet`;
- Windows system test compile;
- Windows app test compile;
- final Windows x64 application build;
- installer vet/build using the exact final application payload;
- Windows arm64 compile validation;
- GitHub workflow YAML and certification JSON parsing;
- ZIP integrity and SHA-256 manifest verification.

## Hardware gates still open

See `D1_MODEL_CERTIFICATION_REPORT.md` and `HARDWARE_CERTIFICATION.md`. In particular, no G25/G27/DFGT model is promoted to Stable until real input/output, crash/recovery, USB removal, suspend/resume and migration evidence exists.

## Runtime independence statement

For Modern mode, D2 needs neither external OpenG27 nor Logitech LGS/Profiler. Legacy remains available intentionally for users who want the original Logitech stack. Historical OpenG27 import/parity source remains bundled with attribution but is not a productive runtime dependency.

## Next development checkpoint

**D3 — Adapter ecosystem.** Add a stable telemetry/game-adapter contract, per-adapter health/diagnostics and additional games without modifying the wheel/motor core.
