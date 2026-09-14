# LogiMate 0.0.1-alpha · Build 017

## GitHub / security / release candidate

Build 017 finishes the planned pre-publication infrastructure pass.

### Added

- pinned-SHA CodeQL workflow for Go on Windows
- release-candidate workflow with tag/build identity validation
- GitHub artifact provenance for installer and portable package
- weekly Dependabot checks for GitHub Actions and Go modules
- exact recommended repository/ruleset/security settings
- issue templates synchronized to Build 017

### Publication policy

This build is a **publication candidate**, not an instruction to publish immediately. A complete post-Build-017 program audit and remaining real-hardware/accessibility validation still come first.

The program-visible version remains `0.0.1-alpha`. Git tags may append the build number, e.g. `v0.0.1-alpha.017`, solely to identify a reproducible test/release artifact.
