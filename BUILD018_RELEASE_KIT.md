# LogiMate 0.0.1-alpha · Build 018 — Go 1.27.1 Release Kit

This package is the hardened Build 018 source and reproducible Windows release builder. The visible application version remains **0.0.1-alpha** and the internal build ID is **018**.

## Pinned release toolchain

- Go **1.27.1** (exact)
- PowerShell **7.6.6** (exact release shell)
- GitHub Actions are full-SHA pinned in `.github/workflows/`

`Build-Release.cmd` is the normal Windows entry point. It invokes `Build-Release-Go1271.ps1`, downloads the official portable Windows toolchains, verifies their published SHA-256 values, and does not replace machine-wide Go/PowerShell installations.

Pinned upstream archive hashes:

- Go `go1.27.1.windows-amd64.zip` — `a3911b5e0e1b1053f25ed0675f4c1c6aad1e2bfcf253df2b9be4caabd2edd95d`
- PowerShell `PowerShell-7.6.6-win-x64.zip` — `02fe458be20493fbdf43f61ea20610b811ee6c738ab1676c61b9cfcd1a33c860`

## Build on Windows

1. Extract this release kit to a normal writable folder.
2. Run `Build-Release.cmd`.
3. The bootstrap verifies source identity (`0.0.1-alpha`, Build `018`) **and every frozen source file against `SOURCE_MANIFEST_SHA256.txt`**, then verifies the exact toolchain hashes/versions.
4. Formatting, vet and unit gates run before release compilation.
5. `build.ps1` builds application, installer, ARM64 validation binary and release packages.
6. The build runs a second time and app/installer must be byte-identical.
7. `LogiMate-Source.zip` is extracted and independently rebuilt; those app/installer hashes must match the release binaries.
8. `RELEASE_ATTESTATION.json` must state `go1.27.1` and PowerShell `7.6.6`.
9. On success the script creates `LogiMate-0.0.1-alpha-Build018-Go1.27.1-Release.zip`.

Build 018 remains an **alpha/pre-release** and is deliberately not represented as a stable hardware certification. The verified release bootstrap clears ambient signing variables for deterministic unsigned alpha output; stable signing remains a separate future gate.

## GitHub publication gate

The `release-candidate.yml` workflow requires Linux Go-1.27.1 unit/race gates and a Go-1.27.1 CodeQL job before the Windows release job can start. The Windows job then uses this same hash-verified bootstrap and GitHub attests the installer, portable archive and complete release bundle. Tag identity is `v0.0.1-alpha.018`.

## Environment limitation of the chat build runner

The current chat execution sandbox has Go 1.23.2, no Windows runtime, and cannot download binary toolchains from the public internet. Therefore this package intentionally contains **no falsely relabeled Go-1.23 executable**. A binary may be called the Build 018 Go-1.27.1 release only after `Build-Release.cmd` or the equivalent GitHub Windows workflow succeeds and the generated attestation records the exact toolchain.

## Repository publication

`Publish-Repository.cmd` can create `thelittlespace/LogiMate` through GitHub CLI, push `main`, and push `v0.0.1-alpha.018`. The tag-triggered workflow publishes the fully gated asset set as a GitHub prerelease.
