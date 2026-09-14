# LogiMate repository setup — Build 018

Target repository: `thelittlespace/LogiMate`  
Default branch: `main`  
Visible application version: `0.0.1-alpha`  
Internal build: `018`  
Release tag: `v0.0.1-alpha.018`

## One-click path on Windows

1. Extract `LogiMate-Repository-Import.zip`.
2. Ensure Git and GitHub CLI (`gh`) are installed.
3. Run `Publish-Repository.cmd`.

If the stored GitHub CLI token is expired or invalid, the publisher now removes the stale `thelittlespace` login, opens the GitHub browser login automatically, verifies that the active account is exactly `thelittlespace`, configures Git's credential helper, and then continues. Repository-local Git author data is also filled from the authenticated GitHub account when it is missing.

On Windows PowerShell 5.1, GitHub CLI and Git can write normal status text to the native STDERR stream. The publisher deliberately runs native commands through an exit-code wrapper so this output is not misclassified as a PowerShell `NativeCommandError`. Expected non-zero probes such as a missing repository, missing tag, or empty initial Git repository are handled explicitly.

The script creates `thelittlespace/LogiMate` as a public repository when it does not exist, pushes `main`, applies safe merge defaults, creates the tag `v0.0.1-alpha.018`, and pushes that tag.

The tag starts the release workflow. The workflow uses Go 1.27.1 and PowerShell 7.6.6, runs the quality/security/reproducibility gates, and publishes the resulting files as a GitHub prerelease only after the required gates pass.

## Expected release assets

- `LogiMate-0.0.1-alpha-Build018-Go1.27.1-Release.zip`
- `LogiMate.exe`
- `LogiMate-Setup-x64.exe`
- `LogiMate-Portable-x64.zip`
- `LogiMate-arm64-validation.exe`
- `LogiMate-Source.zip`
- `LogiMate-GitHub-Ready.zip`
- `RELEASE_ATTESTATION.json`
- `SHA256SUMS.txt`
- `PACKAGE_VERIFICATION.txt`
- `LogiMate-0.0.1-alpha-Build018-Completion-Report.md`
- `Build018-Go1.27.1-Release-Gate.txt`

## Important status

Build 018 is an alpha/prerelease, not a stable hardware-certified release. G27 LED/FFB evidence is real, but the remaining G27 adverse-lifecycle matrix and physical G25/DFGT certification remain pending. The alpha may be unsigned; stable releases remain blocked until the signing/certification gates are satisfied.

## Reproducible checkout

`.gitattributes` disables Git line-ending conversion for the release source. This is intentional because `SOURCE_MANIFEST_SHA256.txt` validates raw file bytes before the release toolchain is allowed to run.
