# Recommended GitHub repository settings

Target repository: `thelittlespace/LogiMate`.

Do not make the repository public until the final post-Build-017 audit is complete. The first public download should remain a GitHub **pre-release**, not a stable release.

## General

- Default branch: `main`
- Merge method: **Squash merge only**
- Disable merge commits and rebase merges
- Automatically delete head branches after merge
- Issues: enabled
- Discussions: optional; enable once there is enough public usage to justify a second support surface
- Wiki: disabled unless it later replaces part of `docs/`

## `main` ruleset

Apply to `main` and block force pushes and branch deletion.

Require:

- pull request before merge
- linear history
- branch up to date before merge
- successful required checks:
  - `Go format, unit and race tests`
  - `Windows amd64 vet, test, build and smoke`
  - `Windows arm64 compile gate`
  - `CodeQL · Go`

While Markus is the only maintainer, do **not** require a second-person approval because it would make normal maintenance dependent on another account. Once a second maintainer exists, require at least one approval and dismissal of stale approvals after new commits.

## Release-tag ruleset

Protect `v*` tags from update and deletion. A published release tag is immutable.

The visible application version remains `0.0.1-alpha` during stabilization. Tags identify the tested build, for example:

`v0.0.1-alpha.017`

## Actions security

- Default `GITHUB_TOKEN` permissions: read-only
- Permit write permissions only in explicitly reviewed workflows
- Keep third-party actions pinned to full commit SHA
- Require approval for workflows from first-time external contributors if public forks are accepted
- Do not store a code-signing PFX or private key in the repository

## Security features

Enable when the repository is made public:

- CodeQL / code scanning
- Dependabot alerts
- Dependabot security updates
- secret scanning
- push protection for detected secrets
- private vulnerability reporting

Keep `SECURITY.md` and the private-reporting route visible before inviting public testing.

## Releases

The `Release Candidate` workflow intentionally builds and attests artifacts but does not automatically publish a GitHub Release. That manual publication boundary prevents a tag or workflow trigger from immediately exposing a binary without human review.

For a public prerelease:

1. all required CI and CodeQL checks green
2. verify `SHA256SUMS.txt`
3. verify release attestation / provenance
4. test installer and portable package on a clean Windows system
5. review release notes
6. create GitHub Release and mark it **Pre-release**

Stable publication remains blocked by LogiMate's own hardware, recovery, migration, accessibility, HID-stress and Authenticode gates.
