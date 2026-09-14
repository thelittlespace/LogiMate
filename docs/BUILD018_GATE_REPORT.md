# Build 018 — Audit and gate report

Visible version: **0.0.1-alpha**  
Internal build: **018**  
Baseline: verified Build 017 Publication Candidate.

## Audit checklist

| # | Area | Result | Notes |
|---:|---|---|---|
| 1 | Installer / rollback | PASS (static + compile) | Transactional `.new`/`.previous` activation retained; failed activation restores previous EXE. |
| 2 | Uninstaller | PASS (static) | Destructive removal is restricted to the exact managed `Program Files\LogiMate\LogiMate.exe` path. |
| 3 | Legacy LGS download | PASS (static) | HTTPS Logitech source, size bound and valid Logitech Authenticode signer required before execution. |
| 4 | Authenticode trust | PASS policy / EXTERNAL signing | Stable gate remains fail-closed; Build 018 alpha is not claimed signed. |
| 5 | Update helper | PASS (static + compile) | One-shot authorization, age/PID checks, exact target authorization and transactional replacement retained. |
| 6 | Portable update | PASS (static + compile) | Stage must be below Updates root; target is bound exactly to `<authorized dir>\LogiMate.exe`; update-root targets blocked. |
| 7 | Setup/migration cancellation | PASS (static + Windows test compile) | Journal/cancel/failure paths retained; HVCI mutation removed. |
| 8 | Wheel identity | PASS (static + Windows test compile) | Stable identity/fail-closed C294 rules retained. |
| 9 | Multi-wheel | PASS (static + Windows test compile) | No automatic target guessing with multiple wheels. |
| 10 | Calibration | PASS (static + Windows test compile) | Valid-sample requirements retained. |
| 11 | Profiles | PASS (static + Windows test compile) | Effective gain/safety model retained. |
| 12 | Schema/downgrade | PASS (static + Windows test compile) | Future-schema fail-closed protections retained. |
| 13 | Complete FFB chain | PASS software compile; HARDWARE partial | Single productive path retained; existing real G27 evidence remains, full hardware matrix still pending. |
| 14 | OutputLease | PASS (static + Windows test compile) | Central ownership remains authoritative. |
| 15 | Watchdog | PASS (static + Windows test compile) | Existing neutralization/watchdog path retained. |
| 16 | Recovery marker | PASS (static + Windows test compile) | Runtime recovery marker path retained. |
| 17 | Crash recovery | PASS software / HARDWARE pending | Physical process-kill validation still required. |
| 18 | USB lifecycle | PASS software / HARDWARE pending | Physical USB-yank/reconnect validation still required. |
| 19 | Suspend/resume | PASS software / HARDWARE pending | Lifecycle code retained; real Windows power transition still required. |
| 20 | Diagnostics/privacy | PASS (static + compile) | Final redaction pass added; hash-sidecar write failures surfaced; folder-open errors visible. |
| 21 | Entire UI | PASS compile / WINDOWS visual pending | Current UI terminology cleaned; DPI/themes/multi-monitor still require real Windows visual QA. |
| 22 | Accessibility | PASS compile / WINDOWS runtime pending | Existing UIA paths compile; Narrator/keyboard/high-contrast real tests remain. |
| 23 | Text/tooltips | PASS current references | Obsolete normal-UI phase names removed where identified. |
| 24 | Current docs | PASS | `ARCHITECTURE`, `TESTING`, `SUPPORTED_WHEELS` and Build 018 docs updated; historical reports intentionally preserved. |
| 25 | Issue/PR templates | PASS retained | Build 017 GitHub-ready metadata retained. |
| 26 | Secret scan | PASS local static | No obvious embedded secret literal found; GitHub Secret Scanning remains repository-side. |
| 27 | Private paths | PASS local static | No committed absolute user path found; diagnostics final redaction added. |
| 28 | Temporary artifacts | PASS audit | Update/build temporary files are bounded/cleaned; package excludes local validator artifacts. |
| 29 | CI | PASS configuration / EXTERNAL run pending | Workflows updated to Go 1.27.1 and current stable pinned Actions. |
| 30 | CodeQL | PASS configuration / EXTERNAL run pending | Updated to CodeQL Action v4.37.5 full SHA. |
| 31 | Dependabot | PASS retained | Build 017 configuration retained. |
| 32 | Release workflow | PASS configuration / EXTERNAL run pending | Updated Actions plus source-rebuild/reproducibility gate. |
| 33 | Reproducibility | PASS with local validator compiler | x64 app and installer rebuilt byte-identically twice. Go 1.27.1 CI reproduction still must run externally. |
| 34 | Signing | EXTERNAL pending | No signing certificate was provided; alpha remains explicitly unsigned. |
| 35 | Windows runtime matrix | EXTERNAL pending | Requires real Windows 100/125/150/200% DPI, themes, high contrast, mixed DPI, Narrator. |
| 36 | G27 hardware matrix | PARTIAL real evidence | Existing LED output + FFB live tests confirmed; remaining physical adverse/lifecycle matrix pending. |
| 37 | G25/DFGT hardware | EXTERNAL pending | Software paths compile; no real-device certification evidence. |

## Build 018 fixes from this audit

