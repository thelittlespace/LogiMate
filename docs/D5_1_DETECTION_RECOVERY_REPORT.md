# D5.1 Detection Recovery Report

**Version:** 0.5.1-alpha  
**Scope:** Logitech G25 / G27 / Driving Force GT recognition reliability before hardware certification

## Why the wheel could stop being actionable in D5

The primary regression was in `deviceIsPnPVerified()`. The code checked for the raw-string prefixes `USB\\` and `HID\\` (two literal backslashes), while Windows device instance IDs are shaped like `USB\VID_046D...` and `HID\VID_046D...` (one backslash). A real native C29B G27 could therefore be present and model-confirmed while still receiving `PnPVerified=false`. Safety gates correctly refused model-specific input/output and mode-changing actions after that false negative.

The old test data mirrored the same escaping mistake in several early identity tests, so Windows cross-compilation did not expose the logic error.

A second usability regression came from C8's intentionally conservative C294 model policy. OpenG27 is G27-specific and can treat C294 as its known target. LogiMate supports three classic wheels that share C294, so it stopped persisting model authorization from USB topology alone. That is safer, but after a power cycle a G27 could remain read-only C294 until the user confirmed it again.

## Fixes in 0.5.1-alpha

1. **Real PnP IDs are recognized correctly.** `USB\...` and `HID\...` now pass PnP verification; synthetic continuity records still do not.
2. **OpenG27/HidSharp-style HID interface discovery.** A second SetupAPI pass enumerates the actual present HID device interfaces for Logitech VID 046D and PIDs C294/C299/C29A/C29B. This does not depend on the wheel currently emitting Raw Input reports.
3. **Discovery fusion.** SetupAPI devnodes, SetupAPI HID interfaces and Raw Input HID paths are merged and deduplicated. A valid HID-interface devnode can rescue discovery if the all-class devnode pass is incomplete.
4. **USB/HID child correlation.** Current-session grouping prefers the physical USB topology token derivable from the USB node or HID parent before falling back to ContainerID. This prevents one physical wheel from being split into several logical wheels when Windows container metadata is incomplete during re-enumeration.
5. **G27/G25 C294 session consensus.** For a single real C294 wheel, a literal G25/G27 Windows model plus matching explicit WinMM model can authorize that model for the current Windows session. This is read-only evidence fusion and is never persisted as a topology-only hardware identity.
6. **DFGT stays conservative.** C294 DFGT is not auto-authorized from WinMM because classic compatibility-mode naming can be misleading; C29A or explicit user confirmation remains required.
7. **Native restore no longer depends on opening the input page.** After a safely actionable single wheel is discovered and the remembered operating preference is Modern, LogiMate can start the guarded C294→native restore immediately in the background.
8. **G27 input/output sharing fixed.** The G27 direct-HID reader now opens with `FILE_SHARE_READ | FILE_SHARE_WRITE`, matching the classic G25/DFGT reader and allowing LogiMate's own serialized writer to coexist with its read handle.

## Detection order after D5.1

1. SetupAPI native PID C299/C29A/C29B — authoritative model proof.
2. SetupAPI all-class device tree + SetupAPI HID-interface discovery — authoritative current PnP presence.
3. Raw Input HID paths — live-path/identity supplement.
4. C294 read-only model consensus — G25/G27 only when independent Windows evidence agrees.
5. Explicit current-device C294 confirmation — fallback when the model remains ambiguous.
6. WinMM — input fallback and secondary model evidence, never the sole destructive-action authority.

## Safety properties retained

- A different C294 wheel inserted into the same USB port does not inherit a persisted model authorization merely from topology.
- Multi-wheel target selection still never guesses.
- Native C299/C29A/C29B always override C294 hints.
- DFGT C294 remains manual/native-PID-only.
- Synthetic Direct-HID continuity is not treated as real PnP proof.
- Driver/motor operations still require one selected supported actionable physical wheel.

## Validation performed in this build environment

- Host wheelengine/gameadapter tests: PASS.
- Windows amd64 system `go vet`: PASS.
- Windows amd64 system test compile: PASS.
- Windows amd64 app test compile: PASS.
- Windows amd64 LogiMate build: PASS.
- Windows arm64 compile validation: PASS.
- D5 runtime dependency gate remains unchanged: production does not depend on `internal/openg27port`.

Physical confirmation on the user's G27 is still required because this environment cannot execute the Windows USB/HID stack or operate the real motor hardware.
