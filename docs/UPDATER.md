# LogiMate automatic updater

LogiMate checks GitHub Releases from `thelittlespace/LogiMate` after the Win32 startup milestone has completed. Update networking never runs in the window-creation path.

## Release channels

- Stable mode ignores GitHub releases marked as prerelease.
- Preview mode includes alpha, beta and RC releases.
- Tags must be SemVer-compatible (`v0.0.9-alpha`, `v1.0.0`, etc.). Unknown tag formats are ignored.

## Release assets

A GitHub release should contain these exact names:

- `LogiMate.exe`
- `LogiMate-Setup-x64.exe`
- `LogiMate-Portable-x64.zip`
- `LogiMate-Source.zip`
- `SHA256SUMS.txt`

Program Files installations prefer the setup asset. Portable mode uses the portable ZIP. A standalone EXE outside Program Files updates from the single EXE asset.

## Integrity

The downloaded asset must pass SHA-256 verification. LogiMate first uses GitHub's asset `digest` field and falls back to the matching entry in `SHA256SUMS.txt`. If neither is available, the update is rejected.

This is an integrity check, not Authenticode publisher verification. Preview binaries are currently not code-signed.

## Apply model

LogiMate never overwrites the running executable.

- Portable: extract to a staging directory, launch a hidden PowerShell helper, wait for the current PID, copy the staged package, then restart with `--post-update`.
- Standalone: a helper waits for the current PID, replaces only `LogiMate.exe`, then restarts with `--post-update`.
- Installed: the verified setup asset is elevated through UAC with `--update --wait-pid <PID>`. Setup waits for the old process to release Program Files before replacing it and refreshing uninstall metadata.

## Changelog behavior

`settings.json` stores `lastSeenVersion`. On first launch or when the embedded application version differs, LogiMate opens the native **Was ist neu?** Windows-material window after startup is stable. Users can disable automatic display and open it manually from Settings.
