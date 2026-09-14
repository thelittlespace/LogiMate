# LogiMate 0.0.1-alpha · Build 014 — Reliability / Lifecycle

- Suspend/standby neutralizes output and closes the live HID input session.
- Resume is treated as a reconnect: HID/PnP caches and ephemeral model evidence are invalidated before rebinding.
- Force Feedback is never restarted automatically after resume.
- Lifecycle transitions are written to the Diagnostics event stream.
- Added regression coverage for all Windows suspend/resume broadcast variants used by LogiMate.

Physical suspend/resume, USB-yank and process-kill evidence remains a real-hardware gate and is not auto-certified by these tests.