- Removed automatic Memory Integrity/HVCI disable/re-enable behavior and removed the unused registry setter entirely.
- Made the Memory Integrity status interactive; it opens Windows Security Core Isolation and never changes the setting. The status is also keyboard-focusable and exposed as a native UI Automation button for Narrator/screen-reader access.
- Fixed a Build 018 concurrency regression caught during the audit (duplicate `stateMu.Lock()` in setup state synchronization).
- Added/used state snapshots/locks around refresh-sensitive global state where identified.
- Hardened portable update target authorization and GitHub download URL validation.
- Restricted external links to HTTPS.
- Hardened managed uninstall path authorization.
- Added final diagnostic privacy redaction and surfaced diagnostic hash write errors.
- Stopped silently trusting an in-memory wheel-preference migration when durable persistence fails; persistence failures now surface in selection status.
- Updated current user-facing terminology and current reference docs.
- Updated Build ID to 018 while keeping visible version `0.0.1-alpha`.

## Toolchain gate

The source baseline is configured for **Go 1.27.1** and the release build script requires that exact Go patch version. It also requires **exact PowerShell 7.6.6** for the Build 018 release pipeline. GitHub Actions are pinned by full commit SHA.

The execution sandbox used for this audit only contains Go **1.23.2** and has outbound DNS/network access disabled, so the official Go 1.27.1 toolchain could not be downloaded here. The same final source was therefore compiled and tested with a temporary validation-only `go 1.23` copy; this does **not** replace the required Go 1.27.1 CI/release gate.

Local validation with Go 1.23.2 passed:

- gofmt
- cross-platform unit tests
- cross-platform race tests
- Windows amd64 vet for app/system
- Windows test-package compilation
- Windows x64 application build
- Windows ARM64 application compile
- Windows x64 installer vet/build
- static security gates
- app reproducibility (two byte-identical builds)
- installer reproducibility (two byte-identical builds)

The latest-toolchain Windows smoke test, full `go test ./...` execution on Windows, CodeQL run, repository secret scanning, provenance attestation and real hardware/UI tests remain external gates and must not be represented as locally executed.


## Build 018 release-bootstrap hardening

The Windows release kit now contains `Build-Release.cmd` and `Build-Release-Go1271.ps1`. The bootstrap is deliberately compatible with inbox Windows PowerShell for the download step, but the actual release is executed only under a verified portable **PowerShell 7.6.6** process with a verified official **Go 1.27.1** Windows toolchain. No machine-wide Go or PowerShell installation is modified.

Before execution, the bootstrap verifies the official Go and PowerShell archive SHA-256 values, forces `GOTOOLCHAIN=local`, verifies runtime toolchain identity, executes formatting/vet/unit gates and `build.ps1`, rebuilds a second time, compares application and installer bytes, and verifies the resulting release attestation reports `go1.27.1` and PowerShell `7.6.6`.

This audit environment still cannot execute that Windows bootstrap because it has no Windows runtime and outbound binary downloads are blocked. Therefore only a release produced by a successful bootstrap run (or the equivalent GitHub Windows workflow) may be described as a **Go 1.27.1-built binary release**. The supplied release kit itself is source + verified build automation, not falsely relabeled Go 1.23 binaries.

## Final Build 018 release-kit regression pass

After adding the verified release bootstrap, exact toolchain attestation and keyboard/UIA access for the Memory Integrity action, the source was revalidated from a clean temporary copy. Because this sandbox cannot download or execute the Windows Go 1.27.1 toolchain, the local compatibility-only copy temporarily used Go 1.23.2 and is **not** a release artifact. The following gates passed again:

- gofmt clean
- Linux unit tests
- Linux race tests for wheelengine/openg27port/gameadapter
- Windows amd64 `go vet`
- Windows app/system test-package compilation
- Windows amd64 application compile
- Windows ARM64 application compile
- Windows amd64 installer vet/compile

The real release workflow additionally requires exact Go 1.27.1 on Linux quality/CodeQL jobs and the SHA-verified official Go 1.27.1 + PowerShell 7.6.6 Windows bootstrap before release artifacts can be uploaded/attested.

## Repository / full release packaging addendum

Build 018 now has a repository-first publication path targeting `thelittlespace/LogiMate` and tag `v0.0.1-alpha.018`.

The tag-triggered workflow publishes a GitHub **prerelease** only after the Go 1.27.1 quality gate, CodeQL and the Windows reproducibility/bootstrap gate complete successfully. Release assets include the standalone x64 application, x64 installer, x64 portable archive, ARM64 compile-validation executable, source archive, GitHub-ready archive, SHA-256 sums, release attestation, package verification, completion report and gate transcript.

The workflow also creates GitHub build-provenance attestations for the standalone application, installer, portable archive, ARM64 validation binary, source archive, GitHub-ready archive and complete release bundle. Release publication is idempotent: a workflow re-run updates the prerelease metadata and replaces assets instead of failing merely because the tag release already exists.

Repository checkouts are protected against automatic CRLF/LF rewriting by `.gitattributes`; this is deliberate because `SOURCE_MANIFEST_SHA256.txt` validates raw bytes. A local clone test with `core.autocrlf=true` preserved all 262 manifest-tracked source files byte-for-byte.

The ChatGPT GitHub connector available during this preparation can modify existing repositories but cannot create a new repository. `Publish-Repository.cmd` / `Publish-Repository.ps1` therefore provide the one-time GitHub CLI bootstrap: create the public repository if absent, push `main`, apply conservative merge defaults and push `v0.0.1-alpha.018` to trigger the gated prerelease workflow.
