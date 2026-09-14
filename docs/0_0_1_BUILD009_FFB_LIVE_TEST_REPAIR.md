# LogiMate 0.0.1-alpha · Build 009 — FFB Live-Test Repair

## User-visible defect
The Force Feedback Live & Test buttons could appear to do nothing. Two independent causes existed: the custom-painted controls were activated only from a release-time hit test, and a previous motor test could keep the central OutputLease so the next test was rejected. Spring/Damper/Friction also still used the historical generic slot builders rather than the direct OpenG27/lg4ff packet family used by the native wheelengine reference.

## Fix
- explicit mouse press/capture/release state for custom FFB controls;
- replace-current-test lifecycle: Emergency Neutralize -> HID probe -> fresh OutputLease -> test;
- Constant and Autocenter stop any previous motor owner before starting;
- Spring: canonical SpringSet + SpringEnable, periodic SpringSet, SpringOff + global stop;
- Damper: canonical Damper packet, periodic refresh, DamperOff + global stop;
- Friction: canonical Friction packet, periodic refresh, FrictionOff + global stop;
- active-test highlighting and immediate inline status.

## Safety
No safety ceiling is raised. NativeFFBConfig caps, per-wheel gains, OutputLease, recovery marker, watchdog and Emergency Neutralize remain authoritative. Protocol coefficients are derived from the already safety-limited applied percentage and never scale the 30% Alpha cap back to full protocol strength.

## Remaining physical gate
The software path is testable and packet-parity checked without hardware, but motor response still has to be confirmed on the real G27.
