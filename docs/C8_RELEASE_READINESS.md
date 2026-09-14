# LogiMate 0.2.9-alpha — C8 release readiness

## Software state

C8 closes the C7 software BLOCK list and is intended to be packaged as an **alpha test release**. It is not a Stable hardware certification.

Software gates required before packaging:

- Windows/amd64 application build
- Windows/amd64 `go vet`
- Windows system/app test compilation
- hardware-independent OpenG27-port tests
- installer vet/build with final application payload
- Windows/arm64 compile validation
- GitHub workflow YAML parse
- clean source packaging without embedded temporary installer payload
- ZIP structural validation and SHA-256 manifest validation

## Stable gates intentionally still false

- physical G25/G27/DFGT matrix
- crash/process-kill/USB-yank/suspend recovery evidence
- Modern↔Legacy/HVCI migration evidence on real Windows
- HID I/O stress evidence
- full Windows UI/accessibility validation
- final Authenticode signing and verification

These are enforced through `HARDWARE_CERTIFICATION.json` and the Stable release workflow.

## Known architecture debt carried to D0

- `internal/system` remains broad and needs decomposition.
- model/mode strings remain at compatibility/UI boundaries despite the new typed capabilities layer.
- production output is serialized by the central lease, but a single cancellable hardware command queue/overlapped transport is still a D0/D4 goal.

## Final software validation result

**PASS (cross-build / hardware-independent scope).** On the release source:

- hardware-independent `internal/openg27port` tests passed,
- Windows/amd64 `go vet -unsafeptr=false ./...` passed,
- Windows/amd64 system and app test binaries compiled,
- final Windows/amd64 application compiled,
- installer vet/build with the final application payload passed,
- Windows/arm64 application compile validation passed,
- GitHub build/release workflow YAML parsed successfully,
- hardware-certification JSON parsed successfully,
- temporary installer payload was removed from the source tree before packaging.

This Linux build environment does not execute Windows PE binaries, so the Windows `--smoke-test`, full Windows `go test ./...`, Authenticode signing and physical-wheel tests are intentionally left to Windows CI / the external certification gate.

## Packaging result

The release root contains `PACKAGE_VERIFICATION.txt` and `SHA256SUMS.txt`; those are generated after the final artifacts are packed and provide the non-circular package/checksum evidence.
