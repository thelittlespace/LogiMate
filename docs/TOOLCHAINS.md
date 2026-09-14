# Toolchains — Build 018

Verified on 2026-09-14 against official upstream release information. Only stable releases are selected.

| Component | Build 018 baseline | Role |
|---|---:|---|
| Go | **1.27.1** | application and installer compiler |
| PowerShell | **7.6.6 LTS (exact)** | Windows release/build shell |
| actions/checkout | **v7.0.1** | CI checkout, full-SHA pinned |
| actions/setup-go | **v7.0.0** | CI Go provisioning, full-SHA pinned |
| actions/upload-artifact | **v7.0.1** | CI artifact upload, full-SHA pinned |
| github/codeql-action | **v4.37.5** | CodeQL workflow, full-SHA pinned |
| actions/attest-build-provenance | **v4.2.2** | release provenance, full-SHA pinned |

`build.ps1` refuses a release build unless the installed Go patch version exactly matches `go.mod` **and** the release shell is exactly PowerShell 7.6.6. This prevents previews or a later, not-yet-audited compiler/shell from silently changing Build 018 outputs. Windows PowerShell 5.1 remains an operating-system component used by selected runtime integration helpers where Windows ships it; it is not the release compiler/build shell.

The Windows SDK signing tools are only a toolchain dependency once signing is configured. Build 018 remains a pre-release and does not claim a signing gate PASS without a real certificate.


## Build 018 verified bootstrap

`Build-Release.cmd` launches `Build-Release-Go1271.ps1`, which downloads the official portable Windows archives without changing machine-wide installations. It verifies the upstream SHA-256 before extraction:

- `go1.27.1.windows-amd64.zip`: `a3911b5e0e1b1053f25ed0675f4c1c6aad1e2bfcf253df2b9be4caabd2edd95d`
- `PowerShell-7.6.6-win-x64.zip`: `02fe458be20493fbdf43f61ea20610b811ee6c738ab1676c61b9cfcd1a33c860`

The bootstrap sets `GOTOOLCHAIN=local`, puts the verified portable Go first on `PATH`, validates `go version`, runs formatting/vet/tests, executes the normal release pipeline twice, requires byte-identical application and installer outputs, and finally verifies that `RELEASE_ATTESTATION.json` recorded the real Go and PowerShell versions.
