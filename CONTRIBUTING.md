# Contributing to LogiMate

Thanks for helping test or improve LogiMate. The project is still `0.0.1-alpha`, so correctness, fail-closed behavior and reproducible hardware evidence matter more than adding features quickly.

## Development environment

- Go **1.27.1** for Build 018 (exact release compiler; see `docs/TOOLCHAINS.md`)
- Windows 11 recommended for app/system integration work
- PowerShell 7 or Windows PowerShell for `build.ps1`
- supported Logitech hardware for changes that touch wheel I/O

## Before a pull request

Run at minimum:

```text
gofmt -w <changed .go files>
go test ./...
```

On Windows also run:

```text
go vet -unsafeptr=false ./...
```

For production-build changes, run `build.ps1` and the startup smoke test. CI performs Linux unit/race checks and Windows amd64/arm64 compile gates.

## Safety-sensitive changes

Changes to HID output, FFB, OutputLease/watchdog, driver migration, elevation, updater/install or recovery code need focused tests and a written failure-mode explanation. Do not increase motor safety caps merely to make a hardware test easier to feel.

A wheel-output change should preserve:

- single output ownership
- bounded writes/timeouts
- Emergency Neutralize
- recovery marker semantics
- model/capability validation
- no silent fallback to a different physical wheel

## Hardware evidence

When a change affects physical behavior, state exactly which wheel was tested, Windows version, USB/native PID, effect/settings used and whether STOP/reconnect/recovery succeeded. Use the Hardware Compatibility issue template when appropriate.

## Provenance and licensing

OpenG27 is an MIT-licensed protocol/reference source; `internal/openg27port` is reference/test code and must not become a production runtime dependency. `new-lg4ff` is useful for protocol facts, but GPL implementation code must not be copied into this MIT project without a separate licensing decision. Update `THIRD_PARTY_NOTICES.md` / provenance docs when a new external reference materially affects implementation.

## Pull requests

Keep PRs focused. Explain the user-visible result, tests, hardware evidence and any security/safety impact. CI must be green before merge.
