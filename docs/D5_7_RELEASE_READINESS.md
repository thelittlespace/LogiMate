# D5.7 Release Readiness

**Release:** 0.5.7-alpha  
**State:** software-complete Alpha; physical certification intentionally pending.

## Software acceptance

- Hardware Certification Assistant exists for G25/G27/DFGT.
- Certification evidence is persisted per physical wheel and protected by strict JSON handling.
- PASS cannot be stored with empty evidence.
- Native PID/PnP/model state can be auto-proven without user judgement.
- Physical input/effect checks are guided and evidence-producing.
- Motor tests reuse central Output Lease, Watchdog and Emergency Stop paths.
- Reconnect/crash/suspend/migration checks remain explicit real-hardware/user evidence.
- Evidence can be exported as Markdown + JSON.
- Advanced Wheel View displays current physical certification progress.
- Repository Stable flags remain unchanged/false until reviewed physical evidence exists.

## Immediate next gate

Run the G27 matrix on the user's real wheel. The first unresolved product issue remains automatic cold C294→C29B promotion if it still requires manual confirmation. The assistant must record that test as FAIL until it works without manual confirmation.

After the G27 matrix is complete, repeat the same workflow on real G25 and DFGT hardware. Only then proceed to the final Stable release gates (UI/accessibility, HID stress and Authenticode).
