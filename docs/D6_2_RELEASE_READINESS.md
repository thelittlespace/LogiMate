# D6.2 Release Readiness — 0.6.2-alpha

## Software gates

- D6 slider-to-owned-config mapping: PASS
- Running Game FFB live-tuning refresh: PASS (software path)
- D4 true 0% per-channel gain: PASS
- D4 v1 -> v2 zero-value compatibility migration: PASS
- Unavailable Transient channel no longer presented as active: PASS
- Manual Constant live-test canonical lg4ff/OpenG27 packet: PASS (byte parity)
- Autocenter SpringSet + SpringEnable and SpringOff neutralization: PASS (byte parity)
- Partial dirty-region rendering / double buffering: PASS (code/build)
- UIA overlay geometry caching/throttled live text: PASS (code/build)
- Windows amd64 System/App compile: PASS
- Windows amd64 vet: PASS
- Windows amd64 GUI build: PASS
- Windows arm64 compile validation: PASS
- D5 productive OpenG27 dependency gate: must remain PASS in final package

## Still physical / external

- G27 perceived/actual force changes for every D6.2 control
- G25/DFGT equivalent behavior
- Constant/Spring/Damper/Friction/Autocenter physical certification
- no-flicker confirmation on the user's real Windows display/DPI setup
- D5.7 reconnect/crash/suspend/migration evidence
- D5.8 Narrator/UIA/High Contrast/mixed-DPI evidence
- D5.9 HID stress/adverse-I/O evidence
- D5.10 Stable Authenticode signing

**Release classification:** Alpha. D6.2 is software-ready for the next real-G27 verification round; it does not promote Stable certification.
