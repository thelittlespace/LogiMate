# D5.2 Release Readiness — 0.5.2-alpha

## Software gates

- HID-first SetupAPI discovery: PASS (code/build)
- OpenG27-style report-capability ranking: PASS (code + regression compile)
- Coherent detection snapshot / bounded settle: PASS (code + regression compile)
- Direct HID product/serial metadata: PASS (code/build)
- Native identity history v2 safety rules: PASS (code/build)
- C294→native wait uses direct HID first: PASS (code/build)
- Direct HID diagnostics UI: PASS (code/build)
- Windows amd64 system/app test compile: PASS
- Windows amd64 app build: PASS
- Windows arm64 app compile: PASS
- D5 standalone dependency gate: PASS

## Physical gates still open

- Real G27 cold C294 → C29B detection: PENDING USER HARDWARE
- Real G27 already-native C29B detection: PENDING USER HARDWARE
- Plug-after-start / replug continuity: PENDING USER HARDWARE
- Real Direct-HID reader + Native-output coexistence: PENDING USER HARDWARE
- G25 / DFGT equivalent detection matrix: PENDING HARDWARE

D5.2 is an Alpha detection-recovery release. Stable remains blocked by the existing hardware certification policy.
