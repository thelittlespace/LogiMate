# Security Policy

## Supported versions

LogiMate is currently in an early `0.0.1-alpha` line. Security fixes are delivered only for the newest published build. Older alpha builds are not maintained separately.

## Reporting a vulnerability

Please **do not** publish sensitive security details in a normal GitHub issue.

When the public repository is live, use GitHub's **Private vulnerability reporting** / Security Advisory flow for `thelittlespace/LogiMate`. If that option is temporarily unavailable, open a minimal public issue that contains no exploit details, credentials, private device identifiers or diagnostic archives and ask the maintainer for a private contact path.

Include, when safe:

- affected LogiMate build
- Windows version / architecture
- short impact summary
- reproduction prerequisites
- whether wheel output / driver migration / updater code is involved

Never attach secrets, private keys, full registry exports, unredacted diagnostic archives or signing material to public issues.

## Security-sensitive areas

Changes to the following areas require extra review and tests:

- HID / force-feedback output and Emergency Stop
- OutputLease / watchdog / crash recovery
- driver migration and elevation helpers
- update download, digest verification and Authenticode publisher checks
- installer and release signing
- configuration migration / atomic persistence

## Release policy

Stable releases are blocked until the repository's hardware, recovery, migration, accessibility, HID-stress and Authenticode gates are satisfied. Public alpha releases must be marked as pre-releases.
