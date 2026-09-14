# LogiMate

**LogiMate** is a native Windows control center for classic Logitech steering wheels. The current target hardware is **Logitech G25, G27 and Driving Force GT**.

> **Status:** `0.0.1-alpha` — active hardware validation and stabilization. Do not treat alpha builds as production/stable releases.

Current hardened alpha milestone: **Build 018**. Build 018 completed the post-Build-017 re-audit and keeps the public product version at `0.0.1-alpha`. It is an alpha/pre-release, not a stable hardware certification.

## What LogiMate does

- detects supported Logitech wheels and their native/compatibility mode
- shows live steering, pedals, buttons, paddles and H-shifter input
- stores per-wheel calibration and control mappings
- provides a Force Feedback control panel with safe live tests
- manages modern Generic HID/native operation and an optional Logitech Legacy compatibility path
- provides profiles, diagnostics, HID stress tests and recovery checks
- keeps output behind a central OutputLease, watchdog and Emergency Neutralize path

## Hardware status

| Wheel | Detection / input | Calibration | Native FFB | Release certification |
| --- | --- | --- | --- | --- |
| Logitech G27 | active development; real-hardware tested | active development | Constant, Spring, Damper, Friction and Autocenter hardware-tested during alpha | not yet stable-certified |
| Logitech G25 | implemented target | implemented target | implemented target | physical matrix still required |
| Logitech Driving Force GT | implemented target | implemented target | implemented target | physical matrix still required |

The source tree contains the detailed certification and audit history under [`docs/`](docs/).

## Install / run

Public builds will be distributed through **GitHub Releases**. Prefer the signed installer once signed release artifacts are available. Portable builds are intended mainly for testing and troubleshooting.

The Build 018 release compiler is pinned to **Go 1.27.1** and the release shell to **PowerShell 7.6.6**. The normal release pipeline is defined in [`build.ps1`](build.ps1). On a normal Windows x64 machine, use the verified bootstrap so the exact portable toolchains are downloaded and SHA-256 verified without changing machine-wide installations:

```powershell
.\Build-Release.cmd
```

The bootstrap produces `LogiMate-0.0.1-alpha-Build018-Go1.27.1-Release.zip` only after toolchain identity, tests, a double-build reproducibility check and a source-archive rebuild all pass.

The visible version intentionally remains `0.0.1-alpha` during the current stabilization line; internal build numbers identify test iterations.

## Force Feedback safety

LogiMate deliberately keeps conservative alpha safety caps. Do not patch around them while testing. Native motor output is guarded by:

- one central output ownership/lease
- watchdog expiry
- crash/recovery marker
- Emergency Neutralize
- model capability checks
- bounded live-test duration and strength

If a hardware test behaves unexpectedly, use **STOP / Emergency Neutralize** and disconnect power if necessary.

## Modern vs Legacy

**Modern / Generic HID** is the long-term LogiMate-native path. The external OpenG27 application is **not** a runtime dependency. OpenG27 remains an MIT-licensed reference/provenance source for selected protocol behavior and parity tests. See [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) and [`docs/OPENG27_PROVENANCE.md`](docs/OPENG27_PROVENANCE.md).

**Logitech Legacy** exists as a compatibility path for users who still need the older Logitech Gaming Software/Profiler stack.

## Diagnostics

Use LogiMate's **Diagnose** page for active problems, self-tests, protocol/transport state, event log and redacted diagnostic export. Before posting diagnostics publicly, review them and remove anything you do not want to share.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). Hardware test reports are especially useful because stable support must be proven on real G25/G27/Driving Force GT hardware, not only by compile-time tests.

## Security

Do not report vulnerabilities with exploit details in a public issue. See [`SECURITY.md`](SECURITY.md) and use GitHub Private vulnerability reporting once the public repository is enabled.

## License

LogiMate is released under the [MIT License](LICENSE). Third-party notices and provenance are documented separately.
