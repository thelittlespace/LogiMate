# LogiMate 0.3.0-alpha — D0 Release Readiness

Date: 2026-09-12

## Software release gates

- PASS — host `go test ./...` (wheelengine, OpenG27-derived parity/reference package, brand package discovery).
- PASS — Windows x64 `internal/system` test binary compilation.
- PASS — Windows x64 `internal/app` test binary compilation.
- PASS — Windows x64 `go vet -unsafeptr=false ./...`.
- PASS — final Windows x64 `LogiMate.exe` build with version `0.3.0-alpha`.
- PASS — installer `go vet` with `installer` tag.
- PASS — final Windows x64 installer build with final application embedded.
- PASS — Windows ARM64 compile validation.
- PASS — D0 typed descriptor/session tests.
- PASS — D0 Native Engine core-gate test is included in the Windows test suite and built successfully.
- PASS — legacy `openg27-pino` adapter alias canonicalizes to `wreckfest2-pino` in regression coverage.

## Standalone Modern readiness

- PASS — normal Modern setup uses one LogiMate Native migration path for G25/G27/DFGT.
- PASS — external OpenG27 is not required or automatically launched by normal Modern setup.
- PASS — Logitech Profiler/LGS is not required by Modern mode.
- PASS — native input/calibration/profile/output/game adapter functionality is integrated into LogiMate.
- PASS — productive hardware output uses one output ownership/transport boundary.

## External gates not claimable in this environment

- PENDING — real G25 physical input/output certification.
- PENDING — real G27 D0 transport regression certification.
- PENDING — real Driving Force GT physical input/output certification.
- PENDING — USB removal/process kill/shutdown/suspend recovery matrix.
- PENDING — repeated/adverse HID I/O stress.
- PENDING — Modern↔Legacy↔Modern migration matrix on real Windows.
- PENDING — Windows UI/accessibility matrix.
- PENDING — Stable Authenticode signing.

## Release interpretation

`0.3.0-alpha` is a software-complete **D0 architecture/standalone preview**. It is not a Stable hardware-certified motor release. D1 must use real hardware evidence and refine model descriptors/adapters without creating new parallel engines.
