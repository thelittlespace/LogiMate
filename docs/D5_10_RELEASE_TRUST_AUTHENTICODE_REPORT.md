# D5.10 Release Trust / Authenticode Hardening

**Release:** 0.5.10-alpha  
**Purpose:** turn the final Stable signing requirement into an enforced build/runtime contract instead of a prose-only release note.

## What changed

### Runtime trust visibility

LogiMate now inspects the Authenticode identity of the exact running executable and exposes it in:

- `Kalibrieren & Lernen -> D5.10 Release Trust / Authenticode`;
- Engine Health;
- the diagnostic report.

The runtime view reports signature validity/status, publisher subject and certificate thumbprint. Pre-release versions may be unsigned, but the same state becomes a hard blocker when `VERSION` is a Stable version without a prerelease suffix.

### Stable build signing contract

`build.ps1` now treats Stable signing as part of the build, not a post-build suggestion:

1. Stable certification manifest must match the exact `VERSION` and all hardware/recovery/migration/UI/HID-stress gates must already be true.
2. `LogiMate.exe` is built/tested first.
3. The exact application payload is Authenticode-signed with SHA-256 + RFC3161 timestamp and verified.
4. Only that already-signed application is copied into the installer payload.
5. `LogiMate-Setup-x64.exe` is built from the signed payload.
6. The final installer is Authenticode-signed and verified.
7. App and installer must carry the configured publisher certificate thumbprint and therefore the same publisher identity.
8. A machine-readable `RELEASE_ATTESTATION.json` records hashes, Authenticode state, publisher identity and certification flags.
9. Portable packaging happens only after the final signed application exists, so Stable portable and installed payloads use the same signed executable.

Stable signing configuration is explicit:

- `LOGIMATE_SIGN_THUMBPRINT` — certificate thumbprint available to Windows SignTool;
- `LOGIMATE_TIMESTAMP_URL` — RFC3161 timestamp service URL;
- Windows SDK `signtool.exe` must be installed.

No certificate, PFX, password or signing secret is stored in the repository or release package.

## Fail-closed behavior

A Stable build aborts when any of the following is true:

- certification release does not exactly match `VERSION`;
- any global Stable certification flag is false;
- G25/G27/DFGT physical certification/evidence is incomplete;
- signing thumbprint is missing;
- timestamp URL is missing;
- SignTool is unavailable;
- application signing or verification fails;
- installer signing or verification fails;
- the actual certificate thumbprint differs from the configured thumbprint;
- app and installer publisher identities differ;
- the generated release attestation does not satisfy the Stable gate.

Alpha/beta/RC builds may remain unsigned so development is not coupled to possession of the private release certificate. They are explicitly reported as prerelease/unsigned and cannot satisfy the Stable gate.

## Security boundary

D5.10 does not create or manage a code-signing private key. It only consumes an already-provisioned Windows signing identity at release time. Private-key custody, certificate issuance/revocation and CI secret-store policy remain external release-operations responsibilities.

## Current truth

The 0.5.10-alpha artifact produced in the audit environment is intentionally unsigned because no private Authenticode identity is present. Therefore D5.10 software enforcement is complete, but Stable remains blocked until the physical/accessibility/HID evidence is reviewed and an actual Stable release is signed on the trusted Windows release host.
