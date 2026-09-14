# D3 Release Readiness — 0.3.3-alpha

## Software status

**PASS for Alpha packaging.**

Required D3 conditions are met:

- typed adapter contract and registry exist;
- Wreckfest/Pino and Local JSON use the same lifecycle;
- Native Game Output is adapter-generic;
- legacy `openg27-pino` is an alias, not a second runtime;
- adapters cannot own wheel HID/output;
- adapter health/rate/age/setup diagnostics are exposed;
- game-profile schema 2 rejects unknown future versions for writes;
- D0/D2 central output lease and crash-recovery model remain unchanged.

## External gates still open

- real G25/G27/DFGT hardware certification;
- USB yank/process-kill/suspend-resume motor recovery matrix;
- Modern↔Legacy↔Modern real driver-cycle evidence;
- real Authenticode signing for Stable;
- HID stress and accessibility evidence required by the existing Stable gate.

D3 is therefore an Alpha architecture release, not a Stable hardware certification claim.
