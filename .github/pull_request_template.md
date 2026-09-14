## Summary

Describe the user-visible and technical change.

## Validation

- [ ] `gofmt` clean
- [ ] `go test ./...` passes
- [ ] Windows `go vet -unsafeptr=false ./...` passes
- [ ] Relevant Windows build/smoke test passes
- [ ] No production dependency on `internal/openg27port` was introduced
- [ ] UI changes were checked in Dark / Gray / Light where relevant
- [ ] Hardware behavior was tested on the affected wheel, or explicitly marked hardware-unverified

## Safety / migration impact

- [ ] No change to HID/FFB output, elevation, updater or driver migration
- [ ] Or: the safety-sensitive change and its fail-closed behavior are described below

## Provenance

List any external protocol/code reference used. Do not copy GPL implementation code into the MIT codebase without an explicit license review.
