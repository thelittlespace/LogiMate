# GitHub Publication Checklist

## Before making the repository public

- [ ] Run secret scan over the full Git history / source bundle.
- [ ] Ensure no `.pfx`, private key, token, personal diagnostic archive or local path is committed.
- [ ] Confirm LICENSE, THIRD_PARTY_NOTICES and OPENG27_PROVENANCE are accurate.
- [ ] CI passes on Linux, Windows amd64 and Windows arm64 compile gate.
- [ ] Enable CodeQL default setup.
- [ ] Enable Private vulnerability reporting.
- [ ] Enable secret scanning / push protection where available.
- [ ] Configure a `main` ruleset with required CI checks and blocked force-push/deletion.
- [ ] Protect release tags (`v*`).
- [ ] Keep GitHub Actions default token permissions read-only; grant write only in a dedicated release job.

## First public release

- Tag as a **pre-release**, not Stable.
- Use tag form `v0.0.1-alpha.<build>` while the visible app version remains `0.0.1-alpha`.
- Attach installer, portable ZIP, source archive if desired, SHA256SUMS and release attestation.
- Clearly list hardware validation status and known limitations.
- Do not claim G25/DFGT stable support until the physical certification matrix is complete.

## Stable later

- sign application and installer with the same trusted publisher identity and RFC3161 timestamp
- complete physical G25/G27/DFGT certification
- complete USB-yank/crash/suspend/recovery and Modern↔Legacy migration tests
- complete accessibility / DPI / High Contrast matrix
- run HID stress/adverse I/O evidence
- publish only when StableReleaseGate is fully satisfied
