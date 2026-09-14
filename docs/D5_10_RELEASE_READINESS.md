# D5.10 Release Readiness

**Release:** 0.5.10-alpha  
**State:** Authenticode/Stable release enforcement software-complete; external signing and remaining physical evidence pending.

## Software acceptance

- Running executable Authenticode status is visible in LogiMate.
- Pre-release unsigned state is informational, never misreported as Stable trust.
- Stable version numbers require an exact matching certification manifest.
- Stable requires all global hardware/recovery/migration/UIA/HID-stress gates.
- Stable requires per-model G25/G27/DFGT physical certification with evidence.
- Stable requires explicit publisher thumbprint + RFC3161 timestamp configuration.
- Application is signed and verified before being embedded in the installer.
- Installer is signed and verified after its final build.
- App and installer must match the configured publisher thumbprint.
- `RELEASE_ATTESTATION.json` records exact SHA-256 hashes and release trust state.
- Portable packaging consumes the final application payload.
- Production runtime remains independent of `internal/openg27port`.

## Current alpha boundary

`0.5.10-alpha` is intentionally allowed to be unsigned. This is not a Stable PASS. The release attestation must identify the build as prerelease and signature-required=false.

## Still pending before 1.0 Stable

- D5.7 reviewed physical G25/G27/DFGT certification matrix.
- crash/USB-yank/suspend and migration evidence promotion.
- D5.8 real Windows Narrator/UIA + High Contrast + reduced-motion + mixed-DPI validation.
- D5.9 real hardware adverse-I/O/HID-stress promotion.
- provision the trusted release certificate on the Windows release host.
- execute the Stable build with `LOGIMATE_SIGN_THUMBPRINT` and `LOGIMATE_TIMESTAMP_URL`.
- independently verify final Stable app/installer signatures and attestation before publication.
