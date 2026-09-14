# D5.1 Release Readiness

**Release:** 0.5.1-alpha — Detection Recovery

## Software gate

PASS for release as an Alpha detection/reliability update.

## What is fixed

- False-negative PnP verification caused by incorrectly escaped Windows instance-ID prefixes.
- Additional SetupAPI HID-interface discovery comparable to the enumeration layer used by HidSharp-style applications.
- Fused PnP/HID/Raw-Input discovery and improved USB/HID grouping.
- Session-only G25/G27 C294 evidence consensus.
- Immediate guarded Modern native-mode restoration after safe discovery.
- G27 Direct-HID read/write share compatibility.

## Hardware gate

Still required before Stable:

- G27 cold-plug as C294 and automatic C29B restoration.
- G27 already-native C29B detection.
- unplug/replug and port move.
- G25 C294/C299.
- DFGT C294/C29A.
- two same-model wheels simultaneously.
- Legacy→Modern→Legacy migration after detection changes.
- FFB/recovery tests after native re-enumeration.
