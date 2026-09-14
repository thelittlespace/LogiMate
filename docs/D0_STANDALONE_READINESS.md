# D0 standalone-readiness checklist

Release: **0.3.0-alpha**

## Modern-mode dependency check

- [x] G25/G27/DFGT share one LogiMate model/capability registry.
- [x] One Modern migration transaction is used for all supported models.
- [x] Native input is integrated into LogiMate; WinMM remains an internal fallback, not a separate program.
- [x] Native range/autocenter/effect output is integrated.
- [x] Game-profile storage and auto-apply are integrated.
- [x] Wreckfest 2 Pino parser/listener is integrated.
- [x] Game FFB is integrated and capability-driven.
- [x] G27 RPM LED output is integrated and capability-gated.
- [x] External OpenG27 is not downloaded/launched by normal Modern setup.
- [x] Logitech Profiler/LGS is not required by Modern mode.
- [x] Existing `openg27-pino` saved profiles migrate to LogiMate's `wreckfest2-pino` ID.
- [x] All production output paths use the common Native output ownership/transport boundary.

## Deliberately retained compatibility surfaces

- OpenG27 profile import and provenance/reference tests remain so existing users can migrate data and upstream-derived behavior stays attributable.
- An optional historical OpenG27 A/B fallback remains in source for diagnostics during Alpha. It is not part of normal setup or operation and is a D5 removal/retirement candidate.
- Legacy LGS/Profiler mode remains supported as an explicit alternative for users who intentionally want the original Logitech stack.

## External gates before Stable

- [ ] G25 physical input/output certification.
- [ ] G27 physical input/output/recovery certification against D0 transport.
- [ ] DFGT physical input/output certification.
- [ ] USB-yank/process-kill/suspend/shutdown force-neutralization matrix.
- [ ] Modern↔Legacy migration matrix on real Windows.
- [ ] HID stress/adverse-I/O matrix.
- [ ] UI/accessibility matrix.
- [ ] Authenticode signing and verification.

**Interpretation:** software independence is reached for the normal Modern architecture; Stable hardware certification is not.
